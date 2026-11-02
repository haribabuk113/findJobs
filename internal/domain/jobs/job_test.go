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

func TestUnlockHash(t *testing.T) {
	base := SearchCriteria{Query: "go", WorkModes: []string{"Remote", "Hybrid"}, Skills: []int64{3, 1}}
	paged := base
	paged.Sort, paged.Page, paged.PageSize, paged.UnlockID = SortMatch, 4, 50, "x"
	reordered := base
	reordered.WorkModes, reordered.Skills = []string{"Hybrid", "Remote"}, []int64{1, 3}
	if base.UnlockHash() != paged.UnlockHash() || base.UnlockHash() != reordered.UnlockHash() {
		t.Fatal("sort, paging and list order must not change the unlock")
	}
	changed := base
	changed.Query = "golang"
	if base.UnlockHash() == changed.UnlockHash() {
		t.Fatal("a filter change must need a new unlock")
	}
	if base.WorkModes[0] != "Remote" || base.Skills[0] != 3 {
		t.Fatalf("hashing reordered the caller's lists: %+v", base)
	}
}
