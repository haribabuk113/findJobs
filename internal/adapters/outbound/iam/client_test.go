package iam

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"findJobs/internal/domain/auth"
	"findJobs/internal/pkg/config"
)

const (
	testIssuer   = "https://auth.test"
	testAudience = "app-registry"
)

// jwksServer serves keys (kid -> public key) and counts JWKS fetches.
func jwksServer(t *testing.T, keys map[string]ed25519.PublicKey, fetches *atomic.Int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		fetches.Add(1)
		type jwk struct{ Kty, Crv, Kid, X string }
		var out struct {
			Keys []jwk `json:"keys"`
		}
		for kid, k := range keys {
			out.Keys = append(out.Keys, jwk{"OKP", "Ed25519", kid, base64.RawURLEncoding.EncodeToString(k)})
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "eco-123", "email": "asha@example.com", "name": "Asha", "status": "active",
		"iss": testIssuer, "aud": testAudience, "exp": time.Now().Add(time.Hour).Unix(),
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, kid string, c jwt.MapClaims) string {
	tok := jwt.NewWithClaims(method, c)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerify(t *testing.T) {
	pub, priv := newKey(t)
	_, otherPriv := newKey(t)
	var fetches atomic.Int32
	srv := jwksServer(t, map[string]ed25519.PublicKey{"k1": pub}, &fetches)
	c := NewClient(config.IAMConfig{Endpoint: srv.URL, Issuer: testIssuer, Audience: testAudience})
	ctx := context.Background()

	got, err := c.Verify(ctx, sign(t, jwt.SigningMethodEdDSA, priv, "k1", validClaims()))
	if err != nil || got.EcosystemID != "eco-123" || got.Email != "asha@example.com" || got.Status != "active" {
		t.Fatalf("valid token: %+v %v", got, err)
	}

	with := func(key, value any) jwt.MapClaims {
		c := validClaims()
		if value == nil {
			delete(c, key.(string))
		} else {
			c[key.(string)] = value
		}
		return c
	}
	for name, token := range map[string]string{
		"wrong issuer":      sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("iss", "https://evil.test")),
		"wrong audience":    sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("aud", "other-app")),
		"expired":           sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("exp", time.Now().Add(-time.Minute).Unix())),
		"no expiry":         sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("exp", nil)),
		"missing sub":       sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("sub", nil)),
		"empty sub":         sign(t, jwt.SigningMethodEdDSA, priv, "k1", with("sub", "")),
		"other key":         sign(t, jwt.SigningMethodEdDSA, otherPriv, "k1", validClaims()),
		"unknown kid":       sign(t, jwt.SigningMethodEdDSA, priv, "k9", validClaims()),
		"HS256 with pubkey": sign(t, jwt.SigningMethodHS256, []byte(pub), "k1", validClaims()), // algorithm confusion
		"none":              sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "k1", validClaims()),
		"garbage":           "not.a.jwt",
	} {
		if _, err := c.Verify(ctx, token); err != auth.ErrUnauthorized {
			t.Errorf("%s: want ErrUnauthorized, got %v", name, err)
		}
	}
}

// Keys are cached; an unknown kid triggers one refetch (IAM publishes a
// rotated key before signing with it).
func TestVerifyRefetchesOnRotation(t *testing.T) {
	pub1, priv1 := newKey(t)
	pub2, priv2 := newKey(t)
	keys := map[string]ed25519.PublicKey{"k1": pub1}
	var fetches atomic.Int32
	srv := jwksServer(t, keys, &fetches)
	c := NewClient(config.IAMConfig{Endpoint: srv.URL, Issuer: testIssuer, Audience: testAudience})
	ctx := context.Background()

	for range 3 {
		if _, err := c.Verify(ctx, sign(t, jwt.SigningMethodEdDSA, priv1, "k1", validClaims())); err != nil {
			t.Fatal(err)
		}
	}
	if fetches.Load() != 1 {
		t.Fatalf("JWKS fetched %d times for one key, want 1", fetches.Load())
	}
	keys["k2"] = pub2 // rotation
	if _, err := c.Verify(ctx, sign(t, jwt.SigningMethodEdDSA, priv2, "k2", validClaims())); err != nil {
		t.Fatalf("rotated key: %v", err)
	}
	if fetches.Load() != 2 {
		t.Fatalf("fetches after rotation %d, want 2", fetches.Load())
	}
}
