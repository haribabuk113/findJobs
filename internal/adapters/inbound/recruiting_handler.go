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
	r.PUT("/drafts/:id", h.OverwriteDraft)
	r.DELETE("/drafts/:id", h.DeleteDraft)

	j := api.Group("/jobs")
	j.GET("/share/:hash", optionalUser, h.Shared)
	j.GET("/:id", optionalUser, h.Detail)
	j.POST("/:id/outbound", requireUser, h.Outbound)

	api.GET("/recruiters/:id", h.Recruiter)
}

type draftRequest struct {
	Form map[string]any `json:"form"`
}

type recruiterCard struct {
	Recruiter *domainjobs.Person `json:"recruiter"`
	OpenJobs  int                `json:"open_jobs"`
}

// Companies handles GET /recruiter/companies: where the user may post.
//
//	@Summary	Companies the user may post under
//	@ID			getPosterCompanies
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=[]domainjobs.PosterCompany}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/companies [get]
func (h *RecruitingHandler) Companies(ctx *gin.Context) {
	result, err := h.service.PosterCompanies(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Companies retrieved successfully", result, err)
}

// Post handles POST /recruiter/jobs.
//
//	@Summary	Publish a job
//	@ID			postJob
//	@Tags		recruiting
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	domainjobs.PostInput	true	"The job"
//	@Success	201	{object}	adapters.Response{data=domainjobs.Detail}
//	@Failure	400,401,402,403,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/jobs [post]
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
//
//	@Summary	Edit a job (tier, company and payment stay)
//	@ID			editJob
//	@Tags		recruiting
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Job id"
//	@Param		body	body	domainjobs.PostInput	true	"The job"
//	@Success	200	{object}	adapters.Response{data=domainjobs.Detail}
//	@Failure	400,401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/jobs/{id} [put]
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
//
//	@Summary	Close a job
//	@ID			closeJob
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Job id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/jobs/{id}/close [post]
func (h *RecruitingHandler) Close(ctx *gin.Context) {
	err := h.service.Close(ctx, ctx.GetString(userIDKey), ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Job closed", nil, err)
}

// Repost handles POST /recruiter/jobs/:id/repost.
//
//	@Summary	Repost a job for another 30 days
//	@ID			repostJob
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Job id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/jobs/{id}/repost [post]
func (h *RecruitingHandler) Repost(ctx *gin.Context) {
	err := h.service.Repost(ctx, ctx.GetString(userIDKey), ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Job reposted for another 30 days", nil, err)
}

// Mine handles GET /recruiter/jobs?status=active|closed|expired
//
//	@Summary	Jobs the user posted
//	@ID			getMyPostedJobs
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Param		status	query	string	false	"active, closed or expired"
//	@Success	200	{object}	adapters.Response{data=[]domainjobs.Posted}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/jobs [get]
func (h *RecruitingHandler) Mine(ctx *gin.Context) {
	result, err := h.service.Mine(ctx, ctx.GetString(userIDKey), ctx.Query("status"))
	h.respond(ctx, http.StatusOK, "Jobs retrieved successfully", result, err)
}

// Detail handles GET /jobs/:id.
//
//	@Summary	Job page (session optional)
//	@ID			getJob
//	@Tags		jobs
//	@Produce	json
//	@Param		id	path	string	true	"Job id"
//	@Success	200	{object}	adapters.Response{data=domainjobs.Detail}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/{id} [get]
func (h *RecruitingHandler) Detail(ctx *gin.Context) {
	result, err := h.service.Detail(ctx, ctx.Param("id"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Job retrieved successfully", result, err)
}

// Shared handles GET /jobs/share/:hash.
//
//	@Summary	Job page by share link (session optional)
//	@ID			getSharedJob
//	@Tags		jobs
//	@Produce	json
//	@Param		hash	path	string	true	"Share hash"
//	@Success	200	{object}	adapters.Response{data=domainjobs.Detail}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/share/{hash} [get]
func (h *RecruitingHandler) Shared(ctx *gin.Context) {
	result, err := h.service.Shared(ctx, ctx.Param("hash"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Job retrieved successfully", result, err)
}

// Outbound handles POST /jobs/:id/outbound: an external-apply click.
//
//	@Summary	Record an external-apply click
//	@ID			recordOutboundClick
//	@Tags		jobs
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Job id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,404,500	{object}	adapters.Response
//	@Router		/api/v1/jobs/{id}/outbound [post]
func (h *RecruitingHandler) Outbound(ctx *gin.Context) {
	err := h.service.Outbound(ctx, ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Click recorded", nil, err)
}

// Recruiter handles GET /recruiters/:id: a public recruiter card.
//
//	@Summary	Public recruiter card
//	@ID			getRecruiter
//	@Tags		recruiting
//	@Produce	json
//	@Param		id	path	string	true	"Recruiter user id"
//	@Success	200	{object}	adapters.Response{data=recruiterCard}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiters/{id} [get]
func (h *RecruitingHandler) Recruiter(ctx *gin.Context) {
	person, open, err := h.service.Recruiter(ctx, ctx.Param("id"))
	h.respond(ctx, http.StatusOK, "Recruiter retrieved successfully", recruiterCard{Recruiter: person, OpenJobs: open}, err)
}

// Drafts handles GET /recruiter/drafts.
//
//	@Summary	The user's post-a-job drafts
//	@ID			getDrafts
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=[]domainjobs.Draft}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/drafts [get]
func (h *RecruitingHandler) Drafts(ctx *gin.Context) {
	result, err := h.service.Drafts(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Drafts retrieved successfully", result, err)
}

// SaveDraft handles POST /recruiter/drafts and PUT /recruiter/drafts/:id with {"form": {...}}.
//
//	@Summary	Save a new draft
//	@ID			createDraft
//	@Tags		recruiting
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	draftRequest	true	"Unvalidated form state, at most 64 KB"
//	@Success	200	{object}	adapters.Response{data=domainjobs.Draft}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/drafts [post]
func (h *RecruitingHandler) SaveDraft(ctx *gin.Context) {
	var body draftRequest
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Form == nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid draft"))
		return
	}
	result, err := h.service.SaveDraft(ctx, ctx.GetString(userIDKey), ctx.Param("id"), body.Form)
	h.respond(ctx, http.StatusOK, "Draft saved", result, err)
}

// OverwriteDraft handles PUT /recruiter/drafts/:id.
//
//	@Summary	Overwrite a draft
//	@ID			updateDraft
//	@Tags		recruiting
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Draft id"
//	@Param		body	body	draftRequest	true	"Unvalidated form state, at most 64 KB"
//	@Success	200	{object}	adapters.Response{data=domainjobs.Draft}
//	@Failure	400,401,404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/drafts/{id} [put]
func (h *RecruitingHandler) OverwriteDraft(ctx *gin.Context) { h.SaveDraft(ctx) }

// DeleteDraft handles DELETE /recruiter/drafts/:id.
//
//	@Summary	Delete a draft
//	@ID			deleteDraft
//	@Tags		recruiting
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	string	true	"Draft id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,404,500	{object}	adapters.Response
//	@Router		/api/v1/recruiter/drafts/{id} [delete]
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
