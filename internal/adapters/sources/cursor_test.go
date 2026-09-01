package sources_test

import (
	"crypto/rand"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func newCodec(t *testing.T) *sources.CursorCodec {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return sources.NewCursorCodec(k)
}

func TestCursor_RoundTrip(t *testing.T) {
	cc := newCodec(t)
	in := sources.Cursor{SourceID: "src-1", Kind: sources.KindLocalFolder, Position: "the-dispossessed.epub"}

	token := cc.Encode(in)
	out, err := cc.Decode(token, "src-1")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out != in {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
}

func TestCursor_RejectsTamperedToken(t *testing.T) {
	cc := newCodec(t)
	token := cc.Encode(sources.Cursor{SourceID: "src-1", Kind: sources.KindOPDS, Position: "https://opds.example.org/x"})

	// Flip a character in the MAC region (near the front) and, separately,
	// in the payload region (near the middle). base64url is malleable at
	// the very last character, so avoid the boundary.
	for _, pos := range []int{4, len(token) / 2} {
		b := []byte(token)
		if b[pos] == 'A' {
			b[pos] = 'B'
		} else {
			b[pos] = 'A'
		}
		if _, err := cc.Decode(string(b), "src-1"); !isInvalidInput(err) {
			t.Fatalf("tampered token at %d: err = %v, want InvalidInput", pos, err)
		}
	}
}

func TestCursor_RejectsForeignKey(t *testing.T) {
	a, b := newCodec(t), newCodec(t)
	token := a.Encode(sources.Cursor{SourceID: "src-1", Kind: sources.KindOPDS, Position: "https://h/x"})
	if _, err := b.Decode(token, "src-1"); !isInvalidInput(err) {
		t.Fatalf("foreign-key token: err = %v, want InvalidInput", err)
	}
}

func TestCursor_RejectsSourceIDMismatch(t *testing.T) {
	cc := newCodec(t)
	token := cc.Encode(sources.Cursor{SourceID: "src-1", Kind: sources.KindLocalFolder, Position: "x"})
	if _, err := cc.Decode(token, "src-2"); !isInvalidInput(err) {
		t.Fatalf("replayed cursor: err = %v, want InvalidInput", err)
	}
}

func TestCursor_RejectsGarbage(t *testing.T) {
	cc := newCodec(t)
	for _, s := range []string{"", "!!!", "abc", "YWJjZGVm"} {
		if _, err := cc.Decode(s, "src-1"); !isInvalidInput(err) {
			t.Errorf("Decode(%q): err = %v, want InvalidInput", s, err)
		}
	}
}

func TestCursor_ForOPDSPositionOriginCheck(t *testing.T) {
	const base = "https://opds.example.org/catalog"

	ok := sources.Cursor{Kind: sources.KindOPDS, Position: "https://opds.example.org/catalog?page=3"}
	got, err := ok.ForOPDSPosition(base)
	if err != nil || got != ok.Position {
		t.Fatalf("same-origin position: got %q, %v", got, err)
	}

	offOrigin := sources.Cursor{Kind: sources.KindOPDS, Position: "https://evil.example.org/x"}
	if _, err := offOrigin.ForOPDSPosition(base); !isInvalidInput(err) {
		t.Fatalf("off-origin position: err = %v, want InvalidInput", err)
	}

	empty := sources.Cursor{Kind: sources.KindOPDS, Position: ""}
	if got, err := empty.ForOPDSPosition(base); err != nil || got != "" {
		t.Fatalf("empty position: got %q, %v", got, err)
	}
}

func isInvalidInput(err error) bool {
	return err != nil && domain.CategoryOf(err) == domain.InvalidInput
}
