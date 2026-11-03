package profiles

import (
	"context"
	"testing"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

// slugRepo: no profile yet, and other users hold some handles.
type slugRepo struct {
	ports.ProfileRepository
	taken map[string]bool
}

func (r *slugRepo) Get(context.Context, string) (*profiles.Profile, error) {
	return nil, profiles.ErrNotFound
}
func (r *slugRepo) Account(context.Context, string) (string, string, error) {
	return "Asha Rao", "asha@example.com", nil
}
func (r *slugRepo) TakenSlugs(context.Context, string, string) (map[string]bool, error) {
	return r.taken, nil
}

func TestStarterSlugAndAvailability(t *testing.T) {
	ctx := context.Background()
	s := NewProfileService(&slugRepo{taken: map[string]bool{"asha-rao": true, "asha-rao-2": true}}, nil, nil, nil)
	if p, err := s.Mine(ctx, "u1"); err != nil || p.Slug != "asha-rao-3" || p.Saved {
		t.Fatalf("starter: %+v %v", p, err)
	}
	for slug, want := range map[string]bool{"asha-rao": false, "asha-rao-9": true} {
		if got, err := s.SlugAvailable(ctx, "u1", slug); err != nil || got != want {
			t.Errorf("SlugAvailable(%q) = %v, %v; want %v", slug, got, err, want)
		}
	}
	for _, bad := range []string{"ab", "admin", "Asha"} {
		if _, err := s.SlugAvailable(ctx, "u1", bad); !profiles.IsValidation(err) {
			t.Errorf("SlugAvailable(%q): want validation error, got %v", bad, err)
		}
	}
}
