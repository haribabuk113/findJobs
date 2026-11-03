package profiles

import (
	"strings"
	"testing"
)

func validProfile() *Profile {
	return &Profile{
		Slug: "asha-rao", Name: "Asha Rao", Title: "Product Designer", SalaryCurrency: "inr",
		WorkTypes: []string{"Full-time", "Full-time"}, PreferredLocations: []string{"1277333"},
		TargetRoles: []string{"Product Designer", " Product Designer "},
		SkillGroups: []SkillGroup{{Name: "Core", Skills: []string{"Figma", "figma", " Research "}}, {Name: "Empty", Skills: []string{" "}}},
		Experiences: []Experience{{Company: "Forma", Type: "Full-time", Start: "2023-03", End: "2020-01", Current: true,
			Verification: Verified, Location: "injected", Roles: []Role{
				{Title: "Designer", Start: "2023-03", End: "2024-05", Achievements: []string{" Shipped v2 ", ""}},
				{Title: "Senior Designer", Start: "2024-06", Promoted: true}}}},
		Details: Details{
			SocialLinks:    []SocialLink{{Platform: "LinkedIn", URL: "https://linkedin.com/in/asha"}},
			Authorizations: []Authorization{{Country: "India", Type: "Citizen", From: "2001-01-01", Until: "2000-01-01", Indefinite: true}},
			Projects:       []Project{{Title: " Calm ", Description: "Line one\nLine two", Tags: []string{"UX", " "}}},
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
	if e := p.Experiences[0]; e.End != "" || e.Verification != "" || e.Location != "" {
		t.Fatalf("current job keeps no end month; verification and location are read-only: %+v", e)
	}
	if r := p.Experiences[0].Roles; len(r[0].Achievements) != 1 || r[0].Achievements[0] != "Shipped v2" || r[1].End != "" {
		t.Fatalf("roles: achievements cleaned, the ongoing role keeps no end: %+v", r)
	}
	if got := p.Experiences[0].LatestTitle(); got != "Senior Designer" {
		t.Fatalf("latest title %q", got)
	}
	if a := p.Authorizations[0]; a.Until != "" {
		t.Fatalf("indefinite authorization keeps no expiry: %+v", a)
	}
	if pr := p.Projects[0]; pr.Title != "Calm" || pr.URL != "" || len(pr.Tags) != 1 {
		t.Fatalf("project not cleaned (link optional, blank tags dropped): %+v", pr)
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
		"from without date":   func(p *Profile) { p.Availability = AvailableFromDate },
		"availability":        func(p *Profile) { p.Availability = "Maybe" },
		"notice unit":         func(p *Profile) { p.NoticeUnit, p.NoticeValue = "years", 1 },
		"notice too long":     func(p *Profile) { p.NoticeUnit, p.NoticeValue = "months", 13 },
		"negative notice":     func(p *Profile) { p.NoticeUnit, p.NoticeValue = "days", -1 },
		"salary period":       func(p *Profile) { p.ExpectedSalaryPeriod = "fortnight" },
		"travel":              func(p *Profile) { p.Travel = "Always" },
		"hidden contact":      func(p *Profile) { p.ContactVisibility = VisibleHidden },
		"visibility value":    func(p *Profile) { p.CurrentSalaryVisibility = "friends" },
		"currency":            func(p *Profile) { p.SalaryCurrency = "RUPEES" },
		"negative salary":     func(p *Profile) { p.ExpectedSalary = -1 },
		"work type":           func(p *Profile) { p.WorkTypes = []string{"Nap"} },
		"relocation":          func(p *Profile) { p.Relocation = "Mars" },
		"location id":         func(p *Profile) { p.PreferredLocations = []string{"bengaluru"} },
		"unnamed skill group": func(p *Profile) { p.SkillGroups[0].Name = "" },
		"experience type":     func(p *Profile) { p.Experiences[0].Type = "Nap" },
		"experience start":    func(p *Profile) { p.Experiences[0].Start = "2023" },
		"ends before start":   func(p *Profile) { p.Experiences[0].Current, p.Experiences[0].End = false, "2022-01" },
		"no roles":            func(p *Profile) { p.Experiences[0].Roles = nil },
		"untitled role":       func(p *Profile) { p.Experiences[0].Roles[0].Title = "" },
		"role before job":     func(p *Profile) { p.Experiences[0].Roles[0].Start = "2023-01" },
		"role after job ends": func(p *Profile) { p.Experiences[0].Current, p.Experiences[0].End = false, "2024-03" },
		"role ends early":     func(p *Profile) { p.Experiences[0].Roles[0].End = "2023-01" },
		"html achievement":    func(p *Profile) { p.Experiences[0].Roles[0].Achievements = []string{"<b>x</b>"} },
		"arrangement":         func(p *Profile) { p.Experiences[0].WorkArrangement = "Moon" },
		"experience location": func(p *Profile) { p.Experiences[0].LocationID = "bengaluru" },
		"current location":    func(p *Profile) { p.CurrentLocationID = "x1" },
		"current arrangement": func(p *Profile) { p.CurrentWorkArrangement = "Remote or hybrid" },
		"current work type":   func(p *Profile) { p.CurrentWorkType = "Nap" },
		"long grade":          func(p *Profile) { p.Education = []Education{{Degree: "BSc", Grade: strings.Repeat("A", 51)}} },
		"html in education":   func(p *Profile) { p.Education = []Education{{Degree: "BSc", Description: "<i>x</i>"}} },
		"social link scheme":  func(p *Profile) { p.SocialLinks[0].URL = "javascript:alert(1)" },
		"authorization dates": func(p *Profile) { p.Authorizations[0].Indefinite = false },
		"empty education":     func(p *Profile) { p.Education = []Education{{Field: "Design"}} },
		"gender":              func(p *Profile) { p.Gender = "Robot" },
		"untitled project":    func(p *Profile) { p.Projects[0].Title = " " },
		"project link scheme": func(p *Profile) { p.Projects[0].URL = "javascript:alert(1)" },
		"html in project":     func(p *Profile) { p.Projects[0].Description = "<b>hi</b>" },
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
		ContactVisibility: VisibleApplied, CurrentSalaryVisibility: VisibleHidden, ExpectedSalaryVisibility: VisibleApplied,
		HourlyRateVisibility: VisibleApplied}
	pub := p.Public()
	if pub.Email != "" || pub.Phone != "" || pub.CurrentSalary != 0 || pub.ExpectedSalary != 0 || pub.HourlyRate != 0 || pub.LastWorkingDate != "" {
		t.Fatalf("private data leaked: %+v", pub)
	}
	p.ContactVisibility, p.ExpectedSalaryVisibility, p.CurrentSalaryVisibility = VisibleAll, VisibleAll, VisibleApplied
	if pub = p.Public(); pub.Email == "" || pub.ExpectedSalary != 2 || pub.CurrentSalary != 0 {
		t.Fatalf("visible-to-all hidden or applied-only shown: %+v", pub)
	}
}

func TestPrivacyDefaultsAndAvailability(t *testing.T) {
	p := validProfile()
	p.IsServingNotice, p.LastWorkingDate, p.AvailableFrom = true, "2026-12-01", "2026-12-15"
	p.NoticeUnit, p.NoticeValue = "weeks", 3
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	if p.CurrentSalaryVisibility != VisibleHidden || p.ContactVisibility != VisibleApplied || p.ExpectedSalaryPeriod != "year" {
		t.Fatalf("defaults: %+v", p)
	}
	// The old flag still means serving notice; a date the availability doesn't use is dropped.
	if p.Availability != ServingNotice || !p.IsServingNotice || p.AvailableFrom != "" || p.Notice != "30 days" {
		t.Fatalf("availability: %q %v %q %q", p.Availability, p.IsServingNotice, p.AvailableFrom, p.Notice)
	}
	p.Availability, p.IsServingNotice, p.AvailableFrom, p.NoticeUnit = AvailableFromDate, false, "2026-12-15", ""
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	if p.LastWorkingDate != "" || p.IsServingNotice || p.NoticeValue != 0 || p.Notice != "30 days" {
		t.Fatalf("available from: %+v", p)
	}
	for days, want := range map[int]string{0: "Immediate", 10: "15 days", 31: "45 days", 90: "90 days", 91: "More than 90 days"} {
		if got := NoticeBucket(days); got != want {
			t.Errorf("NoticeBucket(%d) = %q, want %q", days, got, want)
		}
	}
}

// Older clients send flat entries and a language proficiency; both still save.
func TestLegacyInput(t *testing.T) {
	p := validProfile()
	p.Experiences = []Experience{{Company: "Forma", Type: "Full-time", Start: "2023-03", End: "2024-01",
		LegacyTitle: "Designer", LegacyPromoted: true, LegacyDescription: "Did things"}}
	p.Languages = []Language{{Name: "Tamil", LegacyProficiency: "Native"}, {Name: "Hindi", Speak: true}}
	p.Education = []Education{{Institution: "IIT", Grade: " 8.9 CGPA ", Achievements: []string{"Gold medal", " "}}}
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	e := p.Experiences[0]
	want := Role{Title: "Designer", Start: "2023-03", End: "2024-01", Promoted: true, Responsibilities: "Did things"}
	if len(e.Roles) != 1 || e.Roles[0].Title != want.Title || e.Roles[0].End != want.End || !e.Roles[0].Promoted ||
		e.Roles[0].Responsibilities != want.Responsibilities || e.LegacyTitle != "" || e.LegacyDescription != "" {
		t.Fatalf("flat entry not turned into one role: %+v", e)
	}
	if l := p.Languages; !(l[0].Read && l[0].Write && l[0].Speak) || l[0].LegacyProficiency != "" || l[1].Read || !l[1].Speak {
		t.Fatalf("languages: %+v", l)
	}
	if ed := p.Education[0]; ed.Grade != "8.9 CGPA" || len(ed.Achievements) != 1 {
		t.Fatalf("education: %+v", ed)
	}
}

func TestCarryReviews(t *testing.T) {
	e := func(title string) Experience {
		return Experience{Type: "Full-time", Start: "2023-01", Roles: []Role{{Title: title, Start: "2023-01"}}}
	}
	old := []LinkedExperience{
		{CompanyID: "a", Entry: e("Designer"), Review: ReviewVerified},
		{CompanyID: "b", Entry: e("Intern"), Review: ReviewRejected},
		{CompanyID: "c", Entry: e("Lead"), Review: ReviewVerified},
		{CompanyID: "d", Entry: e("Writer"), Review: ReviewPending},
	}
	next := []LinkedExperience{
		{CompanyID: "c", Entry: e("Lead")},            // moved, unchanged: keeps its tick
		{CompanyID: "b", Entry: e("Intern (edited)")}, // edited in place, was rejected: stays rejected
		{CompanyID: "x", Entry: e("Lead")},            // new company at an old position
		{CompanyID: "d", Entry: e("Writer, edited")},  // edited while pending: still pending
		{CompanyID: "a", Entry: e("Senior Designer")}, // new position: not an in-place edit
		{CompanyID: "", Entry: e("Freelance")},        // no registered company
	}
	want := []string{ReviewVerified, ReviewRejected, ReviewUnverified, ReviewPending, ReviewUnverified, ReviewUnverified}
	got := CarryReviews(old, next)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %s, want %s", i, got[i], want[i])
		}
	}
	// A verified entry edited in place goes back for review, including an
	// edit inside a role (an added achievement) or a new role.
	edited := e("Designer")
	edited.Roles[0].Achievements = []string{"Shipped v2"}
	promoted := e("Designer")
	promoted.Roles = append(promoted.Roles, Role{Title: "Lead", Start: "2024-01", Promoted: true})
	for _, entry := range []Experience{e("Senior Designer"), edited, promoted} {
		if got := CarryReviews(old[:1], []LinkedExperience{{CompanyID: "a", Entry: entry}}); got[0] != ReviewPending {
			t.Errorf("edited verified entry %+v: %s", entry.Roles, got[0])
		}
	}
	// Equal roles (nil vs empty achievements) are unchanged.
	same := e("Designer")
	same.Roles[0].Achievements = []string{}
	if got := CarryReviews(old[:1], []LinkedExperience{{CompanyID: "a", Entry: same}}); got[0] != ReviewVerified {
		t.Errorf("unchanged entry lost its tick: %s", got[0])
	}
}

