package profiles

import (
	"context"
	"errors"
	"strings"
	"testing"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

type photoRepo struct {
	ports.ProfileRepository
	key, resume string
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
	deleted, put []string
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

func (r *photoRepo) SetResume(_ context.Context, _, key string) (string, error) {
	old := r.resume
	r.resume = key
	return old, nil
}

func (s *photoStore) Put(_ context.Context, key, _ string, _ []byte) error {
	s.put = append(s.put, key)
	return nil
}

func TestSetResume(t *testing.T) {
	repo, store := &photoRepo{}, &photoStore{}
	s := NewProfileService(repo, nil, nil, store)
	ctx := context.Background()
	var invalid *profiles.ValidationError
	if _, err := s.SetResume(ctx, "u1", profiles.PDFType, []byte("<html>")); !errors.As(err, &invalid) || len(store.put) != 0 {
		t.Fatalf("fake PDF: %v, stored %v", err, store.put)
	}
	for range 2 {
		if _, err := s.SetResume(ctx, "u1", profiles.PDFType, []byte("%PDF-1.4")); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.put) != 2 || !strings.HasPrefix(store.put[0], "resume/") || !strings.HasSuffix(store.put[0], ".pdf") ||
		repo.resume != store.put[1] || len(store.deleted) != 1 || store.deleted[0] != store.put[0] {
		t.Fatalf("put %v, deleted %v, linked %q", store.put, store.deleted, repo.resume)
	}
	if _, err := s.DeleteResume(ctx, "u1"); err != nil || repo.resume != "" || len(store.deleted) != 2 {
		t.Fatalf("delete: %q %v %v", repo.resume, store.deleted, err)
	}
}
