package jobs

import "testing"

func TestNormalize(t *testing.T) {
	c := SearchCriteria{PageSize: 999}
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	if c.Sort != "newest" || c.PostedWithin != 30 || c.Page != 1 || c.PageSize != MaxPageSize {
		t.Fatalf("defaults not applied: %+v", c)
	}

	five, two := 5, 2
	for name, bad := range map[string]SearchCriteria{
		"unknown work mode":  {WorkModes: []string{"Moon"}},
		"unknown notice":     {NoticePeriods: []string{"2 days"}},
		"salary min > max":   {MinSalaryINR: 10, MaxSalaryINR: 5},
		"negative salary":    {MinSalaryINR: -1},
		"experience min>max": {MinExperience: &five, MaxExperience: &two},
		"posted within 2":    {PostedWithin: 2},
		"sort injection":     {Sort: "j.id; drop table jobs"},
		"query too long":     {Query: string(make([]byte, 201))},
	} {
		if err := bad.Normalize(); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}
