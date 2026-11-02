package payments

import (
	"context"
	"errors"
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
	duplicated []string
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

func (r *fakeRepo) DuplicateJobPayments(context.Context, time.Time) ([]string, error) {
	return r.duplicated, nil
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
	repo := &fakeRepo{paid: map[string]bool{"order_old": true}, unledgered: []string{"order_x"}, duplicated: []string{"order_y"}}
	gw := &fakeGateway{captured: []payments.Paid{
		{OrderID: "order_new", AmountPaise: 500, Currency: "INR"}, // webhook missed
		{OrderID: "order_old", AmountPaise: 900, Currency: "INR"}, // already paid
		{OrderID: "order_other_app", AmountPaise: 100, Currency: "INR"},
	}}
	s := NewPaymentService(repo, gw).(*service)
	now := time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)

	r, err := s.reconcile(context.Background(), now)
	want := payments.Reconciliation{Checked: 3, Applied: 1, Unknown: 1, Unledgered: 1, Duplicated: 1}
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

// failingRepo opens wallets but can't record payments (the DB fails after the
// gateway order exists).
type failingRepo struct{ ports.WalletRepository }

func (failingRepo) OpenWallet(context.Context, payments.Owner) (string, error) { return "w1", nil }
func (failingRepo) CreatePayment(context.Context, payments.Payment) error {
	return errors.New("db down")
}

type orderGateway struct {
	ports.PaymentGateway
	orders int
}

func (g *orderGateway) CreateOrder(context.Context, int64, string) (string, error) {
	g.orders++
	return "order_1", nil
}
func (g *orderGateway) KeyID() string { return "rzp_test" }

func TestCheckoutWhenSavingFails(t *testing.T) {
	gw := &orderGateway{}
	s := NewPaymentService(failingRepo{}, gw)
	// No checkout is returned, so the browser never opens an order we didn't
	// record. The order itself is left unpaid, which costs nothing; if it were
	// paid anyway, the webhook and reconciliation report it as an unknown order.
	if c, err := s.Topup(context.Background(), payments.Owner{UserID: "u1"}, 10_000); err == nil || c != nil {
		t.Fatalf("Topup = %+v, %v; want an error and no checkout", c, err)
	}
	if c, err := s.JobPostCheckout(context.Background(), "u1", "job1", 200); err == nil || c != nil {
		t.Fatalf("JobPostCheckout = %+v, %v; want an error and no checkout", c, err)
	}
	if gw.orders != 2 {
		t.Fatalf("orders created: %d", gw.orders)
	}
}
