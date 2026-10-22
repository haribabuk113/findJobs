package reference

import (
	"strings"
	"testing"
)

func TestCleanName(t *testing.T) {
	for raw, want := range map[string]CleanedName{
		"  Svelte   JS ":   {"Svelte JS", "svelte js"},
		"ＧｏＬａｎｇ":           {"GoLang", "golang"}, // NFKC folds full-width letters
		"Node.js":          {"Node.js", "node.js"},
		"C++":              {"C++", "c++"},
		"Gestión de Datos": {"Gestión de Datos", "gestión de datos"},
		"数据分析":             {"数据分析", "数据分析"},
	} {
		got, err := CleanName(raw)
		if err != nil || got != want {
			t.Errorf("CleanName(%q) = %+v, %v; want %+v", raw, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", MaxNameLength+1), "<script>", "a>b", "x{{y}}", "back\\slash", "nul\x00"} {
		if _, err := CleanName(bad); !IsValidation(err) {
			t.Errorf("CleanName(%q): want ValidationError, got %v", bad, err)
		}
	}
}

func TestRatesValidate(t *testing.T) {
	if err := (Rates{"INR": 1, "USD": 95.05}).Validate(); err != nil {
		t.Fatal(err)
	}
	for name, r := range map[string]Rates{
		"empty":       {},
		"no INR":      {"USD": 95},
		"INR not 1":   {"INR": 2},
		"zero rate":   {"INR": 1, "USD": 0},
		"bad code":    {"INR": 1, "usd": 95},
		"absurd rate": {"INR": 1, "XXX": 1e12},
	} {
		if r.Validate() == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
