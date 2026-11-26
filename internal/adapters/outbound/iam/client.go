// Package iam is the outbound adapter to the central IAM service: it redeems
// login codes, rotates/revokes refresh tokens, and verifies IAM access tokens
// locally against IAM's JWKS.
package iam

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"findJobs/internal/domain/auth"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/resilience"
	"findJobs/internal/ports"
)

// package-level HTTP client for connection reuse
var client = &http.Client{Timeout: 5 * time.Second}

type Client struct {
	cfg  config.IAMConfig
	http *http.Client

	mu   sync.Mutex
	keys map[string]ed25519.PublicKey // kid -> key, refetched on unknown kid
}

// NewClient creates the IAM adapter.
func NewClient(cfg config.IAMConfig) ports.IAMClient {
	return &Client{cfg: cfg, http: client}
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*auth.IAMTokens, error) {
	return c.tokens(ctx, "/token", map[string]string{"code": code, "app_id": c.cfg.AppID})
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (*auth.IAMTokens, error) {
	return c.tokens(ctx, "/refresh", map[string]string{"refresh_token": refreshToken, "app_id": c.cfg.AppID})
}

func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	resp, err := c.post(ctx, "/logout", map[string]string{"refresh_token": refreshToken})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("iam: logout returned %d", resp.StatusCode)
	}
	return nil
}

// tokens calls /token or /refresh. Any 4xx (expired/reused code or token,
// suspended account) means "not authenticated"; 5xx/network is an internal error.
func (c *Client) tokens(ctx context.Context, path string, body any) (*auth.IAMTokens, error) {
	resp, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, auth.ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iam: %s returned %d", path, resp.StatusCode)
	}
	var t auth.IAMTokens
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, fmt.Errorf("iam: decode %s: %w", path, err)
	}
	return &t, nil
}

func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return resilience.Do(ctx, c.http, req, resilience.WithCircuitBreaker(), resilience.WithRetryConfig())
}

type claims struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
	jwt.RegisteredClaims
}

func (c *Client) Verify(ctx context.Context, accessToken string) (*auth.IAMClaims, error) {
	var cl claims
	_, err := jwt.ParseWithClaims(accessToken, &cl, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return c.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(c.cfg.Issuer),
		jwt.WithAudience(c.cfg.Audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil || cl.Subject == "" {
		return nil, auth.ErrUnauthorized
	}
	return &auth.IAMClaims{EcosystemID: cl.Subject, Email: cl.Email, Name: cl.Name, Status: cl.Status}, nil
}

// key returns the JWKS key for kid, refetching JWKS once when kid is unknown
// (IAM key rotation publishes the new key before signing with it).
//
// The lock is released before the outbound HTTP fetch so that concurrent
// token verifications are not serialized behind a network call. A double-checked
// locking pattern is used: after fetching, re-acquire the lock and re-check the
// cache because another goroutine may have already populated the key.
func (c *Client) key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	c.mu.Lock()
	if k, ok := c.keys[kid]; ok {
		c.mu.Unlock()
		return k, nil
	}
	c.mu.Unlock()

	keys, err := c.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Another goroutine may have already populated the key while we fetched.
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	c.keys = keys
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("iam: unknown kid %q", kid)
}

func (c *Client) fetchJWKS(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.Endpoint+"/.well-known/jwks.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := resilience.Do(ctx, c.http, req, resilience.WithCircuitBreaker(), resilience.WithRetryConfig())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iam: jwks returned %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kty, Crv, Kid, X string
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("iam: decode jwks: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range jwks.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			continue
		}
		keys[k.Kid] = ed25519.PublicKey(x)
	}
	if len(keys) == 0 {
		// Do not cache an empty result: an empty map would be treated as a
		// poisoned cache for every kid and no refetch would ever happen.
		return nil, fmt.Errorf("iam: jwks contains no valid Ed25519 keys")
	}
	return keys, nil
}
