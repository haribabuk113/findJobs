package razorpay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"findJobs/internal/domain/payments"
)

func TestCapturedPayments(t *testing.T) {
	from, to := time.Unix(1_700_000_000, 0), time.Unix(1_700_086_400, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		q := r.URL.Query()
		if r.URL.Path != "/v1/payments" || user != "rzp_test_key" || pass != "key-secret" ||
			q.Get("from") != "1700000000" || q.Get("to") != "1700086399" || q.Get("count") != "100" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var items []map[string]any
		if skip, _ := strconv.Atoi(q.Get("skip")); skip == 0 {
			// A full first page: 98 captured, one refunded, one with no order.
			for i := range 98 {
				items = append(items, map[string]any{"id": fmt.Sprint("pay_", i), "order_id": fmt.Sprint("order_", i), "amount": 100, "currency": "INR", "status": "captured"})
			}
			items = append(items, map[string]any{"id": "pay_r", "order_id": "order_r", "amount": 100, "currency": "INR", "status": "refunded"},
				map[string]any{"id": "pay_n", "order_id": "", "amount": 100, "currency": "INR", "status": "captured"})
		} else {
			items = append(items, map[string]any{"id": "pay_last", "order_id": "order_last", "amount": 250, "currency": "INR", "status": "captured"},
				map[string]any{"id": "pay_f", "order_id": "order_f", "amount": 250, "currency": "INR", "status": "failed"})
		}
		json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": len(items), "items": items})
	}))
	defer srv.Close()
	c := New("rzp_test_key", "key-secret", "hook-secret").(*Client)
	c.baseURL = srv.URL

	got, err := c.CapturedPayments(context.Background(), from, to)
	if err != nil || len(got) != 99 {
		t.Fatalf("CapturedPayments = %d payments, %v; want 99", len(got), err)
	}
	if last := got[98]; last != (payments.Paid{OrderID: "order_last", PaymentID: "pay_last", AmountPaise: 250, Currency: "INR"}) {
		t.Fatalf("last = %+v", last)
	}
}