func TestSlugs(t *testing.T) {
	for name, want := range map[string]string{"Asha Rao": "asha-rao", "  ": "member", "Zoë O'Neil": "zo-o-neil"} {
		if got := SlugBase(name); got != want {
			t.Errorf("SlugBase(%q) = %q, want %q", name, got, want)
		}
	}
	for _, tc := range []struct {
		base  string
		taken []string
		want  string
	}{
		{"asha-rao", nil, "asha-rao"},
		{"asha-rao", []string{"asha-rao"}, "asha-rao-2"},
		{"asha-rao", []string{"asha-rao", "asha-rao-2", "asha-rao-3"}, "asha-rao-4"},
		{"al", nil, "al-2"},       // too short on its own
		{"admin", nil, "admin-2"}, // reserved
		{"member", []string{"member"}, "member-2"},
	} {
		taken := map[string]bool{}
		for _, s := range tc.taken {
			taken[s] = true
		}
		got := FreeSlug(tc.base, taken)
		if got != tc.want {
			t.Errorf("FreeSlug(%q, %v) = %q, want %q", tc.base, tc.taken, got, tc.want)
		}
		p := validProfile()
		p.Slug = got
		if err := p.Normalize(); err != nil {
			t.Errorf("generated slug %q invalid: %v", got, err)
		}
	}
}
