package payments

import (
	"context"
	"testing"
	"time"

	"findJobs/internal/domain/payments"
	"findJobs/internal/ports"
)

// fakeRepo knows two orders: order_new (created, 500) and order_old (paid).
type fakeRepo struct {
	ports.WalletRepository
	paid       map[string]bool
	unledgered []string
}

func (r *fakeRepo) CompletePayment(_ context.Context, p payments.Paid) (bool, error) {
	switch {
	case p.OrderID != "order_new" && p.OrderID != "order_old":
		return false, payments.ErrUnknownOrder
	case p.OrderID == "order_new" && p.AmountPaise != 500:
		return false, payments.ErrAmountMismatch
	case r.paid[p.OrderID]:
		return false, nil
	}
	r.paid[p.OrderID] = true
	return true, nil
}

func (r *fakeRepo) UnledgeredTopups(context.Context, time.Time) ([]string, error) {
	return r.unledgered, nil
}

type fakeGateway struct {
	ports.PaymentGateway
	captured []payments.Paid
	from, to time.Time
}

func (g *fakeGateway) CapturedPayments(_ context.Context, from, to time.Time) ([]payments.Paid, error) {
	g.from, g.to = from, to
	return g.captured, nil
}

func TestReconcile(t *testing.T) {
	repo := &fakeRepo{paid: map[string]bool{"order_old": true}, unledgered: []string{"order_x"}}
	gw := &fakeGateway{captured: []payments.Paid{
		{OrderID: "order_new", AmountPaise: 500, Currency: "INR"}, // webhook missed
		{OrderID: "order_old", AmountPaise: 900, Currency: "INR"}, // already paid
		{OrderID: "order_other_app", AmountPaise: 100, Currency: "INR"},
	}}
	s := NewPaymentService(repo, gw).(*service)
	now := time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)

	r, err := s.reconcile(context.Background(), now)
	want := payments.Reconciliation{Checked: 3, Applied: 1, Unknown: 1, Unledgered: 1}
	if err != nil || r != want {
		t.Fatalf("reconcile = %+v, %v; want %+v", r, err, want)
	}
	if !gw.from.Equal(now.Add(-7*24*time.Hour)) || !gw.to.Equal(now.Add(-time.Hour)) {
		t.Fatalf("window = %v to %v", gw.from, gw.to)
	}
	// A second run applies nothing new.
	if r, _ := s.reconcile(context.Background(), now); r.Applied != 0 {
		t.Fatalf("second run applied %d", r.Applied)
	}
	// A short capture for a known order is flagged, never credited.
	gw.captured = []payments.Paid{{OrderID: "order_new", AmountPaise: 499, Currency: "INR"}}
	repo.paid = map[string]bool{}
	if r, _ := s.reconcile(context.Background(), now); r.Mismatched != 1 || repo.paid["order_new"] {
		t.Fatalf("mismatch run = %+v, paid %v", r, repo.paid)
	}
	// Without a gateway there's nothing to reconcile.
	if err := NewPaymentService(repo, nil).Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
}
