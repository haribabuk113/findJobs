package profiles

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipOf(t *testing.T, names ...string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, n := range names {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("x"))
	}
	z.Close()
	return b.Bytes()
}

func TestCheckResume(t *testing.T) {
	pdf := []byte("%PDF-1.7\n...")
	docx := zipOf(t, "[Content_Types].xml", "word/document.xml")
	for _, c := range []struct {
		name, contentType, ext string
		data                   []byte
	}{
		{"pdf", PDFType, "pdf", pdf},
		{"docx", DOCXType, "docx", docx},
		{"pdf declared as docx", DOCXType, "", pdf},
		{"html as pdf", PDFType, "", []byte("<html><script>")},
		{"plain zip as docx", DOCXType, "", zipOf(t, "a.txt")},
		{"xlsx as docx", DOCXType, "", zipOf(t, "[Content_Types].xml", "xl/workbook.xml")},
		{"image", "image/png", "", pdf},
		{"empty", PDFType, "", nil},
		{"too big", PDFType, "", append([]byte("%PDF-"), strings.Repeat("a", MaxResumeBytes)...)},
	} {
		ext, err := CheckResume(c.data, c.contentType)
		if ext != c.ext || (c.ext == "") != (err != nil) {
			t.Errorf("%s: %q %v", c.name, ext, err)
		}
	}
}

func TestPublicHidesResume(t *testing.T) {
	p := Profile{Resume: &Resume{Format: "pdf"}, ResumeVisibility: VisibleApplied}
	if p.Public().Resume != nil {
		t.Fatal("an Applied Jobs Only résumé is public")
	}
	p.ResumeVisibility = VisibleAll
	if p.Public().Resume == nil {
		t.Fatal("a Visible to All résumé is hidden")
	}
}
