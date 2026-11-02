// Package payments is the wallet and top-up use case.
package payments

import (
	"context"

	"github.com/google/uuid"

	"findJobs/internal/domain/payments"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// PaymentService reads wallets and tops them up through the gateway.
type PaymentService interface {
	Wallet(ctx context.Context, owner payments.Owner, page, pageSize int) (*payments.Wallet, error)
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
	err = s.repo.CompletePayment(ctx, *paid)
	// Neither case heals on a retry, so acknowledge the webhook and leave it
	// to the logs (and reconciliation, once built). An unknown order can be
	// another app's on the same gateway account.
	if err == payments.ErrUnknownOrder || err == payments.ErrAmountMismatch {
		log := logger.Get()
		log.Error().Err(err).Str("order_id", paid.OrderID).Str("payment_id", paid.PaymentID).
			Int64("amount_paise", paid.AmountPaise).Str("currency", paid.Currency).Msg("payment webhook not applied")
		return nil
	}
	return err
}
