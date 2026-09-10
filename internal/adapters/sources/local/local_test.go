package local_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources/local"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func newCodec(t *testing.T) *sources.CursorCodec {
	t.Helper()
	return sources.NewCursorCodec(bytes.Repeat([]byte{7}, 32))
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newProvider(t *testing.T, base string, logger *slog.Logger) *local.Provider {
	t.Helper()
	p, err := local.New("src-local", base, newCodec(t), logger)
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}
	return p
}

func TestProvider_ListsSupportedFilesSortedSkippingTheRest(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "zeta.epub"), 10)
	writeFile(t, filepath.Join(base, "alpha.pdf"), 20)
	writeFile(t, filepath.Join(base, "notes.txt"), 5)          // unsupported ext
	writeFile(t, filepath.Join(base, "sub", "nested.epub"), 3) // subdirectory
	writeFile(t, filepath.Join(base, "Mixed.EPUB"), 7)         // case-insensitive ext

	page, err := newProvider(t, base, nil).List(context.Background(), "", 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var got []string
	for _, it := range page.Items {
		got = append(got, it.FileReference.ReferenceID)
	}
	want := []string{"Mixed.EPUB", "alpha.pdf", "zeta.epub"}
	if len(got) != len(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("items = %v, want %v", got, want)
		}
	}
	if page.NextCursor != nil {
		t.Fatalf("unexpected nextCursor on a complete page")
	}

	// Format + size populated.
	for _, it := range page.Items {
		if it.FileReference.Format == "" || it.FileReference.SizeBytes == nil {
			t.Fatalf("candidate %q missing format/size", it.FileReference.ReferenceID)
		}
	}
}

func TestProvider_Pagination(t *testing.T) {
	base := t.TempDir()
	for _, n := range []string{"a.epub", "b.epub", "c.epub", "d.epub", "e.epub"} {
		writeFile(t, filepath.Join(base, n), 1)
	}
	p := newProvider(t, base, nil)

	page1, err := p.List(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1.Items) != 2 || page1.NextCursor == nil {
		t.Fatalf("page1 = %d items, cursor=%v", len(page1.Items), page1.NextCursor)
	}

	page2, err := p.List(context.Background(), *page1.NextCursor, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	page3, err := p.List(context.Background(), *page2.NextCursor, 2)
	if err != nil {
		t.Fatalf("List page3: %v", err)
	}

	seen := map[string]int{}
	for _, pg := range []sources.CandidatePage{page1, page2, page3} {
		for _, it := range pg.Items {
			seen[it.FileReference.ReferenceID]++
		}
	}
	if len(seen) != 5 {
		t.Fatalf("saw %d distinct files across pages, want 5: %v", len(seen), seen)
	}
	for name, n := range seen {
		if n != 1 {
			t.Fatalf("%s appeared %d times across pages", name, n)
		}
	}
	if page3.NextCursor != nil {
		t.Fatalf("page3 should be the last page")
	}
}

func TestProvider_SymlinkEscapingRootIsOmitted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is POSIX-only")
	}
	base := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(base, "real.epub"), 4)
	writeFile(t, filepath.Join(outside, "secret.epub"), 4)

	// A symlink whose *name* lives inside base but whose *target* is
	// outside — the exact attack FR-12 exists to close.
	if err := os.Symlink(filepath.Join(outside, "secret.epub"), filepath.Join(base, "escape.epub")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// And a broken symlink.
	if err := os.Symlink(filepath.Join(base, "nonexistent.epub"), filepath.Join(base, "broken.epub")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var logbuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logbuf, nil))

	page, err := newProvider(t, base, logger).List(context.Background(), "", 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, it := range page.Items {
		if it.FileReference.ReferenceID != "real.epub" {
			t.Fatalf("escaping/broken symlink leaked into the listing: %q", it.FileReference.ReferenceID)
		}
	}
	if len(page.Items) != 1 {
		t.Fatalf("want exactly real.epub, got %d items", len(page.Items))
	}

	out := logbuf.String()
	if !bytes.Contains(logbuf.Bytes(), []byte("level=WARN")) {
		t.Fatalf("expected a WARN for the omitted symlink: %q", out)
	}
	// The offending target path must not be in the log.
	if bytes.Contains(logbuf.Bytes(), []byte(outside)) {
		t.Fatalf("warn line leaked the escape target path: %q", out)
	}
}

