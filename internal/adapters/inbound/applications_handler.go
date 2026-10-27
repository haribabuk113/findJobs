package inbound

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"findJobs/internal/adapters"
	appapplications "findJobs/internal/application/applications"
	domainapplications "findJobs/internal/domain/applications"

	"github.com/gin-gonic/gin"
)

// ApplicationsHandler handles applying to jobs and the applicant pipeline.
type ApplicationsHandler struct {
	service appapplications.ApplicationService
}

// NewApplicationsHandler creates the HTTP handler for applications.
func NewApplicationsHandler(service appapplications.ApplicationService) *ApplicationsHandler {
	return &ApplicationsHandler{service: service}
}

// RegisterApplicationRoutes registers the candidate and recruiter application routes on api.
func RegisterApplicationRoutes(api *gin.RouterGroup, h *ApplicationsHandler, requireUser gin.HandlerFunc) {
	a := api.Group("/applications", requireUser)
	a.GET("", h.Mine)
	a.POST("", h.Apply)
	a.POST("/external", h.ApplyExternal)
	a.GET("/quota", h.Quota)

	r := api.Group("/recruiter", requireUser)
	r.GET("/jobs/:id/applications", h.Applicants)
	r.PATCH("/applications/:id", h.SetStage)
}

var applicationID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type jobRef struct {
	JobID string `json:"job_id"`
}

func (h *ApplicationsHandler) bindJob(ctx *gin.Context) (string, bool) {
	var in jobRef
	if err := ctx.ShouldBindJSON(&in); err != nil || in.JobID == "" || len(in.JobID) > 150 {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("give a job_id"))
		return "", false
	}
	return in.JobID, true
}

// Apply handles POST /applications {"job_id"}: Easy Apply.
func (h *ApplicationsHandler) Apply(ctx *gin.Context) {
	jobID, ok := h.bindJob(ctx)
	if !ok {
		return
	}
	result, err := h.service.Apply(ctx, ctx.GetString(userIDKey), jobID)
	h.respond(ctx, http.StatusCreated, "Application submitted", result, err)
}

// ApplyExternal handles POST /applications/external {"job_id"}: "Yes, I applied".
func (h *ApplicationsHandler) ApplyExternal(ctx *gin.Context) {
	jobID, ok := h.bindJob(ctx)
	if !ok {
		return
	}
	result, err := h.service.ApplyExternal(ctx, ctx.GetString(userIDKey), jobID)
	h.respond(ctx, http.StatusCreated, "Application recorded", result, err)
}

// Mine handles GET /applications.
func (h *ApplicationsHandler) Mine(ctx *gin.Context) {
	result, err := h.service.Mine(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Applications retrieved successfully", result, err)
}

// Quota handles GET /applications/quota: today's free applications.
func (h *ApplicationsHandler) Quota(ctx *gin.Context) {
	result, err := h.service.Quota(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Quota retrieved successfully", result, err)
}

// Applicants handles GET /recruiter/jobs/:id/applications?stage=&min_match=&sort=recent|match.
func (h *ApplicationsHandler) Applicants(ctx *gin.Context) {
	f := domainapplications.ApplicantFilter{Stage: ctx.Query("stage"), Sort: ctx.Query("sort")}
	if raw := ctx.Query("min_match"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			h.fail(ctx, http.StatusBadRequest, fmt.Errorf("min_match must be a number"))
			return
		}
		f.MinMatch = n
	}
	result, err := h.service.Applicants(ctx, ctx.Param("id"), ctx.GetString(userIDKey), f)
	h.respond(ctx, http.StatusOK, "Applicants retrieved successfully", result, err)
}

// SetStage handles PATCH /recruiter/applications/:id {"stage"}.
func (h *ApplicationsHandler) SetStage(ctx *gin.Context) {
	if !applicationID.MatchString(ctx.Param("id")) {
		h.fail(ctx, http.StatusNotFound, fmt.Errorf("application not found"))
		return
	}
	var in struct {
		Stage string `json:"stage"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("give a stage"))
		return
	}
	result, err := h.service.SetStage(ctx, ctx.Param("id"), ctx.GetString(userIDKey), in.Stage)
	h.respond(ctx, http.StatusOK, "Stage updated", result, err)
}

func (h *ApplicationsHandler) respond(ctx *gin.Context, status int, message string, result any, err error) {
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(status, adapters.BuildResponse(message, result, nil))
}

func (h *ApplicationsHandler) failErr(ctx *gin.Context, err error) {
	switch {
	case domainapplications.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	case domainapplications.IsNotFound(err):
		h.fail(ctx, http.StatusNotFound, err)
	case domainapplications.IsConflict(err):
		h.fail(ctx, http.StatusConflict, err)
	case domainapplications.IsPaymentRequired(err):
		h.fail(ctx, http.StatusPaymentRequired, err)
	default:
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *ApplicationsHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
