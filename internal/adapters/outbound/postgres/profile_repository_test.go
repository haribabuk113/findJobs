package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

func testProfile(slug string) *profiles.Profile {
	p := &profiles.Profile{
		Slug: slug, Name: "Asha Rao", Title: "Designer", About: "Hello", Email: "asha@example.com",
		SalaryCurrency: "INR", ExpectedSalary: 100, PreferredLocations: []string{"1277333"},
		Visible: true, ContactPrivate: true, SalaryPrivate: true,
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
	companyID, title, start, err := repo.Experience(ctx, user, 0)
	if err != nil || companyID != company.ID || title != "Designer" || start != "2023-03" {
		t.Fatalf("experience 0: %q %q %q %v", companyID, title, start, err)
	}
	if id, _, _, err := repo.Experience(ctx, user, 1); err != nil || id != "" {
		t.Fatalf("unregistered company: %q %v", id, err)
	}
	if _, _, _, err := repo.Experience(ctx, user, 7); !profiles.IsNotFound(err) {
		t.Fatalf("missing experience: %v", err)
	}
	started := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := NewCompanyRepository(pool).RequestMembership(ctx, company.ID, user, title, &started); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, user); got.Experiences[0].Verification != profiles.Pending {
		t.Fatalf("pending after request: %s", got.Experiences[0].Verification)
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
	if _, err := repo.SetPhoto(ctx, other, "img/c.jpg"); !profiles.IsNotFound(err) {
		t.Fatalf("photo without profile: %v", err)
	}
}
