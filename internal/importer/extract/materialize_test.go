package extract_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

func TestMaterialize_SmallStream(t *testing.T) {
	ctx := context.Background()
	data := []byte("hello world metadata stream content")
	f, cleanup, err := extract.Materialize(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	defer cleanup()

	read, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(read, data) {
		t.Fatalf("read = %q, want %q", read, data)
	}
}

func TestMaterialize_OversizedStreamRejected(t *testing.T) {
	ctx := context.Background()
	// Create an infinite zero reader
	infReader := &zeroReader{}
	_, _, err := extract.Materialize(ctx, infReader)
	if err != extract.ErrOversized {
		t.Fatalf("err = %v, want ErrOversized", err)
	}
}

func TestMaterialize_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f, cleanup, err := extract.Materialize(ctx, bytes.NewReader([]byte("test")))
	if err == nil {
		cleanup()
		_ = f.Close()
		t.Fatal("expected context error on cancelled context")
	}
}

type zeroReader struct{}

func (z *zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
