package account

import (
	"context"
	"errors"
	"testing"

	"findJobs/internal/domain/account"
	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

type fakeRepo struct {
	ports.AccountRepository
	latest  map[string]account.Consent
	deleted bool
	err     error
}

func (f *fakeRepo) LatestConsents(context.Context, string) (map[string]account.Consent, error) {
	return f.latest, nil
}
func (f *fakeRepo) RecordConsent(_ context.Context, _, docID, version, status string) error {
	if f.latest == nil {
		f.latest = map[string]account.Consent{}
	}
	f.latest[docID] = account.Consent{DocID: docID, Version: version, Status: status}
	return nil
}
func (f *fakeRepo) DeleteAccount(context.Context, string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.deleted = true
	return []string{"img/me.jpg"}, nil
}

type fakeSessions struct {
	ports.SessionStore
	called bool
}

func (f *fakeSessions) DeleteUserSessions(_ context.Context, userID string) ([]auth.Session, error) {
	f.called = true
	return []auth.Session{{ID: "s1", RefreshToken: "rt1"}, {ID: "s2"}, {ID: "s3", RefreshToken: "rt3"}}, nil
}

type fakeIAM struct {
	ports.IAMClient
	revoked []string
}

func (f *fakeIAM) Logout(_ context.Context, token string) error {
	f.revoked = append(f.revoked, token)
	return errors.New("IAM down") // logged, never fails the deletion
}

type fakeStore struct {
	ports.ObjectStore
	deleted []string
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func TestConsentGate(t *testing.T) {
	repo := &fakeRepo{}
	s := NewAccountService(repo, nil, nil, nil, []account.Document{{ID: "terms", Version: "v2"}, {ID: "privacy", Version: "v1"}})
	var pending *account.ConsentRequiredError
	if err := s.Gate(context.Background(), "u"); !errors.As(err, &pending) || len(pending.Documents) != 2 {
		t.Fatalf("new user: %v", err)
	}
	if _, err := s.Decide(context.Background(), "u", "terms", "v1", account.Accepted); err == nil {
		t.Fatal("an old version can't be accepted")
	}
	left, err := s.Decide(context.Background(), "u", "terms", "v2", account.Accepted)
	if err != nil || len(left) != 1 || left[0].ID != "privacy" {
		t.Fatalf("after terms: %+v %v", left, err)
	}
	s.Decide(context.Background(), "u", "privacy", "v1", account.Accepted)
	if err := s.Gate(context.Background(), "u"); err != nil {
		t.Fatalf("all accepted: %v", err)
	}
	// No documents configured: nothing is required, the repo isn't read.
	if err := NewAccountService(nil, nil, nil, nil, nil).Gate(context.Background(), "u"); err != nil {
		t.Fatalf("no documents: %v", err)
	}
}

func TestDeleteEndsAndRevokesSessions(t *testing.T) {
	repo, sessions, iam, store := &fakeRepo{}, &fakeSessions{}, &fakeIAM{}, &fakeStore{}
	s := NewAccountService(repo, sessions, iam, store, nil)
	if err := s.Delete(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}
	if !repo.deleted || !sessions.called || len(store.deleted) != 1 || store.deleted[0] != "img/me.jpg" {
		t.Fatalf("deleted %v, sessions %v, files %v", repo.deleted, sessions.called, store.deleted)
	}
	if len(iam.revoked) != 2 || iam.revoked[0] != "rt1" || iam.revoked[1] != "rt3" {
		t.Fatalf("revoked %v (sessions without a readable token are skipped)", iam.revoked)
	}

	// The only admin of a company: nothing else happens.
	repo, sessions = &fakeRepo{err: &account.SoleAdminError{Companies: []string{"Acme"}}}, &fakeSessions{}
	s = NewAccountService(repo, sessions, &fakeIAM{}, &fakeStore{}, nil)
	var sole *account.SoleAdminError
	if err := s.Delete(context.Background(), "u"); !errors.As(err, &sole) || sessions.called {
		t.Fatalf("sole admin: %v, sessions ended %v", err, sessions.called)
	}
}
