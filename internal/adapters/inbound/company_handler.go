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
	c.PUT("/:slug/logo", requireUser, h.Logo)
	c.PUT("/:slug/banner", requireUser, h.Banner)
	c.GET("/:slug/dashboard", requireUser, h.Dashboard)
	c.GET("/:slug/members", requireUser, h.Members)
	c.POST("/:slug/members/:user_id/approve", requireUser, h.Approve)
	c.POST("/:slug/members/:user_id/reject", requireUser, h.Reject)
	c.PATCH("/:slug/members/:user_id", requireUser, h.SetRole)
	c.POST("/:slug/members/:user_id/offboard", requireUser, h.Offboard)
	c.POST("/:slug/leave", requireUser, h.Leave)

	api.GET("/files/img/:name", h.File)
}

type slugAvailability struct {
	Available bool `json:"available"`
}

type roleRequest struct {
	Role string `json:"role"`
}

// Search handles POST /companies/search.
//
//	@Summary	Search companies
//	@ID			searchCompanies
//	@Tags		companies
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domaincompanies.SearchCriteria	true	"Filters"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.SearchResult}
//	@Failure	400,500	{object}	adapters.Response
//	@Router		/api/v1/companies/search [post]
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
//
//	@Summary	Is a company slug free?
//	@ID			checkCompanySlug
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	query	string	true	"Slug to check"
//	@Success	200	{object}	adapters.Response{data=slugAvailability}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/companies/slug-availability [get]
func (h *CompanyHandler) SlugAvailability(ctx *gin.Context) {
	available, err := h.service.SlugAvailable(ctx, ctx.Query("slug"))
	h.respond(ctx, http.StatusOK, "Slug checked", slugAvailability{Available: available}, err)
}

// Create handles POST /companies.
//
//	@Summary	Create a company
//	@ID			createCompany
//	@Tags		companies
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	domaincompanies.Input	true	"The company"
//	@Success	201	{object}	adapters.Response{data=domaincompanies.Company}
//	@Failure	400,401,409,500	{object}	adapters.Response
//	@Router		/api/v1/companies [post]
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
//
//	@Summary	Company page (session optional)
//	@ID			getCompany
//	@Tags		companies
//	@Produce	json
//	@Param		slug	path	string	true	"Company slug"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.Company}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug} [get]
func (h *CompanyHandler) Get(ctx *gin.Context) {
	company, err := h.service.Get(ctx, ctx.Param("slug"), ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Company retrieved successfully", company, err)
}

// Update handles PUT /companies/:slug with the full document.
//
//	@Summary	Update the whole company page
//	@ID			updateCompany
//	@Tags		companies
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		body	body	domaincompanies.Input	true	"The company"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.Company}
//	@Failure	400,401,403,404,409,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug} [put]
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
//
//	@Summary	Delete the company
//	@ID			deleteCompany
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug} [delete]
func (h *CompanyHandler) Delete(ctx *gin.Context) {
	err := h.service.Delete(ctx, ctx.GetString(userIDKey), ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "Company deleted successfully", nil, err)
}

// Logo handles PUT /companies/:slug/logo.
//
//	@Summary	Upload the company logo
//	@ID			setCompanyLogo
//	@Tags		companies
//	@Accept		image/jpeg,image/png,image/webp
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		body	body	string	true	"Raw image bytes"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.Company}
//	@Failure	400,401,403,404,413,415,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/logo [put]
func (h *CompanyHandler) Logo(ctx *gin.Context) { h.image("logo")(ctx) }

