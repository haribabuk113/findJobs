package jobs

import (
	"strings"
	"testing"
)

func validPost() PostInput {
	return PostInput{
		Tier: TierPaid, PaymentMethod: PayOutOfPocket, Designation: "Go Developer", Skills: []string{"Go"},
		LocationIDs: []string{"1277333", "1277333"}, SalaryMax: 2_000_000, Currency: "INR", HideSalary: true,
		WorkModes: []string{"Remote"}, WorkTypes: []string{"Full-time"}, Relocation: "On their own",
		InterviewStages: []string{"Screen", "  ", "Panel"}, Application: "External",
		ExternalURL: "https://ats.example/job/1", Description: "Write **Go**.\n- APIs",
	}
}

func TestFreeTierRules(t *testing.T) {
	in := validPost()
	in.Tier, in.CompanySlug = TierFree, "acme"
	if err := in.Normalize(); err != nil {
		t.Fatal(err)
	}
	// Manual: free jobs are Easy Apply only, salary visible, never on a company page.
	if in.Application != "Easy apply" || in.ExternalURL != "" || in.HideSalary || in.CompanySlug != "" || in.PaymentMethod != "" {
		t.Fatalf("free tier rules not applied: %+v", in)
	}
}

func TestDerivedFields(t *testing.T) {
	in := validPost()
	if err := in.Normalize(); err != nil {
		t.Fatal(err)
	}
	if in.RoundCount() != 2 || len(in.LocationIDs) != 1 || in.NoticePeriods == nil {
		t.Fatalf("rounds %d (want 2 non-empty stages), locations %v", in.RoundCount(), in.LocationIDs)
	}
}

func TestPostRejects(t *testing.T) {
	for name, mutate := range map[string]func(*PostInput){
		"unknown tier":         func(in *PostInput) { in.Tier = "gold" },
		"paid without payment": func(in *PostInput) { in.PaymentMethod = "" },
		"credits without co.":  func(in *PostInput) { in.PaymentMethod = PayCompanyCredits },
		"short title":          func(in *PostInput) { in.Designation = "A" },
		"no skills":            func(in *PostInput) { in.Skills = nil },
		"16 skills":            func(in *PostInput) { in.Skills = strings.Split("a b c d e f g h i j k l m n o p", " ") },
		"negative experience":  func(in *PostInput) { in.MinExperience = -1 },
		"no locations":         func(in *PostInput) { in.LocationIDs = nil },
		"slug-style location":  func(in *PostInput) { in.LocationIDs = []string{"in-blr"} },
		"zero salary":          func(in *PostInput) { in.SalaryMax = 0 },
		"bad currency":         func(in *PostInput) { in.Currency = "rupees" },
		"no work mode":         func(in *PostInput) { in.WorkModes = nil },
		"unknown work type":    func(in *PostInput) { in.WorkTypes = []string{"Pirate"} },
		"unknown notice":       func(in *PostInput) { in.NoticePeriods = []string{"2 days"} },
		"unknown relocation":   func(in *PostInput) { in.Relocation = "maybe" },
		"21 stages":            func(in *PostInput) { in.InterviewStages = strings.Split(strings.Repeat("s,", 21), ",")[:21] },
		"http external url":    func(in *PostInput) { in.ExternalURL = "http://ats.example" },
		"javascript external":  func(in *PostInput) { in.ExternalURL = "javascript:alert(1)" },
		"unknown application":  func(in *PostInput) { in.Application = "Email" },
		"html description":     func(in *PostInput) { in.Description = "<script>alert(1)</script>" },
		"empty description":    func(in *PostInput) { in.Description = " " },
		"huge description":     func(in *PostInput) { in.Description = strings.Repeat("x", MaxDescription+1) },
		"bad company slug":     func(in *PostInput) { in.CompanySlug = "Acme Corp" },
	} {
		in := validPost()
		mutate(&in)
		if err := in.Normalize(); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}
