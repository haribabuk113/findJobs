// Package fx fetches exchange rates from ExchangeRate-API's open endpoint
// (open.er-api.com: no key, daily updates; its terms require attribution,
// shown on the About page).
package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"findJobs/internal/domain/reference"
	"findJobs/internal/pkg/resilience"
	"findJobs/internal/ports"
)

// package-level HTTP client for connection reuse
var client = &http.Client{Timeout: 10 * time.Second}

// Client is the FXProvider adapter.
type Client struct {
	url  string
	http *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient overrides the HTTP client used by the adapter.
// Useful for injecting mock clients in tests or custom timeouts per environment.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.http = hc }
}

// NewClient creates the provider; url returns rates with INR as the base,
// e.g. https://open.er-api.com/v6/latest/INR.
// Optional options can override the default HTTP client (see WithHTTPClient).
func NewClient(url string, opts ...ClientOption) ports.FXProvider {
	c := &Client{url: url, http: client}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type response struct {
	Result   string             `json:"result"`
	BaseCode string             `json:"base_code"`
	Rates    map[string]float64 `json:"rates"` // units of currency per 1 INR
}

func (c *Client) Latest(ctx context.Context) (reference.Rates, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := resilience.Do(ctx, c.http, req, resilience.WithCircuitBreaker(), resilience.WithRetryConfig())
	if err != nil {
		return nil, fmt.Errorf("fx request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) // ensure connection can be reused
		return nil, fmt.Errorf("fx provider returned %d", resp.StatusCode)
	}
	var body response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("fx decode: %w", err)
	}
	if body.Result != "success" || body.BaseCode != "INR" {
		return nil, fmt.Errorf("fx provider result %q base %q", body.Result, body.BaseCode)
	}
	// Invert to INR per one unit, the form salaries are multiplied by.
	rates := make(reference.Rates, len(body.Rates))
	for code, perINR := range body.Rates {
		if perINR > 0 {
			rates[code] = 1 / perINR
		}
	}
	rates["INR"] = 1
	return rates, rates.Validate()
}
