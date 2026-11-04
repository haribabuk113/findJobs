package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"findJobs/internal/domain/applications"
	"findJobs/internal/domain/profiles"
)

// TestResumeAccess: who may read a résumé, by slug and by application.
func TestResumeAccess(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	repo := NewProfileRepository(pool)
	owner, recruiter, stranger := testUser(t, pool, "Résumé owner"), testUser(t, pool, "Résumé recruiter"), testUser(t, pool, "Stranger")
	slug := "cv-" + uuid.NewString()[:8]
	if err := repo.Save(ctx, owner, testProfile(slug), nil, nil); err != nil { // résumé: Applied Jobs Only
		t.Fatal(err)
	}
	if _, err := repo.Resume(ctx, slug, owner); !profiles.IsNotFound(err) {
		t.Fatalf("no résumé yet: %v", err)
	}
	if old, err := repo.SetResume(ctx, owner, "resume/a.pdf"); err != nil || old != "" {
		t.Fatalf("set: %q %v", old, err)
	}
	if p, _ := repo.Get(ctx, owner); p.Resume == nil || p.Resume.Format != "pdf" {
		t.Fatalf("profile résumé: %+v", p.Resume)
	}

	allowed := func(viewer string) bool {
		ref, err := repo.Resume(ctx, slug, viewer)
		if err != nil && !profiles.IsNotFound(err) {
			t.Fatal(err)
		}
		return err == nil && ref.Key == "resume/a.pdf" && ref.OwnerID == owner && ref.Slug == slug
	}
	if !allowed(owner) || allowed("") || allowed(stranger) || allowed(recruiter) {
		t.Fatal("Applied Jobs Only: only the owner before any application")
	}

	job := "cv-job-" + uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO jobs (id, title, tier, application, share_hash, recruiter_id)
		VALUES ($1::text, 'CV job', 'Premium', 'Easy apply', $1::text, $2)`, job, recruiter); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, job) })
	var appID string
	if err := pool.QueryRow(ctx, `INSERT INTO applications (user_id, job_id, apply_type, calculated_match_score_pct, dossier)
		VALUES ($1, $2, 'platform', 0, '{}') RETURNING id`, owner, job).Scan(&appID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM applications WHERE id = $1`, appID) })
	if !allowed(recruiter) || allowed(stranger) {
		t.Fatal("after applying: the job's recruiter only")
	}
	list, err := NewApplicationRepository(pool).Applicants(ctx, job, recruiter, applications.ApplicantFilter{})
	if err != nil || len(list) != 1 || !list[0].HasResume {
		t.Fatalf("applicant list: %+v %v", list, err)
	}
	if ref, err := repo.ApplicantResume(ctx, appID, recruiter); err != nil || ref.Key != "resume/a.pdf" {
		t.Fatalf("applicant résumé: %+v %v", ref, err)
	}
	for _, c := range []struct{ app, viewer string }{{appID, stranger}, {appID, ""}, {"not-a-uuid", recruiter}, {uuid.NewString(), recruiter}} {
		if _, err := repo.ApplicantResume(ctx, c.app, c.viewer); !profiles.IsNotFound(err) {
			t.Fatalf("applicant résumé %v: %v", c, err)
		}
	}
	// A private profile still reaches its recruiters, like the dossier.
	if _, err := pool.Exec(ctx, `UPDATE profiles SET is_public = FALSE WHERE user_id = $1`, owner); err != nil {
		t.Fatal(err)
	}
	if !allowed(recruiter) {
		t.Fatal("private profile: recruiter refused")
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET is_public = TRUE, resume_visibility = 'all' WHERE user_id = $1`, owner); err != nil {
		t.Fatal(err)
	}
	if !allowed("") || !allowed(stranger) {
		t.Fatal("Visible to All: refused")
	}

	if old, err := repo.SetResume(ctx, owner, ""); err != nil || old != "resume/a.pdf" {
		t.Fatalf("remove: %q %v", old, err)
	}
	if p, _ := repo.Get(ctx, owner); p.Resume != nil || allowed(owner) {
		t.Fatal("removed résumé still there")
	}
	if _, err := repo.SetResume(ctx, stranger, "resume/b.pdf"); !profiles.IsNotFound(err) {
		t.Fatalf("résumé without profile: %v", err)
	}
}
