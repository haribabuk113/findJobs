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
	"findJobs/internal/ports"
)

// Client is the FXProvider adapter.
type Client struct {
	url  string
	http *http.Client
}

// NewClient creates the provider; url returns rates with INR as the base,
// e.g. https://open.er-api.com/v6/latest/INR.
func NewClient(url string) ports.FXProvider {
	return &Client{url: url, http: &http.Client{Timeout: 10 * time.Second}}
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
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fx request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
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
