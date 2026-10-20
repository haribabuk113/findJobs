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

// RegisterSettingsRoutes registers settings routes behind the platform-admin guard.
func RegisterSettingsRoutes(rg *gin.RouterGroup, handler *SettingsHandler, requireAdmin gin.HandlerFunc) {
	rg.GET("/settings", requireAdmin, handler.Get)
	rg.PUT("/settings", requireAdmin, handler.Update)
}

// Get handles GET /admin/settings
func (h *SettingsHandler) Get(ctx *gin.Context) {
	result, err := h.service.Get(ctx)
	h.respond(ctx, "Settings retrieved successfully", result, err)
}

// Update handles PUT /admin/settings with the full settings object.
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
