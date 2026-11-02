// Package account is the account and legal use case: consents, the consent
// gate and account deletion.
package account

import (
	"context"
	"time"

	"findJobs/internal/domain/account"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// AccountService is the inbound port for consents and account deletion.
type AccountService interface {
	// Required lists the documents the user still has to accept (empty: none).
	Required(ctx context.Context, userID string) ([]account.Required, error)
	// Decide records the user's acceptance or withdrawal of a document's
	// current version and returns what is still required.
	Decide(ctx context.Context, userID, docID, version, status string) ([]account.Required, error)
	// Gate returns *account.ConsentRequiredError while documents are pending.
	Gate(ctx context.Context, userID string) error
	// Delete deletes or anonymises the account, ends every session and
	// revokes them at IAM. The IAM identity itself is never deleted.
	Delete(ctx context.Context, userID string) error
}

type service struct {
	repo     ports.AccountRepository
	sessions ports.SessionStore
	iam      ports.IAMClient
	store    ports.ObjectStore
	docs     []account.Document
}

// NewAccountService creates the account service. docs are the current legal
// documents (LEGAL_DOCUMENTS); with none, nothing is required.
func NewAccountService(repo ports.AccountRepository, sessions ports.SessionStore, iam ports.IAMClient,
	store ports.ObjectStore, docs []account.Document) AccountService {
	return &service{repo: repo, sessions: sessions, iam: iam, store: store, docs: docs}
}

func (s *service) Required(ctx context.Context, userID string) ([]account.Required, error) {
	if len(s.docs) == 0 {
		return []account.Required{}, nil
	}
	latest, err := s.repo.LatestConsents(ctx, userID)
	if err != nil {
		return nil, err
	}
	return account.Pending(s.docs, latest), nil
}

func (s *service) Decide(ctx context.Context, userID, docID, version, status string) ([]account.Required, error) {
	if err := account.CheckDecision(s.docs, docID, version, status); err != nil {
		return nil, err
	}
	if err := s.repo.RecordConsent(ctx, userID, docID, version, status); err != nil {
		return nil, err
	}
	l := logger.Get()
	l.Info().Str("user_id", userID).Str("doc_id", docID).Str("version", version).Str("status", status).Msg("consent recorded")
	return s.Required(ctx, userID)
}

// ponytail: one indexed query per gated request; cache it with the session if it shows up in latency.
func (s *service) Gate(ctx context.Context, userID string) error {
	pending, err := s.Required(ctx, userID)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return &account.ConsentRequiredError{Documents: pending}
	}
	return nil
}

// revokeTimeout bounds the IAM logouts after a deletion (5 s HTTP timeout each).
const revokeTimeout = 30 * time.Second

func (s *service) Delete(ctx context.Context, userID string) error {
	photo, err := s.repo.DeleteAccount(ctx, userID)
	if err != nil {
		return err
	}
	l := logger.Get()
	l.Info().Str("user_id", userID).Msg("account deleted")

	// The account is gone; finish the rest even if the client disconnects.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), revokeTimeout)
	defer cancel()
	if photo != "" {
		if err := s.store.Delete(ctx, photo); err != nil {
			l.Warn().Err(err).Str("key", photo).Msg("could not delete a deleted account's photo")
		}
	}
	sessions, err := s.sessions.DeleteUserSessions(ctx, userID)
	if err != nil {
		return err // the account is anonymised; its sessions see no data, and the purge job removes them
	}
	for _, sess := range sessions {
		if sess.RefreshToken == "" {
			continue
		}
		if err := s.iam.Logout(ctx, sess.RefreshToken); err != nil {
			l.Warn().Err(err).Str("user_id", userID).Msg("could not revoke a deleted account's IAM session")
		}
	}
	return nil
}
