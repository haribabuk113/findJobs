package profiles

import (
	"context"
	"testing"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

type photoRepo struct {
	ports.ProfileRepository
	key string
}

func (r *photoRepo) SetPhoto(_ context.Context, _, key string) (string, error) {
	old := r.key
	r.key = key
	return old, nil
}

func (r *photoRepo) Get(context.Context, string) (*profiles.Profile, error) {
	return &profiles.Profile{}, nil
}

type photoStore struct {
	ports.ObjectStore
	deleted []string
}

func (s *photoStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func TestDeletePhoto(t *testing.T) {
	repo, store := &photoRepo{key: "img/a.jpg"}, &photoStore{}
	s := NewProfileService(repo, nil, nil, store)
	if _, err := s.DeletePhoto(context.Background(), "u1"); err != nil || repo.key != "" {
		t.Fatalf("delete: key %q, %v", repo.key, err)
	}
	// Again without a photo: no error, and no file deleted.
	if _, err := s.DeletePhoto(context.Background(), "u1"); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "img/a.jpg" {
		t.Fatalf("deleted files = %v", store.deleted)
	}
}
