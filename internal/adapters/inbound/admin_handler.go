package inbound

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"findJobs/internal/application/admin"
	domainadmin "findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// AdminHandler handles the platform-admin (God Mode) HTTP routes.
type AdminHandler struct {
	service admin.AdminService
}

// NewAdminHandler creates a new HTTP handler for admin routes with dependency injection.
func NewAdminHandler(service admin.AdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

// paging reads page and page_size from the query string, applying defaults and
// the [1,100] page-size clamp shared by every paged admin list.
func paging(ctx *gin.Context) (page, pageSize int) {
	page = 1
	if p := ctx.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	pageSize = 20
	if ps := ctx.Query("page_size"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}
	return page, pageSize
}

// RegisterAdminRoutes registers all God Mode / platform-admin routes under /admin
// on api (the /api/v1 group). requireAdmin is authHandler.RequirePlatformAdmin.
func RegisterAdminRoutes(api *gin.RouterGroup, h *AdminHandler, requireAdmin gin.HandlerFunc) {
	a := api.Group("/admin", requireAdmin)
	a.GET("/stats", h.Stats)
	a.GET("/audit", h.ListAudit)

	a.GET("/companies", h.ListCompanies)
	a.GET("/companies/:slug", h.GetCompany)
	a.PATCH("/companies/:slug", h.PatchCompany)
	a.DELETE("/companies/:slug", h.DeleteCompany)
	a.PATCH("/companies/:slug/members/:user_id", h.SetMemberDecision)

	a.GET("/reports", h.ListReports)
	a.PATCH("/reports/:id", h.ResolveReport)

	a.POST("/subscriptions", h.CreateSubscription)
	a.GET("/subscriptions", h.ListSubscriptions)
	a.POST("/subscriptions/:id/extend", h.ExtendSubscription)
}

// Stats handles GET /admin/stats.
//
//	@Summary	Platform-wide stats for the God Mode dashboard
//	@ID			adminStats
//	@Tags		admin
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=ports.AdminStats}
//	@Failure	401,403,500	{object}	adapters.Response
//	@Router		/api/v1/admin/stats [get]
func (h *AdminHandler) Stats(ctx *gin.Context) {
	result, err := h.service.Stats(ctx.Request.Context())
	respond(ctx, http.StatusOK, "Platform stats retrieved successfully", result, err)
}

// ListAudit handles GET /admin/audit?limit=N.
//
//	@Summary	Recent God Mode audit rows
//	@ID			listAdminAudit
//	@Tags		admin
//	@Produce	json
//	@Security	sessionCookie
//	@Param		limit	query	int	false	"Max rows (default 50)"
//	@Success	200	{object}	adapters.Response{data=[]admin.AuditLog}
//	@Failure	401,403,500	{object}	adapters.Response
//	@Router		/api/v1/admin/audit [get]
func (h *AdminHandler) ListAudit(ctx *gin.Context) {
	limit := 50
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	result, err := h.service.ListAudit(ctx.Request.Context(), limit)
	respond(ctx, http.StatusOK, "Audit log retrieved successfully", result, err)
}

// ListCompanies handles GET /admin/companies?q=&page=&page_size=20
func (h *AdminHandler) ListCompanies(ctx *gin.Context) {
	page, pageSize := paging(ctx)
	input := admin.CompanyListInput{Query: ctx.Query("q"), Page: page, PageSize: pageSize}
	result, err := h.service.ListCompanies(ctx.Request.Context(), &input)
	respond(ctx, http.StatusOK, "Companies retrieved successfully", result, err)
}

// GetCompany handles GET /admin/companies/:slug
func (h *AdminHandler) GetCompany(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		respond(ctx, http.StatusBadRequest, "slug is required", nil, nil)
		return
	}
	result, err := h.service.GetCompany(ctx.Request.Context(), slug)
	respond(ctx, http.StatusOK, "Company retrieved successfully", result, err)
}

// PatchCompany handles PATCH /admin/companies/:slug
func (h *AdminHandler) PatchCompany(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		respond(ctx, http.StatusBadRequest, "slug is required", nil, nil)
		return
	}
	var in ports.CompanyAdminPatch
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	result, err := h.service.PatchCompany(ctx.Request.Context(), slug, ctx.GetString("user_id"), &in)
	respond(ctx, http.StatusOK, "Company overridden successfully", result, err)
}

// DeleteCompany handles DELETE /admin/companies/:slug
func (h *AdminHandler) DeleteCompany(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		respond(ctx, http.StatusBadRequest, "slug is required", nil, nil)
		return
	}
	adminID := ctx.GetString("user_id")
	err := h.service.DeleteCompany(ctx.Request.Context(), slug, adminID)
	respond(ctx, http.StatusOK, "Company deleted successfully", nil, err)
}

// SetMemberDecision handles PATCH /admin/companies/:slug/members/:user_id
func (h *AdminHandler) SetMemberDecision(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		respond(ctx, http.StatusBadRequest, "slug is required", nil, nil)
		return
	}
	var in ports.MemberAdminDecision
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	err := h.service.SetMemberDecision(ctx.Request.Context(), slug, ctx.GetString("user_id"), ctx.Param("user_id"), &in)
	respond(ctx, http.StatusOK, "Member decision updated successfully", nil, err)
}

