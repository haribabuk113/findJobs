package companies

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"slices"
	"testing"

	"findJobs/internal/domain/companies"
	"findJobs/internal/ports"
)

type imageRepo struct {
	ports.CompanyRepository
	old     string   // key SetProductImage replaces
	removed []string // keys Update reports
	fail    bool
}

func (r *imageRepo) SetProductImage(_ context.Context, _, _ string, _ int, key string) (string, error) {
	if r.fail {
		return "", errors.New("db down")
	}
	return r.old, nil
}
func (r *imageRepo) GetBySlug(context.Context, string, string) (*companies.Company, error) {
	return &companies.Company{}, nil
}
func (r *imageRepo) LocationCountries(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		out[id] = "IN"
	}
	return out, nil
}
func (r *imageRepo) Update(context.Context, string, string, *companies.Input, string) (*companies.Company, []string, error) {
	return &companies.Company{}, r.removed, nil
}

type imageStore struct {
	ports.ObjectStore
	put, deleted []string
}

func (s *imageStore) Put(_ context.Context, key, _ string, _ []byte) error {
	s.put = append(s.put, key)
	return nil
}
func (s *imageStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

// The replaced or dropped image file is deleted; an upload that can't be
// linked doesn't leave its file behind; non-images and oversize files are refused.
func TestProductImageFiles(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))

	repo, store := &imageRepo{old: "img/old.png"}, &imageStore{}
	s := NewCompanyService(repo, store).(*service)
	if _, err := s.SetProductImage(ctx, "u", "acme", 0, "image/png", buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if len(store.put) != 1 || !slices.Equal(store.deleted, []string{"img/old.png"}) {
		t.Fatalf("replace: put %v deleted %v", store.put, store.deleted)
	}

	store.put, store.deleted = nil, nil
	if _, err := s.SetProductImage(ctx, "u", "acme", 0, "", nil); err != nil || len(store.put) != 0 ||
		!slices.Equal(store.deleted, []string{"img/old.png"}) {
		t.Fatalf("remove: put %v deleted %v %v", store.put, store.deleted, err)
	}

	store.put, store.deleted, repo.fail = nil, nil, true
	if _, err := s.SetProductImage(ctx, "u", "acme", 0, "image/png", buf.Bytes()); err == nil ||
		len(store.put) != 1 || !slices.Equal(store.deleted, store.put) {
		t.Fatalf("unlinked upload must be removed: put %v deleted %v %v", store.put, store.deleted, err)
	}

	for name, data := range map[string][]byte{"empty": {}, "not an image": []byte("hello"),
		"over 1 MB": append(buf.Bytes(), make([]byte, companies.MaxProductImageBytes)...)} {
		if _, err := s.SetProductImage(ctx, "u", "acme", 0, "image/png", data); !companies.IsValidation(err) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}
	if _, err := s.SetProductImage(ctx, "u", "acme", -1, "", nil); !companies.IsNotFound(err) {
		t.Errorf("negative index: %v", err)
	}

	// A save that dropped a card's image deletes the file.
	repo.removed, store.deleted = []string{"img/dropped.png"}, nil
	in := &companies.Input{Name: "Acme", Slug: "acme", Size: "11–50 people", Industries: []string{"Testing"},
		Website: "https://acme.example", HQLocationID: "1277333"}
	if _, err := s.Update(ctx, "u", "acme", in); err != nil || !slices.Equal(store.deleted, repo.removed) {
		t.Fatalf("update: deleted %v %v", store.deleted, err)
	}
}
