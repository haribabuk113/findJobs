package inbound

import (
	"fmt"
	"net/http"
	"strconv"

	"findJobs/internal/adapters"
	appprofiles "findJobs/internal/application/profiles"
	domainprofiles "findJobs/internal/domain/profiles"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and delegates to the application service.
type Handler struct {
	service appprofiles.ProfileService
}

// NewHandler creates a new HTTP handler with dependency injection.
func NewHandler(service appprofiles.ProfileService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers profile HTTP routes under the provided Gin group.
func RegisterRoutes(rg *gin.RouterGroup, handler *Handler) {
	rg.GET("/", handler.GetProfile)
	rg.PATCH("/update", handler.UpdateProfile)
}

// GetProfile handles GET /profiles/:id
func (h *Handler) GetProfile(ctx *gin.Context) {
	id := ctx.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainprofiles.ErrInvalidProfileID))
		return
	}

	result, err := h.service.GetProfile(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// UpdateProfile handles PUT /profiles/:id
func (h *Handler) UpdateProfile(ctx *gin.Context) {
	var request domainprofiles.ProfileRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	result, err := h.service.UpdateProfile(ctx, &request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}
