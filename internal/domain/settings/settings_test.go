package settings

import "testing"

func TestValidate(t *testing.T) {
	valid := Settings{DailyFreeApplications: 15, ApplyFeePaise: 500, MatchSortFeePaise: 2000, FXRefreshTime: "05:00"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}

	for name, mutate := range map[string]func(*Settings){
		"negative free applications": func(s *Settings) { s.DailyFreeApplications = -1 },
		"too many free applications": func(s *Settings) { s.DailyFreeApplications = 1001 },
		"negative fee":               func(s *Settings) { s.ApplyFeePaise = -1 },
		"fee over cap":               func(s *Settings) { s.JobPostFeePaise = MaxFeePaise + 1 },
		"time 24:00":                 func(s *Settings) { s.FXRefreshTime = "24:00" },
		"time without padding":       func(s *Settings) { s.FXRefreshTime = "5:00" },
		"time injection":             func(s *Settings) { s.FXRefreshTime = "05:00'; DROP TABLE settings;--" },
	} {
		s := valid
		mutate(&s)
		if err := s.Validate(); !IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
}

func TestPublicOmitsOperationalValues(t *testing.T) {
	s := Settings{DailyFreeApplications: 15, ApplyFeePaise: 500, MatchSortFeePaise: 2000, JobPostFeePaise: 9900, FXRefreshTime: "05:00"}
	if p := s.Public(); p != (PublicSettings{15, 500, 2000, 9900}) {
		t.Fatalf("public settings: %+v", p)
	}
}
