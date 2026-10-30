package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	appsettings "findJobs/internal/application/settings"
	domainsettings "findJobs/internal/domain/settings"

	"github.com/gin-gonic/gin"
)

// SettingsHandler handles the platform-admin settings endpoints.
type SettingsHandler struct {
	service appsettings.SettingsService
}

// NewSettingsHandler creates a new HTTP handler for settings with dependency injection.
func NewSettingsHandler(service appsettings.SettingsService) *SettingsHandler {
	return &SettingsHandler{service: service}
}

// RegisterSettingsRoutes registers the admin settings routes behind the
// platform-admin guard, and the public read-only view on api.
func RegisterSettingsRoutes(api, admin *gin.RouterGroup, handler *SettingsHandler, requireAdmin gin.HandlerFunc) {
	admin.GET("/settings", requireAdmin, handler.Get)
	admin.PUT("/settings", requireAdmin, handler.Update)
	api.GET("/settings", handler.Public)
}

// Public handles GET /settings: the limits and prices candidates and
// recruiters see (no operational values like the FX schedule).
//
//	@Summary	Public limits and prices
//	@ID			getPublicSettings
//	@Tags		settings
//	@Produce	json
//	@Success	200	{object}	adapters.Response{data=domainsettings.PublicSettings}
//	@Failure	500	{object}	adapters.Response
//	@Router		/api/v1/settings [get]
func (h *SettingsHandler) Public(ctx *gin.Context) {
	result, err := h.service.Get(ctx)
	if err != nil {
		h.respond(ctx, "", nil, err)
		return
	}
	h.respond(ctx, "Settings retrieved successfully", result.Public(), nil)
}

// Get handles GET /admin/settings
//
//	@Summary	All platform settings
//	@ID			getSettings
//	@Tags		admin
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=domainsettings.Settings}
//	@Failure	401,403,500	{object}	adapters.Response
//	@Router		/api/v1/admin/settings [get]
func (h *SettingsHandler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx)
	h.respond(ctx, "Settings retrieved successfully", result, err)
}

// Update handles PUT /admin/settings with the full settings object.
//
//	@Summary	Replace the platform settings
//	@ID			updateSettings
//	@Tags		admin
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	domainsettings.Settings	true	"All settings"
//	@Success	200	{object}	adapters.Response{data=domainsettings.Settings}
//	@Failure	400,401,403,500	{object}	adapters.Response
//	@Router		/api/v1/admin/settings [put]
func (h *SettingsHandler) Update(ctx *gin.Context) {
	var values domainsettings.Settings
	if err := ctx.ShouldBindJSON(&values); err != nil {
		h.fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid settings request"))
		return
	}
	result, err := h.service.Update(ctx, &values, ctx.GetString(userIDKey))
	h.respond(ctx, "Settings updated successfully", result, err)
}

func (h *SettingsHandler) respond(ctx *gin.Context, message string, result any, err error) {
	ctx.Header("Cache-Control", "no-store")
	switch {
	case err == nil:
		ctx.JSON(http.StatusOK, adapters.BuildResponse(message, result, nil))
	case domainsettings.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	default:
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *SettingsHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
