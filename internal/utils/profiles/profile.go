package profiles

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

const MaxPhotoSize = 5 << 20 // 5 MB

var AllowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var SlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

var reserved = map[string]bool{
	"admin": true, "api": true, "login": true, "logout": true,
	"signup": true, "settings": true, "me": true, "new": true,
	"profile": true, "auth": true, "vol": true, "v1": true,
}

// randomSuffix returns n random chars from [a-z0-9] using crypto/rand.
func randomSuffix(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			idx = big.NewInt(0)
		}
		sb.WriteByte(alphabet[idx.Int64()])
	}
	return sb.String()
}

// baseSlug converts a display name to a URL-safe slug.
// "José Álvarez!" -> "jose-alvarez"
func BaseSlug(name string) string {
	base := slug.Make(name)
	if base == "" || reserved[base] {
		// Empty (e.g. emoji-only name) or reserved word: fall back.
		base = "user"
	}
	base += "-" + randomSuffix(4)
	return base
}
