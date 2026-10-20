package companies

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func validInput() Input {
	return Input{
		Name: "  Acme Corp ", Slug: "acme-corp", Size: "51–200 people",
		Industries: []string{"Fintech", "fintech", " B2B Software "},
		About:      "We build **tools**.\n- one\n- two\n\na < b is fine",
		Website:    "https://acme.example", HQLocationID: "1277333",
		Social:            map[string]string{"linkedin": "https://linkedin.com/company/acme", "x": ""},
		BranchLocationIDs: []string{"1275339", "1275339"},
		Products:          []Product{{Name: "Ledger", About: "Books in minutes."}},
		Benefits:          []Benefit{{Title: "Gym membership", Icon: "heart"}},
	}
}

func TestNormalizeCleansValidInput(t *testing.T) {
	in := validInput()
	if err := in.Normalize(time.Now()); err != nil {
		t.Fatal(err)
	}
	if in.Name != "Acme Corp" || len(in.Industries) != 2 || in.Industries[1] != "B2B Software" {
		t.Errorf("not cleaned: name %q industries %q", in.Name, in.Industries)
	}
	if len(in.BranchLocationIDs) != 1 || len(in.Social) != 1 {
		t.Errorf("duplicates/empties kept: branches %v social %v", in.BranchLocationIDs, in.Social)
	}
}

func TestNormalizeRejects(t *testing.T) {
	year := 2999
	for name, mutate := range map[string]func(*Input){
		"short name":          func(in *Input) { in.Name = "A" },
		"html in name":        func(in *Input) { in.Name = "<b>Acme</b>" },
		"script in about":     func(in *Input) { in.About = "hi <script>alert(1)</script>" },
		"comment in about":    func(in *Input) { in.About = "<!-- x -->" },
		"control char":        func(in *Input) { in.Tagline = "tag\x07line" },
		"long tagline":        func(in *Input) { in.Tagline = strings.Repeat("a", 151) },
		"unknown size":        func(in *Input) { in.Size = "huge" },
		"no industries":       func(in *Input) { in.Industries = nil },
		"six industries":      func(in *Input) { in.Industries = []string{"a", "b", "c", "d", "e", "f"} },
		"long industry":       func(in *Input) { in.Industries = []string{strings.Repeat("x", 26)} },
		"no website":          func(in *Input) { in.Website = "" },
		"javascript website":  func(in *Input) { in.Website = "javascript:alert(1)" },
		"website credentials": func(in *Input) { in.Website = "https://user:pw@acme.example" },
		"unknown social":      func(in *Input) { in.Social = map[string]string{"myspace": "https://x.example"} },
		"bad social url":      func(in *Input) { in.Social = map[string]string{"x": "ftp://x"} },
		"future founding":     func(in *Input) { in.FoundedYear = &year },
		"no headquarters":     func(in *Input) { in.HQLocationID = "" },
		"bad branch id":       func(in *Input) { in.BranchLocationIDs = []string{"in-blr"} },
		"product no about":    func(in *Input) { in.Products = []Product{{Name: "Ledger"}} },
		"product bad url":     func(in *Input) { in.Products = []Product{{Name: "Ledger", About: "x", URL: "data:text/html,x"}} },
		"benefit bad icon":    func(in *Input) { in.Benefits = []Benefit{{Title: "Perk", Icon: "skull"}} },
		"reserved slug":       func(in *Input) { in.Slug = "admin" },
		"uppercase slug":      func(in *Input) { in.Slug = "Acme" },
		"double hyphen slug":  func(in *Input) { in.Slug = "acme--corp" },
		"edge hyphen slug":    func(in *Input) { in.Slug = "-acme" },
	} {
		in := validInput()
		mutate(&in)
		if err := in.Normalize(time.Now()); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

func TestSearchCriteriaNormalize(t *testing.T) {
	c := SearchCriteria{Industries: []string{" Fintech "}}
	if err := c.Normalize(); err != nil || c.Page != 1 || c.PageSize != 20 || c.Industries[0] != "fintech" {
		t.Fatalf("defaults: %+v, %v", c, err)
	}
	for name, bad := range map[string]SearchCriteria{
		"country name": {Countries: []string{"India"}},
		"unknown size": {Sizes: []string{"tiny"}},
		"page size":    {PageSize: 51},
		"long query":   {Query: strings.Repeat("q", 101)},
	} {
		if err := bad.Normalize(); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCheckImage(t *testing.T) {
	good := pngBytes(t, 4, 4)
	if ext, err := CheckImage(good, "image/png", MaxLogoBytes); err != nil || ext != "png" {
		t.Fatalf("valid png: %q, %v", ext, err)
	}
	for name, tc := range map[string]struct {
		data     []byte
		declared string
		max      int
	}{
		"declared type mismatch": {good, "image/jpeg", MaxLogoBytes},
		"svg":                    {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "image/svg+xml", MaxLogoBytes},
		"html posing as png":     {[]byte("<html><script>alert(1)</script></html>"), "image/png", MaxLogoBytes},
		"too large":              {good, "image/png", 10},
		"empty":                  {nil, "image/png", MaxLogoBytes},
		"truncated png":          {good[:20], "image/png", MaxLogoBytes},
	} {
		if _, err := CheckImage(tc.data, tc.declared, tc.max); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}
