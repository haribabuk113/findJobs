package settings

import (
	"context"
	"testing"

	"findJobs/internal/domain/settings"
)

type stubRepo struct{ s settings.Settings }

func (r *stubRepo) Get(context.Context) (*settings.Settings, error) { s := r.s; return &s, nil }
func (r *stubRepo) Update(_ context.Context, s *settings.Settings, _ string) (*settings.Settings, error) {
	r.s = *s
	return r.Get(context.Background())
}

func TestWithJobPostFee(t *testing.T) {
	repo := WithJobPostFee(&stubRepo{s: settings.Settings{JobPostFeePaise: 0, ApplyFeePaise: 500}}, 200)
	if s, _ := repo.Get(context.Background()); s.JobPostFeePaise != 200 || s.ApplyFeePaise != 500 {
		t.Fatalf("Get = %+v", s)
	}
	if s, _ := repo.Update(context.Background(), &settings.Settings{JobPostFeePaise: 9999, ApplyFeePaise: 700}, "admin"); s.JobPostFeePaise != 200 || s.ApplyFeePaise != 700 {
		t.Fatalf("Update = %+v", s)
	}
}
