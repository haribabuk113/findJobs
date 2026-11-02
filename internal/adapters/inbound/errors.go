package inbound

import (
	"errors"
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
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

// One error mapper for every handler, so a domain error gets the same status
// wherever it surfaces (a profile save can fail with a reference or company
// error, a job post with a payments one). Anything unknown is a 500 whose
// cause goes to ctx.Errors (the request log), never to the client.

// respond writes result, or the mapped error. Responses are per user or
// change at any moment, so they are never share-cached.
func respond(ctx *gin.Context, status int, message string, result any, err error) {
	if err != nil {
		failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(status, adapters.BuildResponse(message, result, nil))
}

// fail writes an error envelope with the given status.
func fail(ctx *gin.Context, status int, err error) {
	failWith(ctx, status, nil, err)
}

func failWith(ctx *gin.Context, status int, data any, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), data, err))
}

// failErr maps a domain error to its status.
func failErr(ctx *gin.Context, err error) {
	var consent *domainaccount.ConsentRequiredError
	var soleAdmin *domainaccount.SoleAdminError
	var accountInvalid *domainaccount.ValidationError
	switch {
	case domainapplications.IsPaymentRequired(err), domainjobs.IsPaymentRequired(err):
		// The error is also the data: reason, fee and balance for the modal.
		failWith(ctx, http.StatusPaymentRequired, err, err)
	case errors.As(err, &consent):
		failWith(ctx, http.StatusForbidden, gin.H{"required_documents": consent.Documents}, err)
	case errors.As(err, &soleAdmin):
		failWith(ctx, http.StatusConflict, gin.H{"companies": soleAdmin.Companies}, err)
	case errors.As(err, &accountInvalid), domainapplications.IsValidation(err), domainauth.IsValidation(err),
		domaincompanies.IsValidation(err), domainjobs.IsValidation(err), domainpayments.IsValidation(err),
		domainprofiles.IsValidation(err), domainreference.IsValidation(err), domainsettings.IsValidation(err):
		fail(ctx, http.StatusBadRequest, err)
	case domainauth.IsUnauthorized(err):
		fail(ctx, http.StatusUnauthorized, err)
	case domaincompanies.IsForbidden(err), domainjobs.IsForbidden(err):
		fail(ctx, http.StatusForbidden, err)
	case domainapplications.IsNotFound(err), domainauth.IsNotFound(err), domaincompanies.IsNotFound(err),
		domainjobs.IsNotFound(err), domainprofiles.IsNotFound(err), domainreference.IsNotFound(err):
		fail(ctx, http.StatusNotFound, err)
	case domainapplications.IsConflict(err), domaincompanies.IsConflict(err), domainjobs.IsConflict(err),
		domainprofiles.IsConflict(err), domainreference.IsConflict(err):
		fail(ctx, http.StatusConflict, err)
	case domainpayments.IsUnavailable(err):
		fail(ctx, http.StatusServiceUnavailable, err)
	default:
		ctx.Error(err)
		fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}
