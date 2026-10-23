package inbound

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"findJobs/internal/adapters"
	appprofiles "findJobs/internal/application/profiles"
	domaincompanies "findJobs/internal/domain/companies"
	domainprofiles "findJobs/internal/domain/profiles"
	domainreference "findJobs/internal/domain/reference"

	"github.com/gin-gonic/gin"
)

// ProfileHandler handles the candidate profile's HTTP requests.
type ProfileHandler struct {
	service appprofiles.ProfileService
}

// NewProfileHandler creates the HTTP handler for profiles.
func NewProfileHandler(service appprofiles.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// RegisterProfileRoutes registers profile routes on api (the /api/v1 group).
// Everything under /profile/me is the signed-in user's own profile.
func RegisterProfileRoutes(api *gin.RouterGroup, h *ProfileHandler, requireUser gin.HandlerFunc) {
	me := api.Group("/profile/me", requireUser)
	me.GET("", h.Mine)
	me.PUT("", h.Save)
	me.PUT("/photo", h.Photo)
	me.POST("/experiences/:index/verification", h.RequestVerification)

	api.GET("/profiles/:slug", h.Public)
}

// Mine handles GET /profile/me (a starter with saved=false before the first save).
func (h *ProfileHandler) Mine(ctx *gin.Context) {
	p, err := h.service.Mine(ctx, ctx.GetString(userIDKey))
	h.respond(ctx, http.StatusOK, "Profile retrieved successfully", p, err)
}

// Save handles PUT /profile/me: the whole profile document.
func (h *ProfileHandler) Save(ctx *gin.Context) {
	var in domainprofiles.Profile
	if err := ctx.ShouldBindJSON(&in); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid profile"))
		return
	}
	p, err := h.service.Save(ctx, ctx.GetString(userIDKey), &in)
	h.respond(ctx, http.StatusOK, "Profile saved successfully", p, err)
}

// Photo handles PUT /profile/me/photo with a raw image body.
func (h *ProfileHandler) Photo(ctx *gin.Context) {
	data, err := io.ReadAll(ctx.Request.Body)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || len(data) > appprofiles.MaxPhotoBytes {
		h.fail(ctx, http.StatusRequestEntityTooLarge, fmt.Errorf("the photo is too large"))
		return
	}
	if err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("could not read the upload"))
		return
	}
	p, err := h.service.SetPhoto(ctx, ctx.GetString(userIDKey), ctx.ContentType(), data)
	h.respond(ctx, http.StatusOK, "Photo updated successfully", p, err)
}

// RequestVerification handles POST /profile/me/experiences/:index/verification.
func (h *ProfileHandler) RequestVerification(ctx *gin.Context) {
	index, err := strconv.Atoi(ctx.Param("index"))
	if err != nil || index < 0 || index >= domainprofiles.MaxExperiences {
		h.fail(ctx, http.StatusNotFound, fmt.Errorf("experience not found"))
		return
	}
	p, err := h.service.RequestVerification(ctx, ctx.GetString(userIDKey), index)
	h.respond(ctx, http.StatusOK, "Verification requested", p, err)
}

// Public handles GET /profiles/:slug (404 when missing or private).
func (h *ProfileHandler) Public(ctx *gin.Context) {
	p, err := h.service.Public(ctx, ctx.Param("slug"))
	h.respond(ctx, http.StatusOK, "Profile retrieved successfully", p, err)
}

func (h *ProfileHandler) respond(ctx *gin.Context, status int, message string, result any, err error) {
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	// Owner data, or a page whose privacy can change any moment: never share-cache.
	ctx.Header("Cache-Control", "private, no-store")
	ctx.JSON(status, adapters.BuildResponse(message, result, nil))
}

func (h *ProfileHandler) failErr(ctx *gin.Context, err error) {
	switch {
	case domainprofiles.IsValidation(err), domainreference.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	case domainprofiles.IsNotFound(err), domaincompanies.IsNotFound(err):
		h.fail(ctx, http.StatusNotFound, err)
	case domainprofiles.IsConflict(err):
		h.fail(ctx, http.StatusConflict, err)
	default:
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *ProfileHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