func TestProvider_SymlinkWithinRootIsListed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is POSIX-only")
	}
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "d", "target.epub"), 4)
	if err := os.Symlink(filepath.Join(base, "d", "target.epub"), filepath.Join(base, "link.epub")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	page, err := newProvider(t, base, nil).List(context.Background(), "", 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// link.epub resolves inside base → allowed. (target.epub itself is in
	// a subdirectory, not listed.)
	var names []string
	for _, it := range page.Items {
		names = append(names, it.FileReference.ReferenceID)
	}
	if len(names) != 1 || names[0] != "link.epub" {
		t.Fatalf("in-root symlink handling wrong: %v", names)
	}
}

func TestProvider_Probe(t *testing.T) {
	t.Run("reachable", func(t *testing.T) {
		res := newProvider(t, t.TempDir(), nil).Probe(context.Background())
		if res.Status != sources.HealthReachable {
			t.Fatalf("status = %q, want reachable (%q)", res.Status, res.Detail)
		}
		if !res.Capabilities.CanList || res.Capabilities.CanSearch || !res.Capabilities.CanDownload {
			t.Fatalf("caps = %+v, want list+download only", res.Capabilities)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		p, err := local.New("s", filepath.Join(t.TempDir(), "gone"), newCodec(t), nil)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		res := p.Probe(context.Background())
		if res.Status != sources.HealthUnreachable || res.Detail != sources.DetailPathNotFound {
			t.Fatalf("res = %+v, want unreachable/path-not-found", res)
		}
	})

	t.Run("not a directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "afile")
		writeFile(t, f, 1)
		p, _ := local.New("s", f, newCodec(t), nil)
		res := p.Probe(context.Background())
		if res.Status != sources.HealthUnreachable {
			t.Fatalf("res = %+v, want unreachable", res)
		}
	})

	t.Run("unreadable directory", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("chmod-based unreadable test needs a non-root POSIX host")
		}
		dir := filepath.Join(t.TempDir(), "noperm")
		if err := os.Mkdir(dir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		p, _ := local.New("s", dir, newCodec(t), nil)
		res := p.Probe(context.Background())
		if res.Status != sources.HealthUnreachable || res.Detail != sources.DetailPathNotReadable {
			t.Fatalf("res = %+v, want unreachable/path-not-readable", res)
		}
	})
}

func TestProvider_SearchIsConflict(t *testing.T) {
	_, err := newProvider(t, t.TempDir(), nil).Search(context.Background(), "anything", "", 20)
	if domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("Search err = %v, want Conflict", err)
	}
}

func TestProvider_ResolveRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "book.epub"), 9)
	p := newProvider(t, base, nil)

	rc, err := p.Resolve(context.Background(), domain.FileReference{ReferenceID: "book.epub", Format: "EPUB"})
	if err != nil {
		t.Fatalf("Resolve valid: %v", err)
	}
	_ = rc.Close()

	for _, bad := range []string{"../book.epub", "../../etc/passwd", "sub/book.epub", "/etc/passwd", "", "."} {
		if _, err := p.Resolve(context.Background(), domain.FileReference{ReferenceID: bad, Format: "EPUB"}); err == nil {
			t.Errorf("Resolve(%q) = nil error, want rejection", bad)
		}
	}
}

// #176: Resolve applies the same extension filter as List, so a request
// for a non-book file in the source root — a .env, an SSH key — is not
// found even though the file is really there.
func TestProvider_ResolveRejectsUnsupportedExtension(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, ".env"), 20)
	writeFile(t, filepath.Join(base, "secrets.txt"), 20)
	p := newProvider(t, base, nil)

	for _, name := range []string{".env", "secrets.txt"} {
		rc, err := p.Resolve(context.Background(), domain.FileReference{ReferenceID: name, Format: "EPUB"})
		if err == nil {
			_ = rc.Close()
			t.Errorf("Resolve(%q) = nil error, want NotFound", name)
			continue
		}
		if domain.CategoryOf(err) != domain.NotFound {
			t.Errorf("Resolve(%q) category = %s, want NotFound", name, domain.CategoryOf(err))
		}
	}
}

func TestNew_RejectsBadPathShape(t *testing.T) {
	if _, err := local.New("s", "relative/path", newCodec(t), nil); err == nil {
		t.Fatal("New accepted a relative basePath")
	}
}
