package razorpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"findJobs/internal/domain/payments"
)

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestParseWebhook(t *testing.T) {
	c := New("rzp_test_key", "key-secret", "hook-secret")
	paid := `{"event":"order.paid","payload":{"payment":{"entity":{"id":"pay_1","order_id":"order_1","amount":50000}},
		"order":{"entity":{"id":"order_1","amount":50000,"amount_paid":50000,"currency":"INR","status":"paid"}}}}`

	got, err := c.ParseWebhook([]byte(paid), sign("hook-secret", paid))
	if err != nil || got == nil || *got != (payments.Paid{OrderID: "order_1", PaymentID: "pay_1", AmountPaise: 50000, Currency: "INR"}) {
		t.Fatalf("order.paid = %+v, %v", got, err)
	}
	// Signed with the key secret instead of the webhook secret, or tampered.
	for _, sig := range []string{sign("key-secret", paid), sign("hook-secret", paid+" "), "", "zz"} {
		if _, err := c.ParseWebhook([]byte(paid), sig); err != payments.ErrBadSignature {
			t.Errorf("signature %q: err = %v, want ErrBadSignature", sig, err)
		}
	}
	// Valid but not an event we act on.
	authorized := `{"event":"payment.authorized","payload":{"payment":{"entity":{"id":"pay_1"}}}}`
	if got, err := c.ParseWebhook([]byte(authorized), sign("hook-secret", authorized)); got != nil || err != nil {
		t.Errorf("payment.authorized = %+v, %v; want ignored", got, err)
	}
}

func TestCreateOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		if r.URL.Path != "/v1/orders" || user != "rzp_test_key" || pass != "key-secret" || in["currency"] != "INR" || in["receipt"] != "r1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "order_9", "amount": in["amount"]})
	}))
	defer srv.Close()
	c := New("rzp_test_key", "key-secret", "hook-secret").(*Client)
	c.baseURL = srv.URL
	if id, err := c.CreateOrder(context.Background(), 1000, "r1"); err != nil || id != "order_9" {
		t.Fatalf("CreateOrder = %q, %v", id, err)
	}
	if _, err := c.CreateOrder(context.Background(), 1000, "other"); err == nil {
		t.Fatal("a 400 from Razorpay should fail")
	}
}