// ListReports handles GET /admin/reports?status=&page=&page_size=20
func (h *AdminHandler) ListReports(ctx *gin.Context) {
	status := ctx.Query("status")
	page, pageSize := paging(ctx)
	result, err := h.service.ListReports(ctx.Request.Context(), status, page, pageSize)
	respond(ctx, http.StatusOK, "Reports retrieved successfully", result, err)
}

// ResolveReport handles PATCH /admin/reports/:id
func (h *AdminHandler) ResolveReport(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		respond(ctx, http.StatusBadRequest, "report id is required", nil, nil)
		return
	}
	var in struct {
		ModeratorNote string `json:"moderation_note"`
		Reject        bool   `json:"reject"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	adminID := ctx.GetString("user_id")
	result, err := h.service.ResolveReport(ctx.Request.Context(), id, adminID, in.Reject, in.ModeratorNote)
	respond(ctx, http.StatusOK, "Report resolved successfully", result, err)
}

// CreateSubscription handles POST /admin/subscriptions
func (h *AdminHandler) CreateSubscription(ctx *gin.Context) {
	var in struct {
		UserID       string `json:"user_id"`
		DurationDays int    `json:"duration_days"`
		Status       string `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	if in.DurationDays < 1 || in.DurationDays > 365 {
		respond(ctx, http.StatusBadRequest, "duration_days must be between 1 and 365", nil, nil)
		return
	}
	s := &domainadmin.Subscription{
		UserID:       in.UserID,
		DurationDays: in.DurationDays,
		Status:       in.Status,
	}
	if err := h.service.CreateSubscription(ctx.Request.Context(), s); err != nil {
		respond(ctx, http.StatusInternalServerError, "Failed to create subscription", nil, err)
		return
	}
	respond(ctx, http.StatusOK, "Subscription created successfully", s, nil)
}

// ListSubscriptions handles GET /admin/subscriptions?page=&page_size=20
func (h *AdminHandler) ListSubscriptions(ctx *gin.Context) {
	page, pageSize := paging(ctx)
	result, err := h.service.ListSubscriptions(ctx.Request.Context(), "", page, pageSize)
	respond(ctx, http.StatusOK, "Subscriptions retrieved successfully", result, err)
}

// ExtendSubscription handles POST /admin/subscriptions/:id/extend
func (h *AdminHandler) ExtendSubscription(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		respond(ctx, http.StatusBadRequest, "subscription id is required", nil, nil)
		return
	}
	var in struct {
		DurationDays int `json:"duration_days"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	if in.DurationDays < 1 || in.DurationDays > 365 {
		respond(ctx, http.StatusBadRequest, "duration_days must be between 1 and 365", nil, nil)
		return
	}
	if err := h.service.ExtendSubscription(ctx.Request.Context(), id, in.DurationDays); err != nil {
		respond(ctx, http.StatusInternalServerError, "Failed to extend subscription", nil, err)
		return
	}
	respond(ctx, http.StatusOK, "Subscription extended successfully", nil, nil)
}

// TopupCompanyWallet handles POST /admin/companies/:slug/wallet/top-up
func (h *AdminHandler) TopupCompanyWallet(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		respond(ctx, http.StatusBadRequest, "slug is required", nil, nil)
		return
	}
	var in struct {
		AmountPaise int64 `json:"amount_paise"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		respond(ctx, http.StatusBadRequest, "Invalid request body", nil, err)
		return
	}
	// TODO: Implement company wallet top-up logic
	respond(ctx, http.StatusNotImplemented, "Company wallet top-up not yet implemented", nil, nil)
}

// SearchUsers handles GET /admin/users
func (h *AdminHandler) SearchUsers(ctx *gin.Context) {
	// TODO: Implement user search
	respond(ctx, http.StatusNotImplemented, "User search not yet implemented", nil, nil)
}

// SuspendUser handles PATCH /admin/users/:id
func (h *AdminHandler) SuspendUser(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		respond(ctx, http.StatusBadRequest, "user id is required", nil, nil)
		return
	}
	// TODO: Implement user suspend/unsuspend logic
	respond(ctx, http.StatusNotImplemented, "User suspend not yet implemented", nil, nil)
}

// TriggerLeaveCompany handles POST /admin/users/:id/leave-company
func (h *AdminHandler) TriggerLeaveCompany(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		respond(ctx, http.StatusBadRequest, "user id is required", nil, nil)
		return
	}
	// TODO: Implement self-demotion logic
	respond(ctx, http.StatusNotImplemented, "Leave company not yet implemented", nil, nil)
}

// ListUserCompanies handles GET /admin/users/:id/companies
func (h *AdminHandler) ListUserCompanies(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		respond(ctx, http.StatusBadRequest, "user id is required", nil, nil)
		return
	}
	// TODO: Implement company listing
	respond(ctx, http.StatusNotImplemented, "User companies not yet implemented", nil, nil)
}
