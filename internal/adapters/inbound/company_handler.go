package inbound

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"findJobs/internal/adapters"
	"findJobs/internal/adapters/outbound/storage"
	appcompanies "findJobs/internal/application/companies"
	domaincompanies "findJobs/internal/domain/companies"

	"github.com/gin-gonic/gin"
)

// CompanyHandler handles the company module's HTTP requests.
type CompanyHandler struct {
	service appcompanies.CompanyService
}

// NewCompanyHandler creates a new HTTP handler for companies with dependency injection.
func NewCompanyHandler(service appcompanies.CompanyService) *CompanyHandler {
	return &CompanyHandler{service: service}
}

// RegisterCompanyRoutes registers company routes on api (the /api/v1 group).
func RegisterCompanyRoutes(api *gin.RouterGroup, h *CompanyHandler, optionalUser, requireUser gin.HandlerFunc) {
	c := api.Group("/companies")
	c.POST("/search", h.Search)
	c.GET("/slug-availability", requireUser, h.SlugAvailability)
	c.POST("", requireUser, h.Create)
	c.GET("/:slug", optionalUser, h.Get)
	c.PUT("/:slug", requireUser, h.Update)
	c.DELETE("/:slug", requireUser, h.Delete)
	c.PUT("/:slug/logo", requireUser, h.image("logo"))
	c.PUT("/:slug/banner", requireUser, h.image("banner"))
	c.GET("/:slug/dashboard", requireUser, h.Dashboard)
	c.GET("/:slug/members", requireUser, h.Members)
	c.POST("/:slug/members/:user_id/approve", requireUser, h.decide(true))
	c.POST("/:slug/members/:user_id/reject", requireUser, h.decide(false))
	c.PATCH("/:slug/members/:user_id", requireUser, h.SetRole)
	c.POST("/:slug/members/:user_id/offboard", requireUser, h.Offboard)
	c.POST("/:slug/leave", requireUser, h.Leave)

	api.GET("/files/img/:name", h.File)
}

// Search handles POST /companies/search.
func (h *CompanyHandler) Search(ctx *gin.Context) {
	var criteria domaincompanies.SearchCriteria
	if err := ctx.ShouldBindJSON(&criteria); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid search request"))
		return
	}
	result, err := h.service.Search(ctx, &criteria)
	h.respond(ctx, http.StatusOK, "Companies retrieved successfully", result, err)
}

// SlugAvailability handles GET /companies/slug-availability?slug=
func (h *CompanyHandler) SlugAvailability(ctx *gin.Context) {
	available, err := h.service.SlugAvailable(ctx, ctx.Query("slug"))
	h.respond(ctx, http.StatusOK, "Slug checked", gin.H{"available": available}, err)
}

// Create handles POST /companies.
func (h *CompanyHandler) Create(ctx *gin.Context) {
	var in domaincompanies.Input
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid company"))
		return
	}
	company, err := h.service.Create(ctx, ctx.GetString(userIDKey), &in)
	h.respond(ctx, http.StatusCreated, "Company created successfully", company, err)
}

// Get handles GET /companies/:slug (public page).
func (h *CompanyHandler) Get(ctx *gin.Context) {
	company, err := h.service.Get(ctx, ctx.Param("slug"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Company retrieved successfully", company, err)
}

// Update handles PUT /companies/:slug with the full document.
func (h *CompanyHandler) Update(ctx *gin.Context) {
	var in domaincompanies.Input
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid company"))
		return
	}
	company, err := h.service.Update(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), &in)
	h.respond(ctx, http.StatusOK, "Company updated successfully", company, err)
}

// Delete handles DELETE /companies/:slug.
func (h *CompanyHandler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx, ctx.GetString(userIDKey), ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "Company deleted successfully", nil, err)
}

// image handles PUT /companies/:slug/{logo,banner} with the raw image as the body.
func (h *CompanyHandler) image(kind string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data, err := io.ReadAll(ctx.Request.Body)
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.fail(ctx, http.StatusRequestEntityTooLarge, fmt.Errorf("the image is too large"))
			return
		}
		if err != nil {
			h.fail(ctx, http.StatusBadRequest, fmt.Errorf("could not read the upload"))
			return
		}
		company, err := h.service.SetImage(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), kind, ctx.ContentType(), data)
		h.respond(ctx, http.StatusOK, "Image updated successfully", company, err)
	}
}

// File handles GET /files/img/:name: an uploaded public image. Names are
// random and never reused, so the bytes behind a URL never change.
func (h *CompanyHandler) File(ctx *gin.Context) {
	data, contentType, err := h.service.File(ctx, "img/"+ctx.Param("name"))
	if errors.Is(err, storage.ErrNotFound) {
		h.fail(ctx, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	// Even if a crafted file got through, it can't run script in our origin.
	ctx.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; sandbox")
	ctx.Data(http.StatusOK, contentType, data)
}

// Dashboard handles GET /companies/:slug/dashboard.
func (h *CompanyHandler) Dashboard(ctx *gin.Context) {
	result, err := h.service.Dashboard(ctx, ctx.GetString(userIDKey), ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "Dashboard retrieved successfully", result, err)
}

// Members handles GET /companies/:slug/members?status=pending|current|past|rejected
func (h *CompanyHandler) Members(ctx *gin.Context) {
	status := ctx.DefaultQuery("status", domaincompanies.StatusCurrent)
	result, err := h.service.Members(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), status)
	h.respond(ctx, http.StatusOK, "Members retrieved successfully", result, err)
}

func (h *CompanyHandler) decide(approve bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := h.service.DecideMember(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"), approve)
		h.respond(ctx, http.StatusOK, "Request updated successfully", nil, err)
	}
}

// SetRole handles PATCH /companies/:slug/members/:user_id {"role": "admin"|"hr"|"employee"}
func (h *CompanyHandler) SetRole(ctx *gin.Context) {
	var body struct {
		Role string `json:"role"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid role request"))
		return
	}
	err := h.service.SetMemberRole(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"), body.Role)
	h.respond(ctx, http.StatusOK, "Role updated successfully", nil, err)
}

// Offboard handles POST /companies/:slug/members/:user_id/offboard.
func (h *CompanyHandler) Offboard(ctx *gin.Context) {
	err := h.service.Offboard(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"))
	h.respond(ctx, http.StatusOK, "Member moved to past employees", nil, err)
}

// Leave handles POST /companies/:slug/leave.
func (h *CompanyHandler) Leave(ctx *gin.Context) {
	err := h.service.Leave(ctx, ctx.GetString(userIDKey), ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "You have left the company", nil, err)
}

func (h *CompanyHandler) respond(ctx *gin.Context, status int, message string, result any, err error) {
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	// Responses can depend on the viewer (viewer_role, admin data): never share-cache.
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(status, adapters.BuildResponse(message, result, nil))
}

func (h *CompanyHandler) failErr(ctx *gin.Context, err error) {
	switch {
	case domaincompanies.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	case domaincompanies.IsNotFound(err):
		h.fail(ctx, http.StatusNotFound, err)
	case domaincompanies.IsForbidden(err):
		h.fail(ctx, http.StatusForbidden, err)
	case domaincompanies.IsConflict(err):
		h.fail(ctx, http.StatusConflict, err)
	default:
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *CompanyHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
