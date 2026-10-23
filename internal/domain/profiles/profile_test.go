package profiles

import "testing"

func validProfile() *Profile {
	return &Profile{
		Slug: "asha-rao", Name: "Asha Rao", Title: "Product Designer", SalaryCurrency: "inr",
		WorkTypes: []string{"Full-time", "Full-time"}, PreferredLocations: []string{"1277333"},
		TargetRoles: []string{"Product Designer", " Product Designer "},
		SkillGroups: []SkillGroup{{Name: "Core", Skills: []string{"Figma", "figma", " Research "}}, {Name: "Empty", Skills: []string{" "}}},
		Experiences: []Experience{{Company: "Forma", Title: "Designer", Type: "Full-time", Start: "2023-03", End: "2020-01", Current: true, Verification: Verified}},
		Details: Details{
			SocialLinks:    []SocialLink{{Platform: "LinkedIn", URL: "https://linkedin.com/in/asha"}},
			Authorizations: []Authorization{{Country: "India", Type: "Citizen", From: "2001-01-01", Until: "2000-01-01", Indefinite: true}},
		},
		Saved: true, Skills: []string{"injected"},
	}
}

func TestNormalizeCleansAndClearsReadOnlyFields(t *testing.T) {
	p := validProfile()
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	if p.Saved || p.Skills != nil || p.SalaryCurrency != "INR" {
		t.Fatalf("read-only fields or currency not normalized: %+v", p)
	}
	if len(p.WorkTypes) != 1 || len(p.TargetRoles) != 1 {
		t.Fatalf("duplicates kept: %v %v", p.WorkTypes, p.TargetRoles)
	}
	if len(p.SkillGroups) != 1 || len(p.SkillGroups[0].Skills) != 2 || p.SkillGroups[0].Skills[1] != "Research" {
		t.Fatalf("skills: %+v", p.SkillGroups) // case-insensitive dedupe, empty group dropped
	}
	if e := p.Experiences[0]; e.End != "" || e.Verification != "" {
		t.Fatalf("current job keeps no end month and verification is read-only: %+v", e)
	}
	if a := p.Authorizations[0]; a.Until != "" {
		t.Fatalf("indefinite authorization keeps no expiry: %+v", a)
	}
}

func TestNormalizeRejects(t *testing.T) {
	cases := map[string]func(*Profile){
		"short slug":          func(p *Profile) { p.Slug = "ab" },
		"slug double hyphen":  func(p *Profile) { p.Slug = "asha--rao" },
		"reserved slug":       func(p *Profile) { p.Slug = "admin" },
		"short name":          func(p *Profile) { p.Name = "A" },
		"no title":            func(p *Profile) { p.Title = " " },
		"html in about":       func(p *Profile) { p.About = "<script>x</script>" },
		"bad email":           func(p *Profile) { p.Email = "not-an-email" },
		"bad phone":           func(p *Profile) { p.Phone = "call me" },
		"experience years":    func(p *Profile) { p.ExperienceYears = 100 },
		"notice":              func(p *Profile) { p.Notice = "2 days" },
		"serving without day": func(p *Profile) { p.IsServingNotice = true },
		"currency":            func(p *Profile) { p.SalaryCurrency = "RUPEES" },
		"negative salary":     func(p *Profile) { p.ExpectedSalary = -1 },
		"work type":           func(p *Profile) { p.WorkTypes = []string{"Nap"} },
		"relocation":          func(p *Profile) { p.Relocation = "Mars" },
		"location id":         func(p *Profile) { p.PreferredLocations = []string{"bengaluru"} },
		"unnamed skill group": func(p *Profile) { p.SkillGroups[0].Name = "" },
		"experience type":     func(p *Profile) { p.Experiences[0].Type = "Nap" },
		"experience start":    func(p *Profile) { p.Experiences[0].Start = "2023" },
		"ends before start":   func(p *Profile) { p.Experiences[0].Current, p.Experiences[0].End = false, "2022-01" },
		"social link scheme":  func(p *Profile) { p.SocialLinks[0].URL = "javascript:alert(1)" },
		"authorization dates": func(p *Profile) { p.Authorizations[0].Indefinite = false },
		"empty education":     func(p *Profile) { p.Education = []Education{{Field: "Design"}} },
		"gender":              func(p *Profile) { p.Gender = "Robot" },
	}
	for name, mutate := range cases {
		p := validProfile()
		mutate(p)
		if err := p.Normalize(); !IsValidation(err) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}
}

func TestPublicHidesPrivateFields(t *testing.T) {
	p := Profile{Email: "a@b.co", Phone: "1", CurrentSalary: 1, ExpectedSalary: 2, HourlyRate: 3, LastWorkingDate: "2026-01-01",
		ContactPrivate: true, CurrentSalaryPrivate: true, SalaryPrivate: true, HourlyPrivate: true}
	pub := p.Public()
	if pub.Email != "" || pub.Phone != "" || pub.CurrentSalary != 0 || pub.ExpectedSalary != 0 || pub.HourlyRate != 0 || pub.LastWorkingDate != "" {
		t.Fatalf("private data leaked: %+v", pub)
	}
	p.ContactPrivate, p.SalaryPrivate = false, false
	if pub = p.Public(); pub.Email == "" || pub.ExpectedSalary != 2 {
		t.Fatalf("public data hidden: %+v", pub)
	}
}

func TestStarterSlug(t *testing.T) {
	for name, want := range map[string]string{"Asha Rao": "asha-rao-ab12", "  ": "member-ab12", "Zoë O'Neil": "zo-o-neil-ab12"} {
		if got := StarterSlug(name, "ab12"); got != want {
			t.Errorf("StarterSlug(%q) = %q, want %q", name, got, want)
		}
		p := validProfile()
		p.Slug = StarterSlug(name, "ab12")
		if err := p.Normalize(); err != nil {
			t.Errorf("starter slug %q invalid: %v", p.Slug, err)
		}
	}
}
