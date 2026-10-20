package companies

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // registers DecodeConfig for JPEG
	_ "image/png"  // registers DecodeConfig for PNG
	"net/http"
)

// Image size limits (manual "Company Logo", "Company Cover Banner").
const (
	MaxLogoBytes   = 2 << 20
	MaxBannerBytes = 5 << 20
	maxImageSide   = 8000
)

var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// CheckImage validates an upload by its bytes, never its name or the
// client's word alone (system requirements §18): the sniffed type must be
// JPEG, PNG or WebP and match the declared type. Returns the file extension.
func CheckImage(data []byte, declaredType string, maxBytes int) (string, error) {
	if len(data) == 0 {
		return "", invalid("image", "the file is empty")
	}
	if len(data) > maxBytes {
		return "", invalid("image", fmt.Sprintf("images can be at most %d MB", maxBytes>>20))
	}
	sniffed := http.DetectContentType(data)
	ext, ok := imageExt[sniffed]
	if !ok || sniffed != declaredType {
		return "", invalid("image", "upload a JPEG, PNG or WebP image")
	}
	// Bound pixel dimensions so a tiny file can't claim a gigantic image.
	// (Go's stdlib has no WebP decoder; WebP is size-capped only.)
	if sniffed != "image/webp" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImageSide || cfg.Height > maxImageSide {
			return "", invalid("image", fmt.Sprintf("images must be valid and at most %d×%d pixels", maxImageSide, maxImageSide))
		}
	}
	return ext, nil
}
