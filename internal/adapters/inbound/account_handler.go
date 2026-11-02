package inbound

import (
	"fmt"
	"net/http"

	appaccount "findJobs/internal/application/account"
	domainaccount "findJobs/internal/domain/account"

	"github.com/gin-gonic/gin"
)

// AccountHandler handles consents and account deletion.
type AccountHandler struct {
	service appaccount.AccountService
}

// NewAccountHandler creates the HTTP handler for consents and the account.
func NewAccountHandler(service appaccount.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// RegisterAccountRoutes registers the consent and account routes on api (the
// /api/v1 group). requireUser must be the plain session check, not the
// consent gate: these are the routes a user who hasn't accepted can still use.
func RegisterAccountRoutes(api *gin.RouterGroup, h *AccountHandler, requireUser gin.HandlerFunc) {
	api.GET("/consents/required", requireUser, h.Required)
	api.POST("/consents", requireUser, h.Decide)
	api.DELETE("/account", requireUser, h.Delete)
}

type consentRequest struct {
	DocID   string `json:"doc_id"`
	Version string `json:"version"`
	// Status is accepted (the default) or withdrawn.
	Status string `json:"status"`
}

// Required handles GET /consents/required.
//
//	@Summary	Legal documents the user still has to accept
//	@ID			getRequiredConsents
//	@Tags		account
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=[]domainaccount.Required}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/consents/required [get]
func (h *AccountHandler) Required(ctx *gin.Context) {
	list, err := h.service.Required(ctx, ctx.GetString(userIDKey))
	respond(ctx, http.StatusOK, "Required documents retrieved successfully", list, err)
}

// Decide handles POST /consents: records an acceptance (or withdrawal) of a
// document's current version. Records are never changed, only added.
//
//	@Summary	Accept or withdraw a legal document
//	@ID			recordConsent
//	@Tags		account
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	consentRequest	true	"Document, its current version, and accepted (default) or withdrawn"
//	@Success	200	{object}	adapters.Response{data=[]domainaccount.Required}	"What is still required"
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/consents [post]
func (h *AccountHandler) Decide(ctx *gin.Context) {
	var in consentRequest
	if err := ctx.ShouldBindJSON(&in); err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("give doc_id and version"))
		return
	}
	if in.Status == "" {
		in.Status = domainaccount.Accepted
	}
	list, err := h.service.Decide(ctx, ctx.GetString(userIDKey), in.DocID, in.Version, in.Status)
	respond(ctx, http.StatusOK, "Consent recorded", list, err)
}

// Delete handles DELETE /account: deletes or anonymises the account's data,
// ends every session and revokes them at IAM. The IAM identity is kept.
//
//	@Summary	Delete your Referise account
//	@ID			deleteAccount
//	@Tags		account
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response
//	@Failure	401,409,500	{object}	adapters.Response	"409: the only admin of a company"
//	@Router		/api/v1/account [delete]
func (h *AccountHandler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx, ctx.GetString(userIDKey))
	respond(ctx, http.StatusOK, "Account deleted", nil, err)
}
