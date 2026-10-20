// Package tokencrypt seals short secrets (IAM tokens) with AES-256-GCM before
// they are stored, so a database reader or a leaked backup can't use them.
package tokencrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrDecrypt means the key id is unknown or the ciphertext/AAD was altered.
var ErrDecrypt = errors.New("tokencrypt: cannot decrypt")

var keyID = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// Keyring holds the encryption keys. The first key encrypts; every key
// decrypts, so a new key can be put first and old ones retired later.
type Keyring struct {
	current string
	aeads   map[string]cipher.AEAD
}

// Parse reads "kid:base64key[,kid:base64key...]"; each key is 32 random bytes
// (e.g. `openssl rand -base64 32`).
func Parse(spec string) (*Keyring, error) {
	k := &Keyring{aeads: map[string]cipher.AEAD{}}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kid, encoded, ok := strings.Cut(entry, ":")
		if !ok || !keyID.MatchString(kid) {
			return nil, fmt.Errorf("tokencrypt: entry must be kid:base64key with kid matching %s", keyID)
		}
		if _, dup := k.aeads[kid]; dup {
			return nil, fmt.Errorf("tokencrypt: duplicate key id %q", kid)
		}
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("tokencrypt: key %q must be 32 bytes, base64-encoded", kid)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		k.aeads[kid] = aead
		if k.current == "" {
			k.current = kid
		}
	}
	if k.current == "" {
		return nil, errors.New("tokencrypt: no keys configured")
	}
	return k, nil
}

// Seal encrypts plaintext bound to aad (e.g. the row it belongs to, so the
// ciphertext can't be moved to another row). Returns the key id and
// nonce||ciphertext.
func (k *Keyring) Seal(plaintext, aad []byte) (string, []byte) {
	aead := k.aeads[k.current]
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce) // never fails (crypto/rand panics instead)
	return k.current, aead.Seal(nonce, nonce, plaintext, aad)
}

// Open decrypts what Seal produced with the same aad.
func (k *Keyring) Open(kid string, sealed, aad []byte) ([]byte, error) {
	aead, ok := k.aeads[kid]
	if !ok || len(sealed) < aead.NonceSize() {
		return nil, ErrDecrypt
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
