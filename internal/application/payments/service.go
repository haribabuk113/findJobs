// Package payments is the wallet and top-up use case.
package payments

import (
	"context"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/payments"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// PaymentService reads wallets and tops them up through the gateway.
type PaymentService interface {
	Wallet(ctx context.Context, owner payments.Owner, page, pageSize int) (*payments.Wallet, error)
	// Ledger reads the balance and every ledger entry, newest first (CSV export).
	Ledger(ctx context.Context, owner payments.Owner) (*payments.Wallet, error)
	// Topup creates a gateway order for the owner's wallet. The wallet is
	// credited later, when the gateway's webhook reports the order paid.
	Topup(ctx context.Context, owner payments.Owner, amountPaise int64) (*payments.Checkout, error)
	// HandleWebhook verifies a gateway webhook and applies it (idempotent).
	HandleWebhook(ctx context.Context, body []byte, signature string) error
	// Configured reports whether a gateway is set up (out-of-pocket posts need one).
	Configured() bool
	// JobPostCheckout creates a gateway order for a pending job's fee; the
	// webhook publishes the job once it's paid.
	JobPostCheckout(ctx context.Context, userID, jobID string, amountPaise int64) (*payments.Checkout, error)
	// Reconcile compares the gateway's captured payments with our payments and
	// ledger: it applies paid orders whose webhook never arrived and logs every
	// mismatch. It errors only when the gateway or the database can't be read.
	Reconcile(ctx context.Context) error
}

type service struct {
	repo    ports.WalletRepository
	gateway ports.PaymentGateway // nil when no gateway is configured
}

// NewPaymentService creates the payment service; gateway may be nil, which
// leaves wallets readable and top-ups unavailable.
func NewPaymentService(repo ports.WalletRepository, gateway ports.PaymentGateway) PaymentService {
	return &service{repo: repo, gateway: gateway}
}

func (s *service) Wallet(ctx context.Context, owner payments.Owner, page, pageSize int) (*payments.Wallet, error) {
	page, pageSize, err := payments.NormalizePage(page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.repo.Wallet(ctx, owner, page, pageSize)
}

func (s *service) Ledger(ctx context.Context, owner payments.Owner) (*payments.Wallet, error) {
	return s.repo.Wallet(ctx, owner, 1, 0)
}

func (s *service) Topup(ctx context.Context, owner payments.Owner, amountPaise int64) (*payments.Checkout, error) {
	if s.gateway == nil {
		return nil, payments.ErrNotConfigured
	}
	if err := payments.ValidateTopup(amountPaise); err != nil {
		return nil, err
	}
	walletID, err := s.repo.OpenWallet(ctx, owner)
	if err != nil {
		return nil, err
	}
	return s.checkout(ctx, payments.Payment{WalletID: walletID, UserID: owner.UserID, AmountPaise: amountPaise})
}

func (s *service) Configured() bool { return s.gateway != nil }

func (s *service) JobPostCheckout(ctx context.Context, userID, jobID string, amountPaise int64) (*payments.Checkout, error) {
	if s.gateway == nil {
		return nil, payments.ErrNotConfigured
	}
	return s.checkout(ctx, payments.Payment{JobID: jobID, UserID: userID, AmountPaise: amountPaise})
}

// checkout creates the gateway order for p and records it. An order nobody
// pays costs nothing, so a retried request needs no Idempotency-Key: it just
// leaves an unpaid order behind.
func (s *service) checkout(ctx context.Context, p payments.Payment) (*payments.Checkout, error) {
	p.ID = uuid.NewString()
	orderID, err := s.gateway.CreateOrder(ctx, p.AmountPaise, p.ID)
	if err != nil {
		return nil, err
	}
	p.OrderID = orderID
	if err := s.repo.CreatePayment(ctx, p); err != nil {
		return nil, err
	}
	return &payments.Checkout{Gateway: "razorpay", KeyID: s.gateway.KeyID(), OrderID: orderID, AmountPaise: p.AmountPaise, Currency: "INR"}, nil
}

func (s *service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	if s.gateway == nil {
		return payments.ErrNotConfigured
	}
	paid, err := s.gateway.ParseWebhook(body, signature)
	if err != nil || paid == nil {
		return err
	}
	_, err = s.repo.CompletePayment(ctx, *paid)
	// Neither case heals on a retry, so acknowledge the webhook and leave it
	// to the logs; the daily Reconcile reports them again. An unknown order can be
	// another app's on the same gateway account.
	if err == payments.ErrUnknownOrder || err == payments.ErrAmountMismatch {
		log := logger.Get()
		log.Error().Err(err).Str("order_id", paid.OrderID).Str("payment_id", paid.PaymentID).
			Int64("amount_paise", paid.AmountPaise).Str("currency", paid.Currency).Msg("payment webhook not applied")
		return nil
	}
	return err
}

// Reconciliation looks back reconcileWindow, so a week of missed daily runs
// still heals, and skips the last webhookGrace, so a webhook still in flight
// isn't reported as missed.
const (
	reconcileWindow = 7 * 24 * time.Hour
	webhookGrace    = time.Hour
)

func (s *service) Reconcile(ctx context.Context) error {
	if s.gateway == nil {
		return nil
	}
	r, err := s.reconcile(ctx, time.Now())
	if err != nil {
		return err
	}
	log := logger.Get()
	ev := log.Info()
	if r.Mismatched > 0 || r.Unledgered > 0 || r.Applied > 0 {
		ev = log.Error()
	} else if r.Unknown > 0 {
		ev = log.Warn()
	}
	ev.Int("checked", r.Checked).Int("applied", r.Applied).Int("unknown", r.Unknown).
		Int("mismatched", r.Mismatched).Int("unledgered", r.Unledgered).Msg("payments reconciled")
	return nil
}

func (s *service) reconcile(ctx context.Context, now time.Time) (payments.Reconciliation, error) {
	var r payments.Reconciliation
	from, to := now.Add(-reconcileWindow), now.Add(-webhookGrace)
	captured, err := s.gateway.CapturedPayments(ctx, from, to)
	if err != nil {
		return r, err
	}
	log := logger.Get()
	for _, paid := range captured {
		r.Checked++
		applied, err := s.repo.CompletePayment(ctx, paid)
		switch {
		case err == payments.ErrUnknownOrder:
			r.Unknown++
			log.Warn().Str("order_id", paid.OrderID).Str("payment_id", paid.PaymentID).
				Int64("amount_paise", paid.AmountPaise).Msg("reconcile: captured payment for an order we don't have")
		case err == payments.ErrAmountMismatch:
			r.Mismatched++
			log.Error().Str("order_id", paid.OrderID).Str("payment_id", paid.PaymentID).Int64("amount_paise", paid.AmountPaise).
				Str("currency", paid.Currency).Msg("reconcile: captured amount doesn't match the order; nothing credited")
		case err != nil:
			return r, err
		case applied:
			r.Applied++
			log.Error().Str("order_id", paid.OrderID).Str("payment_id", paid.PaymentID).
				Int64("amount_paise", paid.AmountPaise).Msg("reconcile: paid order had no webhook; applied now")
		}
	}
	orders, err := s.repo.UnledgeredTopups(ctx, from)
	if err != nil {
		return r, err
	}
	for _, order := range orders {
		r.Unledgered++
		log.Error().Str("order_id", order).Msg("reconcile: paid top-up has no matching ledger row")
	}
	return r, nil
}
