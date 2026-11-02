package account

import "testing"

func TestPending(t *testing.T) {
	docs := []Document{{"terms", "v2"}, {"privacy", "v1"}, {"cookies", "v1"}}
	latest := map[string]Consent{
		"terms":   {DocID: "terms", Version: "v1", Status: Accepted},     // accepted an older version
		"privacy": {DocID: "privacy", Version: "v1", Status: Accepted},   // current: nothing to do
		"cookies": {DocID: "cookies", Version: "v1", Status: Withdrawn}, // withdrawn: needs action again
	}
	got := Pending(docs, latest)
	if len(got) != 2 || got[0].ID != "terms" || got[0].AcceptedVersion != "v1" || got[1].ID != "cookies" || got[1].AcceptedVersion != "" {
		t.Fatalf("pending: %+v", got)
	}
	if got := Pending(docs, nil); len(got) != 3 {
		t.Fatalf("new user: %+v", got)
	}
	if got := Pending(nil, latest); got == nil || len(got) != 0 {
		t.Fatalf("no documents configured: %#v", got)
	}
}

func TestCheckDecision(t *testing.T) {
	docs := []Document{{"terms", "v2"}}
	for name, tc := range map[string]struct {
		doc, version, status string
		ok                   bool
	}{
		"accept current":   {"terms", "v2", Accepted, true},
		"withdraw current": {"terms", "v2", Withdrawn, true},
		"old version":      {"terms", "v1", Accepted, false},
		"unknown document": {"nda", "v1", Accepted, false},
		"bad status":       {"terms", "v2", "maybe", false},
	} {
		if err := CheckDecision(docs, tc.doc, tc.version, tc.status); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}