// Banner handles PUT /companies/:slug/banner.
//
//	@Summary	Upload the company banner
//	@ID			setCompanyBanner
//	@Tags		companies
//	@Accept		image/jpeg,image/png,image/webp
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		body	body	string	true	"Raw image bytes"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.Company}
//	@Failure	400,401,403,404,413,415,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/banner [put]
func (h *CompanyHandler) Banner(ctx *gin.Context) { h.image("banner")(ctx) }

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
//
//	@Summary	An uploaded image (profile photo, logo, banner)
//	@ID			getImage
//	@Tags		companies
//	@Produce	image/jpeg,image/png,image/webp
//	@Param		name	path	string	true	"File name"
//	@Success	200	{file}	file
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/files/img/{name} [get]
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
//
//	@Summary	Company admin dashboard counts
//	@ID			getCompanyDashboard
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Success	200	{object}	adapters.Response{data=domaincompanies.Dashboard}
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/dashboard [get]
func (h *CompanyHandler) Dashboard(ctx *gin.Context) {
	result, err := h.service.Dashboard(ctx, ctx.GetString(userIDKey), ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "Dashboard retrieved successfully", result, err)
}

// Members handles GET /companies/:slug/members?status=pending|current|past|rejected
//
//	@Summary	Company members and membership requests
//	@ID			getCompanyMembers
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		status	query	string	false	"pending, current (default), past or rejected"
//	@Success	200	{object}	adapters.Response{data=[]domaincompanies.Member}
//	@Failure	400,401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/members [get]
func (h *CompanyHandler) Members(ctx *gin.Context) {
	status := ctx.DefaultQuery("status", domaincompanies.StatusCurrent)
	result, err := h.service.Members(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), status)
	h.respond(ctx, http.StatusOK, "Members retrieved successfully", result, err)
}

// Approve handles POST /companies/:slug/members/:user_id/approve.
//
//	@Summary	Approve (verify) a membership request
//	@ID			approveMember
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		user_id	path	string	true	"Member user id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/members/{user_id}/approve [post]
func (h *CompanyHandler) Approve(ctx *gin.Context) { h.decide(true)(ctx) }

// Reject handles POST /companies/:slug/members/:user_id/reject.
//
//	@Summary	Reject a membership request
//	@ID			rejectMember
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		user_id	path	string	true	"Member user id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/members/{user_id}/reject [post]
func (h *CompanyHandler) Reject(ctx *gin.Context) { h.decide(false)(ctx) }

// decide handles POST /companies/:slug/members/:user_id/{approve,reject}.
func (h *CompanyHandler) decide(approve bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := h.service.DecideMember(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"), approve)
		h.respond(ctx, http.StatusOK, "Request updated successfully", nil, err)
	}
}

// SetRole handles PATCH /companies/:slug/members/:user_id {"role": "admin"|"hr"|"employee"}
//
//	@Summary	Change a member's role
//	@ID			setMemberRole
//	@Tags		companies
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		user_id	path	string	true	"Member user id"
//	@Param		body	body	roleRequest	true	"admin, hr or employee"
//	@Success	200	{object}	adapters.Response
//	@Failure	400,401,403,404,409,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/members/{user_id} [patch]
func (h *CompanyHandler) SetRole(ctx *gin.Context) {
	var body roleRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid role request"))
		return
	}
	err := h.service.SetMemberRole(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"), body.Role)
	h.respond(ctx, http.StatusOK, "Role updated successfully", nil, err)
}

// Offboard handles POST /companies/:slug/members/:user_id/offboard.
//
//	@Summary	Move a member to past employees
//	@ID			offboardMember
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Param		user_id	path	string	true	"Member user id"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,403,404,409,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/members/{user_id}/offboard [post]
func (h *CompanyHandler) Offboard(ctx *gin.Context) {
	err := h.service.Offboard(ctx, ctx.GetString(userIDKey), ctx.Param("slug"), ctx.Param("user_id"))
	h.respond(ctx, http.StatusOK, "Member moved to past employees", nil, err)
}

// Leave handles POST /companies/:slug/leave.
//
//	@Summary	Leave the company
//	@ID			leaveCompany
//	@Tags		companies
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	path	string	true	"Company slug"
//	@Success	200	{object}	adapters.Response
//	@Failure	401,404,409,500	{object}	adapters.Response
//	@Router		/api/v1/companies/{slug}/leave [post]
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
