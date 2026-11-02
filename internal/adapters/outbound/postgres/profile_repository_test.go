package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/profiles"
	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

func testProfile(slug string) *profiles.Profile {
	p := &profiles.Profile{
		Slug: slug, Name: "Asha Rao", Title: "Designer", About: "Hello", Email: "asha@example.com",
		SalaryCurrency: "INR", ExpectedSalary: 100, PreferredLocations: []string{"1277333"},
		Visible: true, ContactVisibility: "applied", ExpectedSalaryVisibility: "applied", CurrentSalaryVisibility: "applied",
		HourlyRateVisibility: "all", ResumeVisibility: "applied", CurrentSalaryPeriod: "year", ExpectedSalaryPeriod: "month",
		Availability: profiles.ServingNotice, LastWorkingDate: "2026-12-01", NoticeValue: 2, NoticeUnit: "months",
		Notice: "60 days", Travel: "Occasional travel",
		Experiences: []profiles.Experience{{Company: "Test Co " + slug, Title: "Designer", Type: "Full-time", Start: "2023-03", Current: true},
			{Company: "Unknown Ltd", Title: "Intern", Type: "Internship", Start: "2020-01", End: "2020-06"}},
		Details: profiles.Details{Languages: []profiles.Language{{Name: "English", Proficiency: "Fluent"}}},
	}
	return p
}

func TestProfileLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewProfileRepository(pool)
	taxonomy := NewTaxonomyRepository(pool)
	user, other := testUser(t, pool, "Asha"), testUser(t, pool, "Ravi")
	slug := "it-" + uuid.NewString()[:8]
	// The city the profile prefers must exist (a fresh DB has no GeoNames import).
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	company, err := NewCompanyRepository(pool).Create(ctx, testCompanyInput(t, slug), "IN", other)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, company.ID) })

	n := names(t, "Figma", "Figma app", "Research", "Product Designer")
	skills, _, err := taxonomy.Resolve(ctx, reference.Skill, n[:3], "")
	if err != nil {
		t.Fatal(err)
	}
	roles, _, err := taxonomy.Resolve(ctx, reference.Designation, n[3:], "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		norms := []string{n[0].Normalized, n[1].Normalized, n[2].Normalized, n[3].Normalized}
		pool.Exec(context.Background(), `DELETE FROM skills WHERE name_norm = ANY($1)`, norms)
		pool.Exec(context.Background(), `DELETE FROM designations WHERE name_norm = ANY($1)`, norms)
	})

	if _, err := repo.Get(ctx, user); !profiles.IsNotFound(err) {
		t.Fatalf("no profile yet: %v", err)
	}
	p := testProfile(slug)
	rows := []ports.ProfileSkill{{SkillID: skills[0].ID, Group: "Core"}, {SkillID: skills[1].ID, Group: "Core"}, {SkillID: skills[2].ID, Group: "Craft"}}
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Saved || got.AboutUpdated == nil || len(got.SkillGroups) != 2 || len(got.Skills) != 3 || got.TargetRoles[0] != roles[0].Name {
		t.Fatalf("round trip: %+v", got)
	}
	if got.HourlyRateVisibility != "all" || got.CurrentSalaryVisibility != "applied" || got.ExpectedSalaryPeriod != "month" ||
		!got.IsServingNotice || got.LastWorkingDate != "2026-12-01" || got.NoticeValue != 2 || got.NoticeUnit != "months" ||
		got.Travel != "Occasional travel" || got.AvailableFrom != "" {
		t.Fatalf("privacy/availability round trip: %+v", got)
	}
	// Expected salary is kept on a yearly INR baseline: 100 INR a month is 1200 a year.
	var inr float64
	if err := pool.QueryRow(ctx, `SELECT expected_salary_inr::float8 FROM profiles WHERE user_id = $1`, user).Scan(&inr); err != nil || inr != 1200 {
		t.Fatalf("expected_salary_inr = %v, %v", inr, err)
	}
	// The daily FX refresh re-normalises it the same way (same rates back in).
	ratesRepo := NewRatesRepository(pool)
	rates, err := ratesRepo.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE profiles SET expected_salary_inr = 0 WHERE user_id = $1`, user); err != nil {
		t.Fatal(err)
	}
	if err := ratesRepo.Replace(ctx, rates); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT expected_salary_inr::float8 FROM profiles WHERE user_id = $1`, user).Scan(&inr); err != nil || inr != 1200 {
		t.Fatalf("after FX refresh expected_salary_inr = %v, %v", inr, err)
	}
	// One-click chips: add appends to the last group (no-op when present),
	// remove drops it; no profile means ErrNoProfile.
	if err := repo.RemoveSkill(ctx, user, skills[2].ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := repo.AddSkill(ctx, user, skills[2].ID); err != nil {
			t.Fatal(err)
		}
	}
	if refs, err := repo.Skills(ctx, user); err != nil || len(refs) != 3 || refs[2].ID != skills[2].ID {
		t.Fatalf("skills after re-add: %v %v", refs, err)
	}
	if g, _ := repo.Get(ctx, user); len(g.SkillGroups) != 1 || g.SkillGroups[0].Skills[2] != skills[2].Name {
		t.Fatalf("re-added skill: %+v", g.SkillGroups)
	}
	if err := repo.AddSkill(ctx, user, -1); err != profiles.ErrUnknownSkill {
		t.Fatalf("unknown skill: %v", err)
	}
	if err := repo.AddSkill(ctx, other, skills[0].ID); err != profiles.ErrNoProfile {
		t.Fatalf("add without profile: %v", err)
	}
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if got.Experiences[0].CompanySlug != slug || got.Experiences[0].Verification != profiles.Unverified ||
		got.Experiences[1].CompanySlug != "" || got.Languages[0].Name != "English" {
		t.Fatalf("experiences/details: %+v %+v", got.Experiences, got.Details)
	}

	// Verification comes from the company's membership decision.
	x, err := repo.Experience(ctx, user, 0)
	if err != nil || x.CompanyID != company.ID || x.Title != "Designer" || x.Start != "2023-03" || x.Review != profiles.ReviewUnverified {
		t.Fatalf("experience 0: %+v %v", x, err)
	}
	if x, err := repo.Experience(ctx, user, 1); err != nil || x.CompanyID != "" {
		t.Fatalf("unregistered company: %+v %v", x, err)
	}
	if _, err := repo.Experience(ctx, user, 7); !profiles.IsNotFound(err) {
		t.Fatalf("missing experience: %v", err)
	}
	crepo := NewCompanyRepository(pool)
	started := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := crepo.RequestMembership(ctx, company.ID, user, x.Title, &started); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPending(ctx, user, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, user); got.Experiences[0].Verification != profiles.Pending {
		t.Fatalf("pending after request: %s", got.Experiences[0].Verification)
	}

	// Blue tick per entry: approval verifies it and saving it unchanged keeps
	// the tick; editing it sends it back to the company's queue while the
	// person stays a current member; a rejected re-review keeps the membership.
	verification := func() string {
		g, err := repo.Get(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		return g.Experiences[0].Verification
	}
	inQueue := func() bool {
		pending, err := crepo.Members(ctx, slug, other, "pending")
		if err != nil {
			t.Fatal(err)
		}
		return len(pending) == 1 && pending[0].UserID == user
	}
	if err := crepo.DecideMember(ctx, slug, other, user, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if v := verification(); v != profiles.Verified || inQueue() {
		t.Fatalf("after approval and an unchanged save: %s, queued %v", v, inQueue())
	}
	p.Experiences[0].Description = "Led the design system"
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if v := verification(); v != profiles.Pending || !inQueue() {
		t.Fatalf("edited verified entry: %s, queued %v", v, inQueue())
	}
	if d, err := crepo.Dashboard(ctx, slug, other); err != nil || d.PendingRequests != 1 {
		t.Fatalf("dashboard pending: %+v %v", d, err)
	}
	if err := crepo.DecideMember(ctx, slug, other, user, true); err != nil || verification() != profiles.Verified {
		t.Fatalf("re-approval: %v %s", err, verification())
	}
	p.Experiences[0].Title = "Design Lead"
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if err := crepo.DecideMember(ctx, slug, other, user, false); err != nil || verification() != profiles.Rejected {
		t.Fatalf("rejected re-review: %v %s", err, verification())
	}
	if current, err := crepo.Members(ctx, slug, other, "current"); err != nil || len(current) != 2 {
		t.Fatalf("membership kept after a rejected re-review: %d %v", len(current), err)
	}
	if err := crepo.DecideMember(ctx, slug, other, user, true); !companies.IsNotFound(err) {
		t.Fatalf("nothing left to decide: %v", err)
	}

	// Same about text keeps its timestamp.
	before := *got.AboutUpdated
	if err := repo.Save(ctx, user, p, rows, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, user); !got.AboutUpdated.Equal(before) || len(got.TargetRoles) != 0 {
		t.Fatalf("about timestamp moved or roles kept: %v %v", got.AboutUpdated, got.TargetRoles)
	}

	// Public reads respect visibility; slugs are unique; bad references are rejected.
	if _, err := repo.GetPublic(ctx, slug); err != nil {
		t.Fatalf("public: %v", err)
	}
	if err := repo.Save(ctx, other, testProfile(slug), nil, nil); err != profiles.ErrSlugTaken {
		t.Fatalf("taken slug: %v", err)
	}
	bad := testProfile(slug)
	bad.PreferredLocations = []string{"999999999999"}
	if err := repo.Save(ctx, user, bad, nil, nil); !profiles.IsValidation(err) {
		t.Fatalf("unknown location: %v", err)
	}
	bad = testProfile(slug)
	bad.SalaryCurrency = "ZZZ"
	if err := repo.Save(ctx, user, bad, nil, nil); !profiles.IsValidation(err) {
		t.Fatalf("unknown currency: %v", err)
	}
	p.Visible = false
	if err := repo.Save(ctx, user, p, rows, []int64{roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetPublic(ctx, slug); !profiles.IsNotFound(err) {
		t.Fatalf("private profile is public: %v", err)
	}

	// Merging "Figma app" into "Figma" leaves one row in its first position;
	// merging the designation keeps the target role.
	if err := taxonomy.Merge(ctx, reference.Skill, []int64{skills[1].ID}, skills[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, user); len(got.Skills) != 2 || got.Skills[0] != skills[0].Name || got.Skills[1] != skills[2].Name {
		t.Fatalf("after merge: %v", got.Skills)
	}
	if err := taxonomy.Delete(ctx, reference.Skill, skills[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := taxonomy.Delete(ctx, reference.Designation, roles[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, user); len(got.Skills) != 1 || len(got.TargetRoles) != 0 || len(got.SkillGroups) != 1 {
		t.Fatalf("after delete: %v %v", got.Skills, got.TargetRoles)
	}

	// Photos: the replaced key comes back so the file can be deleted.
	if old, err := repo.SetPhoto(ctx, user, "img/a.jpg"); err != nil || old != "" {
		t.Fatalf("first photo: %q %v", old, err)
	}
	if old, err := repo.SetPhoto(ctx, user, "img/b.jpg"); err != nil || old != "img/a.jpg" {
		t.Fatalf("second photo: %q %v", old, err)
	}
	if old, err := repo.SetPhoto(ctx, user, ""); err != nil || old != "img/b.jpg" {
		t.Fatalf("remove photo: %q %v", old, err)
	}
	if p, err := repo.Get(ctx, user); err != nil || p.Photo != nil {
		t.Fatalf("after removing, photo = %v, %v", p.Photo, err)
	}
	if old, err := repo.SetPhoto(ctx, user, ""); err != nil || old != "" {
		t.Fatalf("remove again: %q %v", old, err)
	}
	if _, err := repo.SetPhoto(ctx, other, "img/c.jpg"); !profiles.IsNotFound(err) {
		t.Fatalf("photo without profile: %v", err)
	}
}
