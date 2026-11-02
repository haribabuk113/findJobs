package ports

import (
	"context"
	"time"

	"findJobs/internal/domain/payments"
)

// WalletRepository persists wallets, their ledger and gateway payments. A
// company owner must be a current admin of a live company (companies.ErrNotFound
// or companies.ErrNotAdmin otherwise).
type WalletRepository interface {
	// Wallet reads the owner's balance and a page of its ledger, newest first.
	// A wallet never topped up reads as empty. pageSize 0 reads every entry.
	Wallet(ctx context.Context, owner payments.Owner, page, pageSize int) (*payments.Wallet, error)
	// OpenWallet returns the owner's wallet id, creating the wallet if needed.
	OpenWallet(ctx context.Context, owner payments.Owner) (walletID string, err error)
	// CreatePayment records a gateway order before it's paid.
	CreatePayment(ctx context.Context, p payments.Payment) error
	// CompletePayment applies a paid order exactly once: a top-up credits its
	// wallet (with its ledger row), a job post publishes its pending job. An
	// order already paid is a no-op (applied false); ErrUnknownOrder or
	// ErrAmountMismatch otherwise.
	CompletePayment(ctx context.Context, paid payments.Paid) (applied bool, err error)
	// UnledgeredTopups returns the order ids of top-ups paid since since whose
	// ledger row is missing or has a different amount (reconciliation).
	UnledgeredTopups(ctx context.Context, since time.Time) ([]string, error)
}

// PaymentGateway is the online payment provider (Razorpay, D2).
type PaymentGateway interface {
	// KeyID is the public key the checkout widget opens with.
	KeyID() string
	// CreateOrder creates an INR order; receipt is our payment id.
	CreateOrder(ctx context.Context, amountPaise int64, receipt string) (orderID string, err error)
	// ParseWebhook verifies a webhook's signature (ErrBadSignature) and
	// returns the order it reports paid, or nil for events we don't act on.
	ParseWebhook(body []byte, signature string) (*payments.Paid, error)
	// CapturedPayments lists the payments created in [from, to) that were
	// captured, each with its order, for reconciliation.
	CapturedPayments(ctx context.Context, from, to time.Time) ([]payments.Paid, error)
}
