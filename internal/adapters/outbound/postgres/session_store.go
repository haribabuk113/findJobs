package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/auth"
	"findJobs/internal/pkg/tokencrypt"
	"findJobs/internal/ports"
)

// SessionStore keeps login sessions in Postgres, the durable source of truth.
// Each commit is durable, so a crash can never roll back a refresh-token
// rotation (IAM revokes the whole session when it sees a used refresh token).
// IAM tokens are stored encrypted, bound to their session id and column.
type SessionStore struct {
	pool *pgxpool.Pool
	keys *tokencrypt.Keyring
}

// NewSessionStore creates the PostgreSQL adapter for sessions.
func NewSessionStore(pool *pgxpool.Pool, keys *tokencrypt.Keyring) ports.SessionStore {
	return &SessionStore{pool: pool, keys: keys}
}

// The AAD binds each ciphertext to its row and column: a token copied into
// another session, or swapped between columns, fails to decrypt.
func refreshAAD(id string) []byte { return []byte(id + "|refresh") }
func accessAAD(id string) []byte  { return []byte(id + "|access") }

func (s *SessionStore) CreateSession(ctx context.Context, sess *auth.Session) (string, error) {
	id := uuid.NewString() // generated here: the AAD needs it before the insert
	kid, refresh := s.keys.Seal([]byte(sess.RefreshToken), refreshAAD(id))
	_, access := s.keys.Seal([]byte(sess.AccessToken), accessAAD(id))
	const query = `
		INSERT INTO sessions (id, user_id, refresh_token_enc, access_token_enc, key_id, access_expires_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, query, id, sess.UserID, refresh, access, kid, sess.AccessExpiresAt, sess.ExpiresAt)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Lookup is the lock-free read used while the access token is valid.
func (s *SessionStore) Lookup(ctx context.Context, sessionID string) (*auth.SessionState, error) {
	if uuid.Validate(sessionID) != nil {
		return nil, auth.ErrUnauthorized
	}
	const query = `
		SELECT s.id::text, s.user_id::text, u.iam_user_id, s.access_expires_at, s.expires_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()`
	var st auth.SessionState
	err := s.pool.QueryRow(ctx, query, sessionID).
		Scan(&st.ID, &st.UserID, &st.IAMUserID, &st.AccessExpiresAt, &st.ExpiresAt, &st.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// ponytail: holds a row lock across fn (which calls IAM /refresh, bounded by
// the IAM client timeout) so concurrent requests on one session never present
// the same refresh token twice — IAM treats that as theft and revokes.
// Only taken for a refresh or a token read; Lookup serves the rest.
func (s *SessionStore) LockSession(ctx context.Context, sessionID string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error) {
	if uuid.Validate(sessionID) != nil {
		return nil, auth.ErrUnauthorized
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const query = `
		SELECT id::text, user_id::text, refresh_token_enc, access_token_enc, key_id, access_expires_at, expires_at
		FROM sessions WHERE id = $1 AND expires_at > NOW() FOR UPDATE`
	var (
		sess            auth.Session
		refresh, access []byte
		kid             string
	)
	err = tx.QueryRow(ctx, query, sessionID).
		Scan(&sess.ID, &sess.UserID, &refresh, &access, &kid, &sess.AccessExpiresAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	// Undecryptable (retired key, tampered row): treat as signed out, so the
	// caller deletes the session and the user signs in again.
	refreshPlain, err := s.keys.Open(kid, refresh, refreshAAD(sess.ID))
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	accessPlain, err := s.keys.Open(kid, access, accessAAD(sess.ID))
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	sess.RefreshToken, sess.AccessToken = string(refreshPlain), string(accessPlain)

	updated, err := fn(&sess)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return &sess, tx.Commit(ctx)
	}
	newKid, newRefresh := s.keys.Seal([]byte(updated.RefreshToken), refreshAAD(sess.ID))
	_, newAccess := s.keys.Seal([]byte(updated.AccessToken), accessAAD(sess.ID))
	const update = `
		UPDATE sessions SET refresh_token_enc = $2, access_token_enc = $3, key_id = $4,
			access_expires_at = $5, expires_at = $6, last_seen_at = NOW()
		WHERE id = $1`
	if _, err := tx.Exec(ctx, update, sess.ID, newRefresh, newAccess, newKid, updated.AccessExpiresAt, updated.ExpiresAt); err != nil {
		return nil, err
	}
	return updated, tx.Commit(ctx)
}

func (s *SessionStore) Touch(ctx context.Context, sessionID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = NOW() WHERE id = $1`, sessionID)
	return err
}

func (s *SessionStore) DeleteSession(ctx context.Context, sessionID string) (string, error) {
	if uuid.Validate(sessionID) != nil {
		return "", nil
	}
	var (
		refresh []byte
		kid     string
	)
	err := s.pool.QueryRow(ctx, `DELETE FROM sessions WHERE id = $1 RETURNING refresh_token_enc, key_id`, sessionID).Scan(&refresh, &kid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	plain, err := s.keys.Open(kid, refresh, refreshAAD(sessionID))
	if err != nil {
		return "", nil // deleted locally; nothing usable to revoke at IAM
	}
	return string(plain), nil
}
