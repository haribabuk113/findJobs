package ports

import (
	"context"

	"findJobs/internal/domain/account"
)

// AccountRepository stores consents and deletes accounts.
type AccountRepository interface {
	// LatestConsents returns the user's latest decision per document id.
	LatestConsents(ctx context.Context, userID string) (map[string]account.Consent, error)
	// RecordConsent appends a decision; earlier ones are never changed.
	RecordConsent(ctx context.Context, userID, docID, version, status string) error
	// DeleteAccount deletes or anonymises the user's product data in one
	// transaction, keeping consents, account records and money records (the
	// ledger is append-only). It returns the profile's file keys (photo,
	// résumé) so the files can be removed, or *account.SoleAdminError.
	DeleteAccount(ctx context.Context, userID string) (fileKeys []string, err error)
}
