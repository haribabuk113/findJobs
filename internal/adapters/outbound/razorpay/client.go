// Package razorpay is the PaymentGateway adapter for Razorpay (D2): the
// Orders API, order.paid webhooks and the Payments list (reconciliation).
package razorpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"findJobs/internal/domain/payments"
	"findJobs/internal/ports"
)

// Client talks to the Razorpay API with the account's key pair.
type Client struct {
	keyID, keySecret, webhookSecret string
	baseURL                         string
	http                            *http.Client
}

// New creates the gateway. The webhook secret is set per webhook in the
// Razorpay dashboard and is separate from the key secret.
func New(keyID, keySecret, webhookSecret string) ports.PaymentGateway {
	return &Client{keyID: keyID, keySecret: keySecret, webhookSecret: webhookSecret,
		baseURL: "https://api.razorpay.com", http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) KeyID() string { return c.keyID }

func (c *Client) CreateOrder(ctx context.Context, amountPaise int64, receipt string) (string, error) {
	body, err := json.Marshal(map[string]any{"amount": amountPaise, "currency": "INR", "receipt": receipt})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/orders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("razorpay order: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("razorpay order returned %d: %s", resp.StatusCode, raw)
	}
	var order struct {
		ID     string `json:"id"`
		Amount int64  `json:"amount"`
	}
	if err := json.Unmarshal(raw, &order); err != nil || order.ID == "" || order.Amount != amountPaise {
		return "", fmt.Errorf("razorpay order: unexpected response %s", raw)
	}
	return order.ID, nil
}

type webhook struct {
	Event   string `json:"event"`
	Payload struct {
		Order struct {
			Entity struct {
				ID         string `json:"id"`
				AmountPaid int64  `json:"amount_paid"`
				Currency   string `json:"currency"`
				Status     string `json:"status"`
			} `json:"entity"`
		} `json:"order"`
		Payment struct {
			Entity struct {
				ID string `json:"id"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

// ParseWebhook checks X-Razorpay-Signature (hex HMAC-SHA256 of the raw body
// with the webhook secret). Only order.paid credits anything: Razorpay sends
// it once the order is captured in full, unlike payment.authorized.
func (c *Client) ParseWebhook(body []byte, signature string) (*payments.Paid, error) {
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(body)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(signature)) {
		return nil, payments.ErrBadSignature
	}
	var w webhook
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("razorpay webhook: %w", err)
	}
	order := w.Payload.Order.Entity
	if w.Event != "order.paid" || order.Status != "paid" {
		return nil, nil
	}
	return &payments.Paid{OrderID: order.ID, PaymentID: w.Payload.Payment.Entity.ID,
		AmountPaise: order.AmountPaid, Currency: order.Currency}, nil
}

// paymentsPage is Razorpay's maximum page size for GET /v1/payments.
const paymentsPage = 100

// CapturedPayments pages through GET /v1/payments for [from, to) and keeps the
// captured ones that belong to an order. A refunded or failed payment never
// counts as paid.
// ponytail: stops at 100 pages (10,000 payments a window); page by day if volume gets there.
func (c *Client) CapturedPayments(ctx context.Context, from, to time.Time) ([]payments.Paid, error) {
	var paid []payments.Paid
	for skip := 0; ; skip += paymentsPage {
		if skip >= 100*paymentsPage {
			return nil, fmt.Errorf("razorpay payments: more than %d in the window", skip)
		}
		q := url.Values{"from": {strconv.FormatInt(from.Unix(), 10)}, "to": {strconv.FormatInt(to.Unix()-1, 10)},
			"count": {strconv.Itoa(paymentsPage)}, "skip": {strconv.Itoa(skip)}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/payments?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.keyID, c.keySecret)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("razorpay payments: %w", err)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("razorpay payments returned %d: %s", resp.StatusCode, raw)
		}
		var page struct {
			Items []struct {
				ID       string `json:"id"`
				OrderID  string `json:"order_id"`
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
				Status   string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("razorpay payments: %w", err)
		}
		for _, p := range page.Items {
			if p.Status == "captured" && p.OrderID != "" {
				paid = append(paid, payments.Paid{OrderID: p.OrderID, PaymentID: p.ID, AmountPaise: p.Amount, Currency: p.Currency})
			}
		}
		if len(page.Items) < paymentsPage {
			return paid, nil
		}
	}
}
