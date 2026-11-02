package inbound

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainaccount "findJobs/internal/domain/account"
	domainapplications "findJobs/internal/domain/applications"
	domainauth "findJobs/internal/domain/auth"
	domaincompanies "findJobs/internal/domain/companies"
	domainjobs "findJobs/internal/domain/jobs"
	domainpayments "findJobs/internal/domain/payments"
	domainprofiles "findJobs/internal/domain/profiles"
	domainreference "findJobs/internal/domain/reference"
	domainsettings "findJobs/internal/domain/settings"

	"github.com/gin-gonic/gin"
)

// Every domain's errors map to the same status, whichever handler they reach.
func TestFailErrMapsEveryDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err  error
		want int
	}{
		{&domainapplications.ValidationError{Message: "x"}, 400},
		{domainauth.NewValidationError("code", "x"), 400},
		{&domaincompanies.ValidationError{Message: "x"}, 400},
		{&domainjobs.ValidationError{Message: "x"}, 400},
		{&domainpayments.ValidationError{Message: "x"}, 400},
		{&domainprofiles.ValidationError{Message: "x"}, 400},
		{&domainreference.ValidationError{Message: "x"}, 400},
		{&domainsettings.ValidationError{Message: "x"}, 400},
		{&domainaccount.ValidationError{Message: "x"}, 400},
		{domainauth.ErrUnauthorized, 401},
		{&domaincompanies.ForbiddenError{Message: "x"}, 403},
		{&domainjobs.ForbiddenError{Message: "x"}, 403},
		{&domainapplications.NotFoundError{Message: "x"}, 404},
		{domainauth.ErrUserNotFound, 404},
		{domaincompanies.ErrNotFound, 404},
		{&domainjobs.NotFoundError{Message: "x"}, 404},
		{domainprofiles.ErrNotFound, 404},
		{&domainreference.NotFoundError{Message: "x"}, 404},
		{&domainapplications.ConflictError{Message: "x"}, 409},
		{&domaincompanies.ConflictError{Message: "x"}, 409},
		{&domainjobs.ConflictError{Message: "x"}, 409},
		{domainprofiles.ErrSlugTaken, 409},
		{&domainreference.ConflictError{Message: "x"}, 409},
		{&domainpayments.UnavailableError{Message: "x"}, 503},
		{errors.New("db exploded"), 500},
	} {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		failErr(ctx, tc.err)
		if w.Code != tc.want {
			t.Errorf("%T: got %d, want %d", tc.err, w.Code, tc.want)
		}
		if tc.want == 500 && (strings.Contains(w.Body.String(), "exploded") || len(ctx.Errors) != 1) {
			t.Errorf("a 500 must hide its cause from the client and keep it for the log: %s", w.Body)
		}
	}
}

// The errors that carry data keep it in the envelope's data field.
func TestFailErrKeepsData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err  error
		want int
		key  string
	}{
		{&domainjobs.PaymentRequiredError{Reason: "insufficient_balance", FeePaise: 2000}, 402, "fee_paise"},
		{&domainapplications.PaymentRequiredError{Reason: "confirm_paid", FeePaise: 500}, 402, "reason"},
		{&domainaccount.ConsentRequiredError{Documents: []domainaccount.Required{{}}}, 403, "required_documents"},
		{fmt.Errorf("wrapped: %w", &domainaccount.SoleAdminError{Companies: []string{"Acme"}}), 409, "companies"},
	} {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		failErr(ctx, tc.err)
		var body struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != tc.want || body.Data[tc.key] == nil {
			t.Errorf("%T: %d %s", tc.err, w.Code, w.Body)
		}
	}
	// respond: success is never share-cached.
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	respond(ctx, http.StatusCreated, "ok", gin.H{"a": 1}, nil)
	if w.Code != 201 || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("respond: %d %q", w.Code, w.Header().Get("Cache-Control"))
	}
}
