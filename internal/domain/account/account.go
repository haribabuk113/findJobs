// Package account holds the account and legal rules (system requirements
// §12–14, §21): legal documents the user must accept, consent records, and
// account deletion.
package account

import (
	"strings"
	"time"
)

// Document is a legal document at its current version (LEGAL_DOCUMENTS),
// e.g. {"terms", "2026-11-02"}. Every listed document is mandatory.
type Document struct {
	ID      string `json:"doc_id"`
	Version string `json:"version"`
}

// Consent statuses. A record is never changed: withdrawing is a new record.
const (
	Accepted  = "accepted"
	Withdrawn = "withdrawn"
)

// Consent is the user's latest decision on a document.
type Consent struct {
	DocID   string    `json:"doc_id"`
	Version string    `json:"version"`
	Status  string    `json:"status"`
	At      time.Time `json:"at"`
}

// Required is a document the user still has to accept, with the version
// they accepted before ("" if none), so the page can say "updated".
type Required struct {
	Document
	AcceptedVersion string `json:"accepted_version"`
}

// Pending lists the documents whose current version the user hasn't accepted
// (latest consents by doc id), in the documents' order.
func Pending(docs []Document, latest map[string]Consent) []Required {
	out := []Required{}
	for _, d := range docs {
		c, ok := latest[d.ID]
		if ok && c.Status == Accepted && c.Version == d.Version {
			continue
		}
		r := Required{Document: d}
		if ok && c.Status == Accepted {
			r.AcceptedVersion = c.Version
		}
		out = append(out, r)
	}
	return out
}

// CheckDecision validates a consent request: only a document's current
// version can be accepted or withdrawn.
func CheckDecision(docs []Document, docID, version, status string) error {
	if status != Accepted && status != Withdrawn {
		return &ValidationError{Message: "status must be accepted or withdrawn"}
	}
	for _, d := range docs {
		if d.ID == docID {
			if d.Version != version {
				return &ValidationError{Message: "that isn't the current version of " + docID + "; reload and try again"}
			}
			return nil
		}
	}
	return &ValidationError{Message: "unknown document " + docID}
}

// SoleAdminError: the user is the only admin of these companies, so the
// account can't be deleted until they appoint another admin or delete them.
type SoleAdminError struct{ Companies []string }

func (e *SoleAdminError) Error() string {
	return "you're the only admin of " + strings.Join(e.Companies, ", ") +
		": make someone else an admin, or delete the company, before deleting your account"
}

// ValidationError: a malformed consent request.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

// ConsentRequiredError: the user must accept these documents first (the gate).
type ConsentRequiredError struct{ Documents []Required }

func (e *ConsentRequiredError) Error() string {
	return "accept the latest terms to continue"
}
