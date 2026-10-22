package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
)

func testUser(t *testing.T, pool *pgxpool.Pool, name string) string {
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO users (iam_user_id, email, name) VALUES ($1, $2, $3) RETURNING id::text`,
		"test-"+uuid.NewString(), name+"@example.com", name).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return id
}

func testCompanyInput(t *testing.T, slug string) *companies.Input {
	in := &companies.Input{
		Name: "Test Co " + slug, Slug: slug, Size: "11–50 people", Industries: []string{"Testing"},
		Website: "https://test.example", HQLocationID: "1277333",
		Products: []companies.Product{{Name: "Probe", About: "Finds bugs."}},
		Benefits: []companies.Benefit{{Title: "Snacks", Icon: "coffee"}},
	}
	if err := in.Normalize(time.Now()); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestCompanyLifecycleAndMemberRules(t *testing.T) {
	pool := testPool(t)
	repo := NewCompanyRepository(pool)
	ctx := context.Background()
	alice, bob, carol := testUser(t, pool, "alice"), testUser(t, pool, "bob"), testUser(t, pool, "carol")
	slug := "test-" + uuid.NewString()[:8]
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE slug LIKE $1`, slug+"%") })

	// Create: live immediately, creator is admin, showcase stored in order.
	c, err := repo.Create(ctx, testCompanyInput(t, slug), "IN", alice)
	if err != nil {
		t.Fatal(err)
	}
	if c.ViewerRole == nil || *c.ViewerRole != companies.RoleAdmin || len(c.Products) != 1 || len(c.Benefits) != 1 {
		t.Fatalf("created company: %+v", c)
	}
	if _, err := repo.Create(ctx, testCompanyInput(t, slug), "IN", bob); !companies.IsConflict(err) {
		t.Errorf("duplicate slug: want ConflictError, got %v", err)
	}
	public, err := repo.GetBySlug(ctx, slug, "")
	if err != nil || public.ViewerRole != nil {
		t.Fatalf("anonymous view: %+v, %v", public, err)
	}

	// Non-admins can't edit, see members or the dashboard.
	if _, err := repo.Update(ctx, slug, bob, testCompanyInput(t, slug), "IN"); err != companies.ErrNotAdmin {
		t.Errorf("non-admin update: want ErrNotAdmin, got %v", err)
	}
	if _, err := repo.Members(ctx, slug, bob, companies.StatusPending); err != companies.ErrNotAdmin {
		t.Errorf("non-admin members: want ErrNotAdmin, got %v", err)
	}

	// Bob claims to work there (from his profile); Alice approves.
	started := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.RequestMembership(ctx, c.ID, bob, "Recruiter", &started); err != nil {
		t.Fatal(err)
	}
	pending, err := repo.Members(ctx, slug, alice, companies.StatusPending)
	if err != nil || len(pending) != 1 || pending[0].Title != "Recruiter" || *pending[0].StartedOn != "2024-03-01" {
		t.Fatalf("pending: %+v, %v", pending, err)
	}
	if err := repo.DecideMember(ctx, slug, alice, bob, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideMember(ctx, slug, alice, bob, true); !companies.IsNotFound(err) {
		t.Errorf("deciding twice: want NotFoundError, got %v", err)
	}
	// A current employee is still not an admin.
	if _, err := repo.Dashboard(ctx, slug, bob); err != companies.ErrNotAdmin {
		t.Errorf("employee dashboard: want ErrNotAdmin, got %v", err)
	}

	// Last-admin rules (manual: the sole admin can't leave or be demoted).
	if err := repo.Leave(ctx, slug, alice); !companies.IsConflict(err) {
		t.Errorf("sole admin leaving: want ConflictError, got %v", err)
	}
	if err := repo.SetMemberRole(ctx, slug, alice, alice, companies.RoleHR); !companies.IsConflict(err) {
		t.Errorf("sole admin demoting self: want ConflictError, got %v", err)
	}
	if err := repo.SetMemberRole(ctx, slug, alice, bob, companies.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := repo.Leave(ctx, slug, alice); err != nil {
		t.Fatalf("leaving with another admin present: %v", err)
	}
	if err := repo.Offboard(ctx, slug, bob, bob); !companies.IsConflict(err) {
		t.Errorf("offboarding the last admin: want ConflictError, got %v", err)
	}
	if _, err := repo.Update(ctx, slug, alice, testCompanyInput(t, slug), "IN"); err != companies.ErrNotAdmin {
		t.Errorf("past admin update: want ErrNotAdmin, got %v", err)
	}

	// Carol's request is rejected; re-requesting puts her back to pending.
	if err := repo.RequestMembership(ctx, c.ID, carol, "Designer", nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideMember(ctx, slug, bob, carol, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequestMembership(ctx, c.ID, carol, "Senior designer", nil); err != nil {
		t.Fatal(err)
	}
	if pending, _ := repo.Members(ctx, slug, bob, companies.StatusPending); len(pending) != 1 || pending[0].Title != "Senior designer" {
		t.Errorf("re-request: %+v", pending)
	}

	d, err := repo.Dashboard(ctx, slug, bob)
	if err != nil || d.Admins != 1 || d.Employees != 1 || d.PendingRequests != 1 {
		t.Errorf("dashboard: %+v, %v", d, err)
	}

	// Slug change via update; search finds it by name and filters.
	in := testCompanyInput(t, slug+"-new")
	updated, err := repo.Update(ctx, slug, bob, in, "IN")
	if err != nil || updated.Slug != slug+"-new" {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	slug = updated.Slug
	res, err := repo.Search(ctx, &companies.SearchCriteria{Query: in.Name, Countries: []string{"IN"}, Industries: []string{"testing"},
		Sizes: []string{"11–50 people"}, Page: 1, PageSize: 5})
	if err != nil || res.Total != 1 || res.Companies[0].Slug != slug {
		t.Errorf("search: %+v, %v", res, err)
	}
	if res, _ := repo.Search(ctx, &companies.SearchCriteria{Query: "%", Countries: []string{}, Industries: []string{}, Sizes: []string{}, Page: 1, PageSize: 5}); res.Total != 0 {
		t.Errorf("'%%' must be literal, matched %d", res.Total)
	}

	// Delete: page gone, slug stays reserved.
	if err := repo.Delete(ctx, slug, bob); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetBySlug(ctx, slug, ""); err != companies.ErrNotFound {
		t.Errorf("deleted company: want ErrNotFound, got %v", err)
	}
	if taken, _ := repo.SlugTaken(ctx, slug); !taken {
		t.Error("deleted company's slug became reusable")
	}
	if err := repo.RequestMembership(ctx, c.ID, carol, "x", nil); err != companies.ErrNotFound {
		t.Errorf("request to deleted company: want ErrNotFound, got %v", err)
	}
}

// Two admins leaving at once: the company-row lock serialises them, so one
// succeeds and the other hits the last-admin rule. Never zero admins.
func TestConcurrentLeaveKeepsAnAdmin(t *testing.T) {
	pool := testPool(t)
	repo := NewCompanyRepository(pool)
	ctx := context.Background()
	alice, bob := testUser(t, pool, "alice"), testUser(t, pool, "bob")
	slug := "test-" + uuid.NewString()[:8]
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE slug = $1`, slug) })
	c, err := repo.Create(ctx, testCompanyInput(t, slug), "IN", alice)
	if err != nil {
		t.Fatal(err)
	}
	repo.RequestMembership(ctx, c.ID, bob, "", nil)
	repo.DecideMember(ctx, slug, alice, bob, true)
	if err := repo.SetMemberRole(ctx, slug, alice, bob, companies.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	for _, user := range []string{alice, bob} {
		go func() { errs <- repo.Leave(ctx, slug, user) }()
	}
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("want exactly one leave to succeed, got %v and %v", first, second)
	}
	var admins int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM company_members WHERE company_id = $1 AND role = 'admin' AND status = 'current'`, c.ID).Scan(&admins)
	if admins != 1 {
		t.Fatalf("%d admins left, want 1", admins)
	}
}
