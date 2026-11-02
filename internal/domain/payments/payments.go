// Package payments is wallets, their ledger and gateway payments (manual: an
// INR wallet, top-ups, non-refundable deposits). Money is int64 paise.
package payments

import "time"

// Top-up limits: ₹1 is the gateway's minimum; ₹5,00,000 is Razorpay's
// default per-payment cap.
const (
	MinTopupPaise = 100
	MaxTopupPaise = 50_000_000
)

const MaxPageSize = 50

// Owner says whose wallet: UserID's own when CompanySlug is empty, otherwise
// that company's, which only its current admins may read or top up.
type Owner struct {
	UserID      string
	CompanySlug string
}

// Wallet is a balance and one page of its ledger, newest first.
type Wallet struct {
	BalancePaise int64   `json:"balance_paise"`
	Entries      []Entry `json:"entries"`
	Page         int     `json:"page"`
	PageSize     int     `json:"page_size"`
	Total        int     `json:"total"`
}

// Entry is one ledger row: a credit (positive) or debit (negative).
type Entry struct {
	ID                int64     `json:"id"`
	Kind              string    `json:"kind"` // topup | apply | job_post | match_unlock | refund
	AmountPaise       int64     `json:"amount_paise"`
	BalanceAfterPaise int64     `json:"balance_after_paise"`
	ActorName         string    `json:"actor_name"` // who topped up or spent (matters on company wallets)
	RefType           *string   `json:"ref_type"`
	RefID             *string   `json:"ref_id"`
	CreatedAt         time.Time `json:"created_at"`
}

// Checkout is what the browser's checkout widget needs to collect a payment.
type Checkout struct {
	Gateway     string `json:"gateway"` // razorpay
	KeyID       string `json:"key_id"`
	OrderID     string `json:"order_id"`
	AmountPaise int64  `json:"amount_paise"`
	Currency    string `json:"currency"`
}

// Payment is a gateway order recorded before the user pays it: a top-up of
// WalletID, or the fee for the pending job JobID.
type Payment struct {
	ID          string
	OrderID     string
	WalletID    string
	JobID       string
	UserID      string
	AmountPaise int64
}

// Paid is the gateway's report that an order was paid in full.
type Paid struct {
	OrderID     string
	PaymentID   string
	AmountPaise int64
	Currency    string
}

// Reconciliation counts what one reconciliation run found.
type Reconciliation struct {
	Checked    int // captured gateway payments in the window
	Applied    int // paid orders whose webhook never arrived, applied now
	Unknown    int // payments for orders we never created (maybe another app's)
	Mismatched int // paid amount or currency differs from the order
	Unledgered int // paid top-ups without their matching ledger row
	Duplicated int // extra paid orders for a job post that was already paid (a double charge)
}

// ValidateTopup checks a top-up amount.
func ValidateTopup(amountPaise int64) error {
	if amountPaise < MinTopupPaise || amountPaise > MaxTopupPaise {
		return &ValidationError{Field: "amount_paise", Message: "amount_paise must be between 100 (₹1) and 50000000 (₹5,00,000)"}
	}
	return nil
}

// NormalizePage fills the default page (1) and size (20) and checks bounds.
func NormalizePage(page, size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || page > 1000 || size < 1 || size > MaxPageSize {
		return 0, 0, &ValidationError{Field: "page", Message: "page must be 1 to 1000 and page_size 1 to 50"}
	}
	return page, size, nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// UnavailableError: payments aren't configured on this server.
type UnavailableError struct{ Message string }

func (e *UnavailableError) Error() string { return e.Message }

var (
	ErrNotConfigured  = &UnavailableError{Message: "payments aren't available yet"}
	ErrBadSignature   = &ValidationError{Field: "signature", Message: "invalid webhook signature"}
	ErrUnknownOrder   = &ValidationError{Field: "order_id", Message: "unknown payment order"}
	ErrAmountMismatch = &ValidationError{Field: "amount", Message: "paid amount doesn't match the order"}
)

func IsValidation(err error) bool  { _, ok := err.(*ValidationError); return ok }
func IsUnavailable(err error) bool { _, ok := err.(*UnavailableError); return ok }
