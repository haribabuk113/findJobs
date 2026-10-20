package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	appjobs "findJobs/internal/application/jobs"
	domainjobs "findJobs/internal/domain/jobs"

	"github.com/gin-gonic/gin"
)

// RecruitingHandler handles posting, managing and viewing jobs.
type RecruitingHandler struct {
	service appjobs.RecruitingService
}

// NewRecruitingHandler creates a new HTTP handler for recruiting with dependency injection.
func NewRecruitingHandler(service appjobs.RecruitingService) *RecruitingHandler {
	return &RecruitingHandler{service: service}
}

// RegisterRecruitingRoutes registers recruiter, job-details and recruiter-card routes on api.
func RegisterRecruitingRoutes(api *gin.RouterGroup, h *RecruitingHandler, optionalUser, requireUser gin.HandlerFunc) {
	r := api.Group("/recruiter", requireUser)
	r.GET("/companies", h.Companies)
	r.GET("/jobs", h.Mine)
	r.POST("/jobs", h.Post)
	r.PUT("/jobs/:id", h.Edit)
	r.POST("/jobs/:id/close", h.Close)
	r.POST("/jobs/:id/repost", h.Repost)
	r.GET("/drafts", h.Drafts)
	r.POST("/drafts", h.SaveDraft)
	r.PUT("/drafts/:id", h.SaveDraft)
	r.DELETE("/drafts/:id", h.DeleteDraft)

	j := api.Group("/jobs")
	j.GET("/share/:hash", optionalUser, h.Shared)
	j.GET("/:id", optionalUser, h.Detail)
	j.POST("/:id/outbound", requireUser, h.Outbound)

	api.GET("/recruiters/:id", h.Recruiter)
}

// Companies handles GET /recruiter/companies: where the user may post.
func (h *RecruitingHandler) Companies(ctx *gin.Context) {
	result, err := h.service.PosterCompanies(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Companies retrieved successfully", result, err)
}

// Post handles POST /recruiter/jobs.
func (h *RecruitingHandler) Post(ctx *gin.Context) {
	var in domainjobs.PostInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid job post"))
		return
	}
	result, err := h.service.Post(ctx, ctx.GetString(userIDKey), &in)
	h.respond(ctx, http.StatusCreated, "Job published successfully", result, err)
}

// Edit handles PUT /recruiter/jobs/:id (tier, company and payment can't change).
func (h *RecruitingHandler) Edit(ctx *gin.Context) {
	var in domainjobs.PostInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid job post"))
		return
	}
	result, err := h.service.Edit(ctx, ctx.GetString(userIDKey), ctx.Param("id"), &in)
	h.respond(ctx, http.StatusOK, "Job updated successfully", result, err)
}

// Close handles POST /recruiter/jobs/:id/close.
func (h *RecruitingHandler) Close(ctx *gin.Context) {
	err := h.service.Close(ctx, ctx.GetString(userIDKey), ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Job closed", nil, err)
}

// Repost handles POST /recruiter/jobs/:id/repost.
func (h *RecruitingHandler) Repost(ctx *gin.Context) {
	err := h.service.Repost(ctx, ctx.GetString(userIDKey), ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Job reposted for another 30 days", nil, err)
}

// Mine handles GET /recruiter/jobs?status=active|closed|expired
func (h *RecruitingHandler) Mine(ctx *gin.Context) {
	result, err := h.service.Mine(ctx, ctx.GetString(userIDKey), ctx.Query("status"))
	h.respond(ctx, http.StatusOK, "Jobs retrieved successfully", result, err)
}

// Detail handles GET /jobs/:id.
func (h *RecruitingHandler) Detail(ctx *gin.Context) {
	result, err := h.service.Detail(ctx, ctx.Param("id"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Job retrieved successfully", result, err)
}

// Shared handles GET /jobs/share/:hash.
func (h *RecruitingHandler) Shared(ctx *gin.Context) {
	result, err := h.service.Shared(ctx, ctx.Param("hash"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Job retrieved successfully", result, err)
}

// Outbound handles POST /jobs/:id/outbound: an external-apply click.
func (h *RecruitingHandler) Outbound(ctx *gin.Context) {
	err := h.service.Outbound(ctx, ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Click recorded", nil, err)
}

// Recruiter handles GET /recruiters/:id: a public recruiter card.
func (h *RecruitingHandler) Recruiter(ctx *gin.Context) {
	person, open, err := h.service.Recruiter(ctx, ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Recruiter retrieved successfully", gin.H{"recruiter": person, "open_jobs": open}, err)
}

// Drafts handles GET /recruiter/drafts.
func (h *RecruitingHandler) Drafts(ctx *gin.Context) {
	result, err := h.service.Drafts(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Drafts retrieved successfully", result, err)
}

// SaveDraft handles POST /recruiter/drafts and PUT /recruiter/drafts/:id with {"form": {...}}.
func (h *RecruitingHandler) SaveDraft(ctx *gin.Context) {
	var body struct {
		Form map[string]any `json:"form"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Form == nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid draft"))
		return
	}
	result, err := h.service.SaveDraft(ctx, ctx.GetString(userIDKey), ctx.Param("id"), body.Form)
	h.respond(ctx, http.StatusOK, "Draft saved", result, err)
}

// DeleteDraft handles DELETE /recruiter/drafts/:id.
func (h *RecruitingHandler) DeleteDraft(ctx *gin.Context) {
	err := h.service.DeleteDraft(ctx, ctx.GetString(userIDKey), ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Draft deleted", nil, err)
}

func (h *RecruitingHandler) respond(ctx *gin.Context, status int, message string, result any, err error) {
	if err != nil {
		switch {
		case domainjobs.IsValidation(err):
			h.fail(ctx, http.StatusBadRequest, err)
		case domainjobs.IsNotFound(err):
			h.fail(ctx, http.StatusNotFound, err)
		case domainjobs.IsForbidden(err):
			h.fail(ctx, http.StatusForbidden, err)
		case domainjobs.IsPaymentRequired(err):
			h.fail(ctx, http.StatusPaymentRequired, err)
		default:
			ctx.Error(err)
			h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
		}
		return
	}
	// Job pages depend on the viewer (applied, manage rights, analytics).
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(status, adapters.BuildResponse(message, result, nil))
}

func (h *RecruitingHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
