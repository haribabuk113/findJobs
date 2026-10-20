package geonames

import (
	"strings"
	"testing"
)

func row(fields ...string) string { return strings.Join(fields, "\t") }

// pad builds a row with n fields, setting the given positions.
func pad(n int, set map[int]string) string {
	f := make([]string, n)
	for i, v := range set {
		f[i] = v
	}
	return row(f...)
}

func TestParse(t *testing.T) {
	countries := strings.Join([]string{
		"# ISO\tISO3\t...",
		pad(19, map[int]string{0: "IN", 4: "India", 16: "1269750"}),
		pad(19, map[int]string{0: "SG", 4: "Singapore", 16: "1880251"}),
		pad(19, map[int]string{0: "xx", 4: "Bad code", 16: "1"}), // skipped
	}, "\n")
	admin1 := strings.Join([]string{
		row("IN.19", "Karnataka", "Karnataka", "1267701"),
		row("ZZ.01", "Orphan", "Orphan", "999"), // unknown country: skipped
	}, "\n")
	cities := strings.Join([]string{
		pad(19, map[int]string{0: "1277333", 1: "Bengaluru", 4: "12.97194", 5: "77.59369", 8: "IN", 10: "19"}),
		pad(19, map[int]string{0: "1880252", 1: "Singapore", 4: "1.28967", 5: "103.85007", 8: "SG", 10: "00"}), // no region: under country
		pad(19, map[int]string{0: "1", 1: "<script>", 4: "0", 5: "0", 8: "IN", 10: "19"}),                      // markup: skipped
		pad(19, map[int]string{0: "2", 1: "Nowhere", 4: "95", 5: "0", 8: "IN", 10: "19"}),                      // bad lat: skipped
	}, "\n")

	got, err := Parse(strings.NewReader(countries), strings.NewReader(admin1), strings.NewReader(cities))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ level, parent string }{
		"1269750": {"country", ""},
		"1880251": {"country", ""},
		"1267701": {"region", "1269750"},
		"1277333": {"city", "1267701"},
		"1880252": {"city", "1880251"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d locations, want %d: %+v", len(got), len(want), got)
	}
	pos := map[string]int{}
	for i, l := range got {
		pos[l.ID] = i
		w, ok := want[l.ID]
		parent := ""
		if l.ParentID != nil {
			parent = *l.ParentID
		}
		if !ok || l.Level != w.level || parent != w.parent {
			t.Errorf("unexpected %+v (parent %q)", l, parent)
		}
	}
	for id, w := range want {
		if w.parent != "" && pos[w.parent] > pos[id] {
			t.Errorf("%s comes before its parent %s", id, w.parent)
		}
	}
}

func TestParseRejectsMalformedFiles(t *testing.T) {
	good := pad(19, map[int]string{0: "IN", 4: "India", 16: "1269750"})
	if _, err := Parse(strings.NewReader("IN\tshort"), strings.NewReader(""), strings.NewReader("")); err == nil {
		t.Error("short country rows: want error")
	}
	if _, err := Parse(strings.NewReader(good), strings.NewReader(""), strings.NewReader("")); err == nil {
		t.Error("empty admin1 file: want error")
	}
}
