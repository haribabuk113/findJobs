package applications

import (
	"fmt"
	"strings"
)

// MaxBulkJobs caps one bulk apply (manual "Saved jobs": bulk apply from the saved list).
const MaxBulkJobs = 50

// Reasons a job in a bulk apply can't be applied to.
const (
	ReasonNotOpen        = "not_open" // missing, closed or past the 30-day window
	ReasonAlreadyApplied = "already_applied"
	ReasonOwnJob         = "own_job"
)

// BulkJob is what the repository read about one requested job.
type BulkJob struct {
	ID       string
	Open     bool // exists, active, inside the 30-day window
	External bool
	Own      bool
	Applied  bool
}

type BulkProblem struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason"`
}

// BulkPlan is the free/paid split the candidate confirms before submitting
// (manual: "Count of remaining free daily applications to be consumed, count
// of paid applications required, total credit cost").
type BulkPlan struct {
	EasyApply []string      `json:"easy_apply"` // job ids, in request order
	External  []string      `json:"external"`   // logged as applied; the client opens their sites
	Free      int           `json:"free"`       // Easy Apply jobs covered by today's free applications
	Paid      int           `json:"paid"`       // Easy Apply jobs past the free allowance
	FeePaise  int64         `json:"fee_paise"`  // Paid × the apply fee
	Problems  []BulkProblem `json:"problems"`   // jobs that block the batch; drop them and retry
}

// BulkResult is the plan, plus the new applications when it was submitted.
type BulkResult struct {
	BulkPlan
	DryRun       bool          `json:"dry_run"`
	Applications []Application `json:"applications"`
}

// NormalizeBulkJobIDs drops duplicates (keeping the first) and checks the count.
func NormalizeBulkJobIDs(ids []string) ([]string, error) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 150 {
			return nil, &ValidationError{Field: "job_ids", Message: "job_ids contains an invalid id"}
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 0 || len(out) > MaxBulkJobs {
		return nil, &ValidationError{Field: "job_ids", Message: fmt.Sprintf("give 1 to %d job ids", MaxBulkJobs)}
	}
	return out, nil
}

// PlanBulk splits the jobs into Easy Apply and External, uses the free
// allowance first, and lists the jobs that block the batch.
func PlanBulk(jobs []BulkJob, freeRemaining int, applyFeePaise int64) BulkPlan {
	p := BulkPlan{EasyApply: []string{}, External: []string{}, Problems: []BulkProblem{}}
	for _, j := range jobs {
		switch {
		case !j.Open:
			p.Problems = append(p.Problems, BulkProblem{j.ID, ReasonNotOpen})
		case j.Own:
			p.Problems = append(p.Problems, BulkProblem{j.ID, ReasonOwnJob})
		case j.Applied:
			p.Problems = append(p.Problems, BulkProblem{j.ID, ReasonAlreadyApplied})
		case j.External:
			p.External = append(p.External, j.ID)
		default:
			p.EasyApply = append(p.EasyApply, j.ID)
		}
	}
	p.Free = min(len(p.EasyApply), max(0, freeRemaining))
	p.Paid = len(p.EasyApply) - p.Free
	p.FeePaise = int64(p.Paid) * applyFeePaise
	return p
}

// Err is why the plan can't be submitted as it is (all or nothing), or nil.
func (p *BulkPlan) Err() error {
	if len(p.Problems) > 0 {
		parts := make([]string, len(p.Problems))
		for i, pr := range p.Problems {
			parts[i] = pr.JobID + " (" + strings.ReplaceAll(pr.Reason, "_", " ") + ")"
		}
		return &ConflictError{Message: "nothing was applied; remove these jobs and try again: " + strings.Join(parts, ", ")}
	}
	if p.Paid > 0 {
		return &PaymentRequiredError{Message: fmt.Sprintf(
			"nothing was applied; %d of these jobs need paid applications, which arrive with the wallet", p.Paid)}
	}
	return nil
}
