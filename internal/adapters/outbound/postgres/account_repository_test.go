package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/account"
	"findJobs/internal/domain/auth"
)

func TestAccountConsentsAndDeletion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewAccountRepository(pool)
	user, other := testUser(t, pool, "Leaver"), testUser(t, pool, "Stayer")
	var ecosystemID string
	if err := pool.QueryRow(ctx, `SELECT iam_user_id FROM users WHERE id = $1`, user).Scan(&ecosystemID); err != nil {
		t.Fatal(err)
	}
	// Consents and account records are append-only; only this cleanup may
	// remove the test's rows (replica mode skips the triggers).
	t.Cleanup(func() {
		pool.Exec(context.Background(), `SET session_replication_role = replica;
			DELETE FROM consents WHERE user_id IN ('`+user+`', '`+other+`');
			DELETE FROM account_events WHERE user_id IN ('`+user+`', '`+other+`');
			SET session_replication_role = DEFAULT;`)
	})

	// Consents: the latest decision per document counts.
	for _, c := range [][3]string{{"terms", "v1", "accepted"}, {"terms", "v2", "accepted"}, {"cookies", "v1", "accepted"}, {"cookies", "v1", "withdrawn"}} {
		if err := repo.RecordConsent(ctx, user, c[0], c[1], c[2]); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := repo.LatestConsents(ctx, user)
	if err != nil || len(latest) != 2 || latest["terms"].Version != "v2" || latest["cookies"].Status != account.Withdrawn {
		t.Fatalf("latest: %+v %v", latest, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE consents SET status = 'accepted' WHERE user_id = $1`, user); err == nil {
		t.Fatal("consents must be append-only")
	}

	// A company the user is the only admin of blocks deletion.
	slug := "ac-" + uuid.NewString()[:8]
	company, err := NewCompanyRepository(pool).Create(ctx, testCompanyInput(t, slug), "IN", user)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, company.ID) })
	var sole *account.SoleAdminError
	if _, err := repo.DeleteAccount(ctx, user); !errors.As(err, &sole) || len(sole.Companies) != 1 {
		t.Fatalf("sole admin: %v", err)
	}

	// With a second admin it goes through.
	if _, err := pool.Exec(ctx, `INSERT INTO company_members (company_id, user_id, role, status) VALUES ($1, $2, 'admin', 'current')`,
		company.ID, other); err != nil {
		t.Fatal(err)
	}
	p := testProfile("del-" + uuid.NewString()[:8])
	p.Experiences = nil
	if _, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if err := NewProfileRepository(pool).Save(ctx, user, p, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProfileRepository(pool).SetPhoto(ctx, user, "img/leaver.jpg"); err != nil {
		t.Fatal(err)
	}
	ownJob, companyJob := "it-own-"+uuid.NewString()[:8], "it-co-"+uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (id, title, tier, application, share_hash, recruiter_id, company_id) VALUES
			($1::text, 'Own job', 'Premium', 'Easy apply', $1::text, $3::uuid, NULL),
			($2::text, 'Company job', 'Premium', 'Easy apply', $2::text, $3::uuid, $4)`, ownJob, companyJob, user, company.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id IN ($1, $2)`, ownJob, companyJob) })
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1)`, user); err != nil {
		t.Fatal(err) // a wallet (like the ledger and payments) keeps the users row alive
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM wallets WHERE user_id = $1`, user) })

	if _, err := NewProfileRepository(pool).SetResume(ctx, user, "resume/leaver.pdf"); err != nil {
		t.Fatal(err)
	}
	files, err := repo.DeleteAccount(ctx, user)
	if err != nil || len(files) != 2 || files[0] != "img/leaver.jpg" || files[1] != "resume/leaver.pdf" {
		t.Fatalf("delete: %q %v", files, err)
	}
	var name, email, iamID, status string
	if err := pool.QueryRow(ctx, `SELECT name, email, iam_user_id, status FROM users WHERE id = $1`, user).
		Scan(&name, &email, &iamID, &status); err != nil {
		t.Fatal(err)
	}
	if name != "Deleted user" || email != "" || iamID != "deleted:"+user || status != "deleted" {
		t.Fatalf("not anonymised: %q %q %q %q", name, email, iamID, status)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM profiles WHERE user_id = $1)
		+ (SELECT COUNT(*) FROM company_members WHERE user_id = $1)`, user).Scan(&left); err != nil || left != 0 {
		t.Fatalf("product data left: %d %v", left, err)
	}
	var ownStatus, coStatus string
	var ownRecruiter, coRecruiter *string
	pool.QueryRow(ctx, `SELECT status, recruiter_id::text FROM jobs WHERE id = $1`, ownJob).Scan(&ownStatus, &ownRecruiter)
	pool.QueryRow(ctx, `SELECT status, recruiter_id::text FROM jobs WHERE id = $1`, companyJob).Scan(&coStatus, &coRecruiter)
	if ownStatus != "closed" || coStatus != "active" || ownRecruiter != nil || coRecruiter != nil {
		t.Fatalf("jobs: own %s %v, company %s %v", ownStatus, ownRecruiter, coStatus, coRecruiter)
	}
	// Consents survive; the deletion is recorded against the IAM identity.
	if latest, _ := repo.LatestConsents(ctx, user); len(latest) != 2 {
		t.Fatalf("consents after deletion: %+v", latest)
	}
	var deleted int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM account_events WHERE user_id = $1 AND ecosystem_id = $2 AND kind = 'deleted'`,
		user, ecosystemID).Scan(&deleted)
	if deleted != 1 {
		t.Fatalf("deletion records: %d", deleted)
	}
	// Deleted accounts can't record consents, or be deleted twice.
	if err := repo.RecordConsent(ctx, user, "terms", "v2", "accepted"); err != auth.ErrUserNotFound {
		t.Fatalf("consent after deletion: %v", err)
	}
	if _, err := repo.DeleteAccount(ctx, user); err != auth.ErrUserNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestAccountCreatedRecord(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewAuthRepository(pool)
	iamID := "test-created-" + uuid.NewString()
	in := &auth.User{IAMUserID: iamID, Name: "New", Email: "new@example.com", Status: "active"}
	u, err := repo.UpsertUser(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = '`+u.ID+`';
			SET session_replication_role = replica;
			DELETE FROM account_events WHERE user_id = '`+u.ID+`';
			SET session_replication_role = DEFAULT;`)
	})
	if _, err := repo.UpsertUser(ctx, in); err != nil { // a later sign-in updates, adds no record
		t.Fatal(err)
	}
	var n int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM account_events WHERE user_id = $1 AND kind = 'created' AND ecosystem_id = $2`, u.ID, iamID).Scan(&n)
	if n != 1 {
		t.Fatalf("created records: %d", n)
	}
}

func TestDeleteUserSessions(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	a := newSession(t, s, userID, time.Now().Add(time.Hour))
	b := newSession(t, s, userID, time.Now().Add(time.Hour))
	ended, err := s.DeleteUserSessions(ctx, userID)
	if err != nil || len(ended) != 2 || ended[0].RefreshToken != "r0" || ended[1].RefreshToken != "r0" {
		t.Fatalf("ended: %+v %v", ended, err)
	}
	for _, id := range []string{a, b} {
		if _, err := s.Lookup(ctx, id); !auth.IsUnauthorized(err) {
			t.Fatalf("session %s still valid: %v", id, err)
		}
	}
}
