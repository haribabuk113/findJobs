package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	appjobs "findJobs/internal/application/jobs"
	domainjobs "findJobs/internal/domain/jobs"

	"github.com/gin-gonic/gin"
)

// JobsHandler handles job search HTTP requests.
type JobsHandler struct {
	service appjobs.JobService
}

// NewJobsHandler creates a new HTTP handler for jobs with dependency injection.
func NewJobsHandler(service appjobs.JobService) *JobsHandler {
	return &JobsHandler{service: service}
}

// RegisterJobRoutes registers job routes. optionalUser resolves the session
// cookie when present so signed-in candidates never see jobs they applied to.
func RegisterJobRoutes(rg *gin.RouterGroup, handler *JobsHandler, optionalUser, requireUser gin.HandlerFunc) {
	rg.POST("/search", optionalUser, handler.Search)
	rg.POST("/search/unlocks", requireUser, handler.Unlock)
}

type unlockRequest struct {
	Criteria domainjobs.SearchCriteria `json:"criteria"`
}

// Unlock handles POST /jobs/search/unlocks {"criteria": {...}}: buys sort=match for these filters.
//
//	@Summary	Unlock match-score sorting for a set of filters (charged from the wallet)
//	@ID			unlockMatchSort
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	unlockRequest	true	"The filters to unlock (sort and page are ignored)"
//	@Success	201	{object}	adapters.Response{data=domainjobs.Unlock}
//	@Failure	402	{object}	adapters.Response{data=domainjobs.PaymentRequiredError}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/search/unlocks [post]
func (h *JobsHandler) Unlock(ctx *gin.Context) {
	var in unlockRequest
	if err := ctx.ShouldBindJSON(&in); err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid unlock request"))
		return
	}
	result, err := h.service.Unlock(ctx, &in.Criteria, ctx.GetString(userIDKey))
	if err != nil {
		failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusCreated, adapters.BuildResponse("Match sorting unlocked", result, nil))
}

// Search handles POST /jobs/search with the consolidated filter payload.
//
//	@Summary	Search public job listings (session optional)
//	@ID			searchJobs
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domainjobs.SearchCriteria	true	"Consolidated filters"
//	@Success	200	{object}	adapters.Response{data=domainjobs.SearchResult}
//	@Failure	402	{object}	adapters.Response{data=domainjobs.PaymentRequiredError}
//	@Failure	400,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/search [post]
func (h *JobsHandler) Search(ctx *gin.Context) {
	var criteria domainjobs.SearchCriteria
	if err := ctx.ShouldBindJSON(&criteria); err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid search request"))
		return
	}

	result, err := h.service.Search(ctx, &criteria, ctx.GetString(userIDKey))
	if err != nil {
		failErr(ctx, err)
		return
	}

	// Results depend on the viewer (applied-jobs exclusion): never share-cache.
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, adapters.BuildResponse("Jobs retrieved successfully", result, nil))
}
