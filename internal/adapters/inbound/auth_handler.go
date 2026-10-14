package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	appauth "findJobs/internal/application/auth"
	domainauth "findJobs/internal/domain/auth"

	"github.com/gin-gonic/gin"
)

// sessionCookie holds the findJobs session ID. HttpOnly so page scripts
// can't read it; SameSite=Lax so cross-site POSTs don't carry it.
const sessionCookie = "jp_session"

// AuthHandler handles HTTP requests for auth and delegates to the application service.
type AuthHandler struct {
	service      appauth.AuthService
	secureCookie bool
}

// NewAuthHandler creates a new HTTP handler for auth with dependency injection.
// secureCookie should be true everywhere except plain-HTTP local dev.
func NewAuthHandler(service appauth.AuthService, secureCookie bool) *AuthHandler {
	return &AuthHandler{service: service, secureCookie: secureCookie}
}

// RegisterAuthRoutes registers auth HTTP routes under the provided Gin group.
func RegisterAuthRoutes(rg *gin.RouterGroup, handler *AuthHandler) {
	rg.POST("/session", handler.CreateSession)
	rg.GET("/me", handler.GetCurrentUser)
	rg.GET("/token", handler.AccessToken)
	rg.POST("/logout", handler.Logout)
}

// CreateSession handles POST /auth/session {"code": "<code from IAM redirect>"}
func (h *AuthHandler) CreateSession(ctx *gin.Context) {
	// JSON-only: an HTML form can't send application/json cross-site without
	// a CORS preflight, which blocks login CSRF via an attacker's own code.
	if ctx.ContentType() != "application/json" {
		h.fail(ctx, http.StatusUnsupportedMediaType, fmt.Errorf("content type must be application/json"))
		return
	}
	var request domainauth.SessionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		h.fail(ctx, http.StatusBadRequest, domainauth.ErrCodeRequired)
		return
	}

	sessionID, result, err := h.service.CreateSession(ctx, &request)
	if err != nil {
		h.failErr(ctx, err)
		return
	}

	h.setCookie(ctx, sessionID, 30*24*3600)
	ctx.JSON(http.StatusOK, adapters.BuildResponse("Session created successfully", result, nil))
}

// GetCurrentUser handles GET /auth/me
func (h *AuthHandler) GetCurrentUser(ctx *gin.Context) {
	h.withSession(ctx, "User retrieved successfully", func(sessionID string) (any, error) {
		return h.service.GetCurrentUser(ctx, sessionID)
	})
}

// AccessToken handles GET /auth/token: the current IAM access token, for the
// app's own server to call IAM (admin API) as this user. no-store: bearer credential.
func (h *AuthHandler) AccessToken(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	h.withSession(ctx, "Token retrieved successfully", func(sessionID string) (any, error) {
		return h.service.AccessToken(ctx, sessionID)
	})
}

func (h *AuthHandler) withSession(ctx *gin.Context, message string, fn func(sessionID string) (any, error)) {
	sessionID, _ := ctx.Cookie(sessionCookie)
	if sessionID == "" {
		h.fail(ctx, http.StatusUnauthorized, domainauth.ErrUnauthorized)
		return
	}

	result, err := fn(sessionID)
	if err != nil {
		if domainauth.IsUnauthorized(err) {
			h.setCookie(ctx, "", -1)
		}
		h.failErr(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse(message, result, nil))
}

// Logout handles POST /auth/logout. Idempotent: no cookie is still a success.
func (h *AuthHandler) Logout(ctx *gin.Context) {
	sessionID, _ := ctx.Cookie(sessionCookie)
	h.setCookie(ctx, "", -1)
	if sessionID != "" {
		if err := h.service.Logout(ctx, sessionID); err != nil {
			h.failErr(ctx, err)
			return
		}
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Logged out successfully", map[string]string{"message": "Logged out successfully"}, nil))
}

func (h *AuthHandler) setCookie(ctx *gin.Context, value string, maxAge int) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(sessionCookie, value, maxAge, "/", "", h.secureCookie, true)
}

func (h *AuthHandler) failErr(ctx *gin.Context, err error) {
	switch {
	case domainauth.IsValidation(err):
		h.fail(ctx, http.StatusBadRequest, err)
	case domainauth.IsUnauthorized(err):
		h.fail(ctx, http.StatusUnauthorized, err)
	case domainauth.IsNotFound(err):
		h.fail(ctx, http.StatusNotFound, err)
	default:
		// Never leak internals (DB/IAM errors) to the client.
		ctx.Error(err)
		h.fail(ctx, http.StatusInternalServerError, fmt.Errorf("internal error"))
	}
}

func (h *AuthHandler) fail(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, err))
}
