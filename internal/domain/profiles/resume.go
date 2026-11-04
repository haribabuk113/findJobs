package profiles

import (
	"archive/zip"
	"bytes"
	"errors"
	"path"
	"time"
)

// MaxResumeBytes caps résumé uploads.
const MaxResumeBytes = 5 << 20

// Résumé upload types.
const (
	PDFType  = "application/pdf"
	DOCXType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// ResumeTypes maps each accepted extension to its content type.
var ResumeTypes = map[string]string{"pdf": PDFType, "docx": DOCXType}

// Resume says that the profile has a résumé (read-only; set with PUT /profile/me/resume).
type Resume struct {
	Format     string    `json:"format"` // pdf or docx
	UploadedAt time.Time `json:"uploaded_at"`
}

// NewResume describes the stored file behind key, or nil without one.
func NewResume(key *string, uploadedAt *time.Time) *Resume {
	if key == nil || uploadedAt == nil {
		return nil
	}
	return &Resume{Format: path.Ext(*key)[1:], UploadedAt: *uploadedAt}
}

// ResumeFile is a résumé ready to stream.
type ResumeFile struct {
	OwnerID     string
	FileName    string // <slug>-resume.<ext>: never the uploader's own file name
	ContentType string
	Data        []byte
}

// CheckResume checks that data really is a PDF or DOCX of the declared type
// and returns its extension. The bytes decide, never the name or the header alone.
func CheckResume(data []byte, contentType string) (string, error) {
	switch {
	case len(data) == 0:
		return "", errors.New("the file is empty")
	case len(data) > MaxResumeBytes:
		return "", errors.New("the résumé must be 5 MB or smaller")
	case contentType == PDFType && bytes.HasPrefix(data, []byte("%PDF-")):
		return "pdf", nil
	case contentType == DOCXType && isDOCX(data):
		return "docx", nil
	case contentType != PDFType && contentType != DOCXType:
		return "", errors.New("upload a PDF or DOCX file")
	}
	return "", errors.New("the file isn't the PDF or DOCX it claims to be")
}

// isDOCX reads only the zip directory (nothing is decompressed, so a zip
// bomb costs nothing) and looks for the Word document part.
func isDOCX(data []byte) bool {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	var types, doc bool
	for _, f := range z.File {
		types = types || f.Name == "[Content_Types].xml"
		doc = doc || f.Name == "word/document.xml"
	}
	return types && doc
}
