package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	appauth "findJobs/internal/application/auth"
	domainauth "findJobs/internal/domain/auth"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for auth and delegates to the application service.
type AuthHandler struct {
	service appauth.AuthService
}

// NewAuthHandler creates a new HTTP handler for auth with dependency injection.
func NewAuthHandler(service appauth.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// RegisterAuthRoutes registers auth HTTP routes under the provided Gin group.
func RegisterAuthRoutes(rg *gin.RouterGroup, handler *AuthHandler) {
	rg.POST("/session", handler.CreateSession)
	rg.GET("/me", handler.GetCurrentUser)
	rg.POST("/logout", handler.Logout)
}

// CreateSession handles POST /auth/session
func (h *AuthHandler) CreateSession(ctx *gin.Context) {
	var request domainauth.SessionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	// Extract IAM user ID from JWT token (from Authorization header)
	// In a real implementation, this would be parsed from the JWT claims
	// For now, we'll extract it from a header or use a placeholder
	iamUserID := ctx.GetHeader("X-IAM-User-ID")
	if iamUserID == "" {
		// Try to get from JWT token in Authorization header
		authHeader := ctx.GetHeader("Authorization")
		if authHeader != "" && len(authHeader) > 7 {
			// Extract token and parse claims to get user ID
			// For now, use a placeholder
			iamUserID = "iam_123"
		}
	}

	if iamUserID == "" {
		ctx.JSON(http.StatusUnauthorized, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)), nil, domainauth.ErrInvalidIAMToken))
		return
	}

	result, err := h.service.CreateSession(ctx, &request, iamUserID)
	if err != nil {
		// Check error type and return appropriate status code
		if domainauth.IsValidation(err) {
			ctx.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
			return
		}
		if domainauth.IsNotFound(err) {
			ctx.JSON(http.StatusNotFound, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", http.StatusNotFound, http.StatusText(http.StatusNotFound)), nil, err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Session created successfully", result, nil))
}

// GetCurrentUser handles GET /auth/me
func (h *AuthHandler) GetCurrentUser(ctx *gin.Context) {
	// In a real implementation, user ID would come from JWT claims
	userID := ctx.GetHeader("X-User-ID")
	if userID == "" {
		userID = "usr_123" // Placeholder
	}

	result, err := h.service.GetCurrentUser(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("User retrieved successfully", result, nil))
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(ctx *gin.Context) {
	userID := ctx.GetHeader("X-User-ID")
	if userID == "" {
		userID = "usr_123" // Placeholder
	}

	err := h.service.Logout(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Logged out successfully", map[string]string{"message": "Logged out successfully"}, nil))
}
