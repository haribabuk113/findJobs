package inbound

import (
	"fmt"
	"net/http"
	"strconv"

	"findJobs/internal/adapters"
	appreference "findJobs/internal/application/reference"
	domainreference "findJobs/internal/domain/reference"

	"github.com/gin-gonic/gin"
)

// ReferenceHandler serves the published reference files and the site-admin
// taxonomy and exchange-rate endpoints.
type ReferenceHandler struct {
	service appreference.ReferenceService
}

// NewReferenceHandler creates a new HTTP handler for reference data with dependency injection.
func NewReferenceHandler(service appreference.ReferenceService) *ReferenceHandler {
	return &ReferenceHandler{service: service}
}

// RegisterReferenceRoutes registers public asset routes on assets and the
// admin routes (behind requireAdmin) on admin.
func RegisterReferenceRoutes(assets, admin *gin.RouterGroup, handler *ReferenceHandler, requireAdmin gin.HandlerFunc) {
	assets.GET("/manifest", handler.Manifest)
	assets.GET("/:file", handler.Asset)

	admin.GET("/taxonomy/:kind", requireAdmin, handler.ListTerms)
	admin.POST("/taxonomy/:kind/merge", requireAdmin, handler.MergeTerms)
	admin.PATCH("/taxonomy/:kind/:id", requireAdmin, handler.UpdateTerm)
	admin.DELETE("/taxonomy/:kind/:id", requireAdmin, handler.DeleteTerm)
	admin.POST("/fx/refresh", requireAdmin, handler.RefreshRates)
}

// Manifest handles GET /assets/manifest: which file is current per kind.
// Public and identical for everyone, so shared caches may hold it briefly.
func (h *ReferenceHandler) Manifest(ctx *gin.Context) {
	manifest, err := h.service.Manifest(ctx)
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=60")
	ctx.JSON(http.StatusOK, adapters.BuildResponse("Manifest retrieved successfully", manifest, nil))
}

// Asset handles GET /assets/:file. File names carry a timestamp and content
// hash, so the bytes behind a name never change: cache forever.
func (h *ReferenceHandler) Asset(ctx *gin.Context) {
	asset, err := h.service.Asset(ctx, ctx.Param("file"))
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Header("ETag", `"`+asset.SHA256+`"`)
	ctx.Data(http.StatusOK, "application/json; charset=utf-8", asset.Content)
}

// ListTerms handles GET /admin/taxonomy/:kind?status=&page=&page_size=
func (h *ReferenceHandler) ListTerms(ctx *gin.Context) {
	kind, err := domainreference.ParseKind(ctx.Param("kind"))
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	page, err1 := optionalInt(ctx.Query("page"))
	pageSize, err2 := optionalInt(ctx.Query("page_size"))
	if err1 != nil || err2 != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("page and page_size must be whole numbers"))
		return
	}
	result, err := h.service.ListTerms(ctx, kind, ctx.Query("status"), page, pageSize)
	h.respond(ctx, "Terms retrieved successfully", result, err)
}

// MergeTerms handles POST /admin/taxonomy/:kind/merge {"source_ids": [..], "master_id": n}
func (h *ReferenceHandler) MergeTerms(ctx *gin.Context) {
	kind, err := domainreference.ParseKind(ctx.Param("kind"))
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	var body struct {
		SourceIDs []int64 `json:"source_ids"`
		MasterID  int64   `json:"master_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid merge request"))
		return
	}
	err = h.service.MergeTerms(ctx, kind, body.SourceIDs, body.MasterID)
	h.respond(ctx, "Terms merged successfully", nil, err)
}

// UpdateTerm handles PATCH /admin/taxonomy/:kind/:id {"name"?: "", "status"?: "verified"|"pending"}
func (h *ReferenceHandler) UpdateTerm(ctx *gin.Context) {
	kind, id, ok := h.kindAndID(ctx)
	if !ok {
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid update request"))
		return
	}
	term, err := h.service.UpdateTerm(ctx, kind, id, body.Name, body.Status)
	h.respond(ctx, "Term updated successfully", term, err)
}

// DeleteTerm handles DELETE /admin/taxonomy/:kind/:id
func (h *ReferenceHandler) DeleteTerm(ctx *gin.Context) {
	kind, id, ok := h.kindAndID(ctx)
	if !ok {
		return
	}
	h.respond(ctx, "Term deleted successfully", nil, h.service.DeleteTerm(ctx, kind, id))
}

// RefreshRates handles POST /admin/fx/refresh: starts a refresh in the
// background (provider retries can outlast the request) and returns 202.
func (h *ReferenceHandler) RefreshRates(ctx *gin.Context) {
	if !h.service.RefreshRatesAsync() {
		h.fail(ctx, http.StatusConflict, fmt.Errorf("a refresh is already running"))
		return
	}
	ctx.JSON(http.StatusAccepted, adapters.BuildResponse("Exchange-rate refresh started; the manifest's rates entry updates when it finishes", nil, nil))
}

func (h *ReferenceHandler) kindAndID(ctx *gin.Context) (domainreference.Kind, int64, bool) {
	kind, err := domainreference.ParseKind(ctx.Param("kind"))
	if err != nil {
		h.failErr(ctx, err)
		return "", 0, false
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("id must be a positive whole number"))
		return "", 0, false
	}
	return kind, id, true
}

func optionalInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

func (h *ReferenceHandler) respond(ctx *gin.Context, message string, result any, err error) {
	if err != nil {
		h.failErr(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adapters.BuildResponse(message, result, nil))
}

func (h *ReferenceHandler) failErr(ctx *gin.Context, err error) {
	switch {
	case domainreference.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	case domainreference.IsNotFound(err):
		h.fail(ctx, http.StatusNotFound, err)
	case domainreference.IsConflict(err):
		h.fail(ctx, http.StatusConflict, err)
	default:
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *ReferenceHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
