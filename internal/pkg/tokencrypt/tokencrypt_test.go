package tokencrypt

import (
	"bytes"
	"strings"
	"testing"
)

const (
	keyA = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=" // 32 bytes
	keyB = "HxwdHhscGRgXFhUUExIREA8ODQwLCgkIBwYFBAMCAQA="
)

func TestRoundTripAndRotation(t *testing.T) {
	old, err := Parse("k1:" + keyA)
	if err != nil {
		t.Fatal(err)
	}
	kid, sealed := old.Seal([]byte("refresh-token"), []byte("session-1"))
	if kid != "k1" || bytes.Contains(sealed, []byte("refresh-token")) {
		t.Fatalf("kid %q or plaintext visible in %x", kid, sealed)
	}

	// New key first: still opens k1 data, seals new data with k2.
	rotated, err := Parse("k2:" + keyB + ", k1:" + keyA)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rotated.Open(kid, sealed, []byte("session-1"))
	if err != nil || string(got) != "refresh-token" {
		t.Fatalf("open after rotation: %q, %v", got, err)
	}
	if kid2, _ := rotated.Seal([]byte("x"), nil); kid2 != "k2" {
		t.Fatalf("sealed with %q, want k2", kid2)
	}

	// Same plaintext twice gives different ciphertexts (random nonce).
	_, again := old.Seal([]byte("refresh-token"), []byte("session-1"))
	if bytes.Equal(sealed, again) {
		t.Fatal("nonce reused")
	}
}

func TestOpenRejects(t *testing.T) {
	k, _ := Parse("k1:" + keyA)
	kid, sealed := k.Seal([]byte("secret"), []byte("session-1"))
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1

	for name, tc := range map[string]struct {
		kid    string
		sealed []byte
		aad    string
	}{
		"other row (aad)": {kid, sealed, "session-2"},
		"tampered":        {kid, tampered, "session-1"},
		"unknown key id":  {"k9", sealed, "session-1"},
		"truncated":       {kid, sealed[:5], "session-1"},
	} {
		if _, err := k.Open(tc.kid, tc.sealed, []byte(tc.aad)); err != ErrDecrypt {
			t.Errorf("%s: want ErrDecrypt, got %v", name, err)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, spec := range []string{
		"",
		"nokey",
		"k1:not-base64!",
		"k1:" + keyA[:20],                    // short key
		"K1:" + keyA,                         // bad kid
		"k1:" + keyA + ",k1:" + keyB,         // duplicate
		strings.Repeat("a", 33) + ":" + keyA, // kid too long
	} {
		if _, err := Parse(spec); err == nil {
			t.Errorf("Parse(%q): want error", spec)
		}
	}
}
