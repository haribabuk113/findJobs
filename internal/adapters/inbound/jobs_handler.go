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
func RegisterJobRoutes(rg *gin.RouterGroup, handler *JobsHandler, optionalUser gin.HandlerFunc) {
	rg.POST("/search", optionalUser, handler.Search)
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
//	@Failure	400,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/search [post]
func (h *JobsHandler) Search(ctx *gin.Context) {
	var criteria domainjobs.SearchCriteria
	if err := ctx.ShouldBindJSON(&criteria); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid search request"))
		return
	}

	result, err := h.service.Search(ctx, &criteria, ctx.GetString(userIDKey))
	if err != nil {
		if domainjobs.IsValidation(err) {
			h.fail(ctx, http.StatusBadRequest, err)
			return
		}
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
		return
	}

	// Results depend on the viewer (applied-jobs exclusion): never share-cache.
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, adapters.BuildResponse("Jobs retrieved successfully", result, nil))
}

func (h *JobsHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
