package extract_test

import (
	"context"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

var validPDF = []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
4 0 obj
<< /Title (Foundation) /Author (Isaac Asimov) /Subject (Science Fiction) >>
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000186 00000 n 
trailer
<< /Size 5 /Root 1 0 R /Info 4 0 R >>
startxref
263
%%EOF
`)

func TestExtractPDF_ValidPDF(t *testing.T) {
	ctx := context.Background()

	tmp := createTempFile(t, validPDF)

	meta, err := extract.Extract(ctx, extract.FormatPDF, tmp)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if meta.Title != "Foundation" {
		t.Errorf("Title = %q, want Foundation", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Isaac Asimov" {
		t.Errorf("Authors = %+v, want [Isaac Asimov]", meta.Authors)
	}
	if meta.Description == nil || *meta.Description != "Science Fiction" {
		t.Errorf("Description = %v, want Science Fiction", meta.Description)
	}
	if meta.Format != extract.FormatPDF {
		t.Errorf("Format = %v, want pdf", meta.Format)
	}
	if meta.CoverBytes != nil {
		t.Error("expected no cover bytes for PDF")
	}
}

func TestExtractPDF_MissingTitleIsErrNoTitle(t *testing.T) {
	ctx := context.Background()

	pdfWithoutTitle := []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
4 0 obj
<< /Author (Isaac Asimov) >>
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000186 00000 n 
trailer
<< /Size 5 /Root 1 0 R /Info 4 0 R >>
startxref
236
%%EOF
`)

	tmp := createTempFile(t, pdfWithoutTitle)

	_, err := extract.Extract(ctx, extract.FormatPDF, tmp)
	if err != extract.ErrNoTitle {
		t.Fatalf("err = %v, want ErrNoTitle", err)
	}
}

func TestExtractPDF_CorruptedPDFIsErrMalformed(t *testing.T) {
	ctx := context.Background()

	tmp := createTempFile(t, []byte("%PDF-1.4\ncorrupted content truncated"))

	_, err := extract.Extract(ctx, extract.FormatPDF, tmp)
	if err != extract.ErrMalformed {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}
