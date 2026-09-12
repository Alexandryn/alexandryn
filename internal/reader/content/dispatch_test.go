package content_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

func TestValidateResourcePath(t *testing.T) {
	bad := []string{"", "/etc/passwd", "../secret", "a/../../b", "OEBPS/../../x", `a\b`, "./a"}
	for _, p := range bad {
		if err := content.ValidateResourcePath(p); err == nil {
			t.Fatalf("path %q should be rejected", p)
		} else if domain.CategoryOf(err) != domain.InvalidInput {
			t.Fatalf("path %q: category = %v, want InvalidInput", p, domain.CategoryOf(err))
		}
	}
	good := []string{"OEBPS/chapter1.xhtml", "images/plate.png", "styles/main.css", "a/b/c.xhtml"}
	for _, p := range good {
		if err := content.ValidateResourcePath(p); err != nil {
			t.Fatalf("path %q should be accepted: %v", p, err)
		}
	}
}

func TestClassify(t *testing.T) {
	htmlBytes := []byte("<!DOCTYPE html><html><body><p>hi</p></body></html>")
	if k, ct, err := content.Classify("OEBPS/c1.xhtml", htmlBytes); err != nil || k != content.KindHTML {
		t.Fatalf("xhtml: kind=%v ct=%q err=%v", k, ct, err)
	}
	if k, _, err := content.Classify("styles/main.css", []byte("body{color:red}")); err != nil || k != content.KindCSS {
		t.Fatalf("css: kind=%v err=%v", k, err)
	}
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000000000000000")
	if k, ct, err := content.Classify("images/a.png", pngBytes); err != nil || k != content.KindBinary || ct != "image/png" {
		t.Fatalf("png: kind=%v ct=%q err=%v", k, ct, err)
	}
	// Standalone SVG is refused, not sanitised-and-served.
	if _, _, err := content.Classify("images/cover.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("svg should be InvalidInput, got %v", err)
	}
	// An embedded executable is refused.
	if _, _, err := content.Classify("x.bin", []byte("MZ\x90\x00\x03\x00\x00\x00")); domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("executable should be InvalidInput, got %v", err)
	}
	// A woff2 font serves as binary.
	if k, ct, err := content.Classify("fonts/f.woff2", []byte("wOF2\x00\x01\x00\x00")); err != nil || k != content.KindBinary || ct != "font/woff2" {
		t.Fatalf("woff2: kind=%v ct=%q err=%v", k, ct, err)
	}
}
