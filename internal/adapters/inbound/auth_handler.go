package inbound

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"findJobs/internal/adapters"
	appauth "findJobs/internal/application/auth"
	domainauth "findJobs/internal/domain/auth"

	"github.com/gin-gonic/gin"
)

// sessionCookie holds the findJobs session ID. HttpOnly so page scripts
// can't read it; SameSite=Lax so cross-site POSTs don't carry it.
const sessionCookie = "jp_session"

// userIDKey is the gin context key the session middlewares set for signed-in requests.
const userIDKey = "user_id"

// AuthHandler handles HTTP requests for auth and delegates to the application service.
type AuthHandler struct {
	service appauth.AuthService
	// secureCookie is the fallback Secure-flag default, used only when neither a
	// trusted proxy's X-Forwarded-Proto nor a direct TLS connection is available.
	secureCookie bool
	// trustedProxies are the CIDR/IP ranges whose X-Forwarded-Proto header is
	// trusted when deciding the cookie's Secure flag at request time.
	trustedProxies []string
	// platformAdmins holds the IAM ecosystem IDs allowed into /admin routes.
	platformAdmins map[string]bool
	// adminsMu guards platformAdmins against concurrent access: it is written
	// only in NewAuthHandler, but reads run on HTTP handler goroutines, so
	// the lock makes the map safe even if future code mutates it at runtime.
	adminsMu sync.RWMutex
}

// NewAuthHandler creates a new HTTP handler for auth with dependency injection.
// secureCookie is deprecated; the handler now determines the secure flag at runtime.
// trustedProxies are the CIDR/IP ranges (SvelteKit server, Cloudflare) whose
// X-Forwarded-Proto header is trusted for secure cookie determination.
func NewAuthHandler(service appauth.AuthService, secureCookie bool, trustedProxies []string, platformAdminIDs []string) *AuthHandler {
	admins := make(map[string]bool, len(platformAdminIDs))
	for _, id := range platformAdminIDs {
		admins[id] = true
	}
	return &AuthHandler{service: service, secureCookie: secureCookie, trustedProxies: trustedProxies, platformAdmins: admins}
}

// RegisterAuthRoutes registers auth HTTP routes under the provided Gin group.
func RegisterAuthRoutes(rg *gin.RouterGroup, handler *AuthHandler) {
	rg.POST("/session", handler.CreateSession)
	rg.GET("/me", handler.GetCurrentUser)
	rg.GET("/token", handler.AccessToken)
	rg.POST("/logout", handler.Logout)
}

// CreateSession handles POST /auth/session {"code": "<code from IAM redirect>"}
//
//	@Summary	Exchange the IAM login code for a session
//	@ID			createSession
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body	domainauth.SessionRequest	true	"Code from IAM's redirect"
//	@Success	200	{object}	adapters.Response{data=domainauth.SessionResponse}
//	@Header	200	{string}	Set-Cookie	"jp_session=<id>; HttpOnly; SameSite=Lax"
//	@Failure	400,401,415,500	{object}	adapters.Response
//	@Router		/api/v1/auth/session [post]
func (h *AuthHandler) CreateSession(ctx *gin.Context) {
	// JSONBody (router-wide) already rejected non-JSON: blocks login CSRF.
	var request domainauth.SessionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		fail(ctx, http.StatusBadRequest, domainauth.ErrCodeRequired)
		return
	}

	sessionID, result, err := h.service.CreateSession(ctx, &request)
	if err != nil {
		failErr(ctx, err)
		return
	}

	// The cookie lives as long as the session can (its absolute end, IAM's 30
	// days); the idle timeout is enforced server-side on every request.
	h.setCookie(ctx, sessionID, int(time.Until(result.ExpiresAt).Seconds()))
	ctx.JSON(http.StatusOK, adapters.BuildResponse("Session created successfully", result, nil))
}

// GetCurrentUser handles GET /auth/me
//
//	@Summary	Current user for the session
//	@ID			getCurrentUser
//	@Tags		auth
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=domainauth.SessionResponse}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(ctx *gin.Context) {
	h.withSession(ctx, "User retrieved successfully", func(sessionID string) (any, error) {
		result, err := h.service.GetCurrentUser(ctx, sessionID)
		if err == nil {
			result.Authorization.PlatformAdmin = h.platformAdmins[result.User.IAMUserID]
		}
		return result, err
	})
}

// AccessToken handles GET /auth/token: the current IAM access token, for the
// app's own server to call IAM (admin API) as this user. no-store: bearer credential.
//
//	@Summary	Current IAM access token (frontend server only)
//	@ID			getIamAccessToken
//	@Tags		auth
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=domainauth.AccessTokenResponse}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/auth/token [get]
func (h *AuthHandler) AccessToken(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	h.withSession(ctx, "Token retrieved successfully", func(sessionID string) (any, error) {
		return h.service.AccessToken(ctx, sessionID)
	})
}

func (h *AuthHandler) withSession(ctx *gin.Context, message string, fn func(sessionID string) (any, error)) {
	sessionID, err := ctx.Cookie(sessionCookie)
	if err != nil || sessionID == "" {
		fail(ctx, http.StatusUnauthorized, domainauth.ErrUnauthorized)
		return
	}

	result, err := fn(sessionID)
	if err != nil {
		if domainauth.IsUnauthorized(err) {
			h.setCookie(ctx, "", -1)
		}
		failErr(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, adapters.BuildResponse(message, result, nil))
}

// Logout handles POST /auth/logout. Idempotent: no cookie is still a success.
//
//	@Summary	End the session
//	@ID			logout
//	@Tags		auth
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=map[string]string}
//	@Failure	500	{object}	adapters.Response
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(ctx *gin.Context) {
	sessionID, err := ctx.Cookie(sessionCookie)
	if err != nil || sessionID == "" {
		// No valid session cookie; nothing to log out.
	} else if sessionID != "" {
		if err := h.service.Logout(ctx, sessionID); err != nil {
			failErr(ctx, err)
			return
		}
	}
	h.setCookie(ctx, "", -1)

	ctx.JSON(http.StatusOK, adapters.BuildResponse("Logged out successfully", map[string]string{"message": "Logged out successfully"}, nil))
}

// OptionalUser sets the findJobs user ID in the context when the request
// carries a valid session; anonymous requests pass through untouched.
func (h *AuthHandler) OptionalUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionID, err := ctx.Cookie(sessionCookie)
		if err != nil || sessionID == "" {
			// No valid session cookie; leave context untouched.
		} else if sessionID != "" {
			if state, err := h.service.Authenticate(ctx, sessionID); err == nil && state != nil {
				ctx.Set(userIDKey, state.UserID)
			}
		}
		ctx.Next()
	}
}

// RequireUser rejects requests without a valid session (401) and sets the
// findJobs user ID in the context for the handlers behind it.
func (h *AuthHandler) RequireUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if h.requireUser(ctx) != nil {
			ctx.Next()
		}
	}
}

// RequireConsentedUser is RequireUser plus the consent gate: while the user
// hasn't accepted the current legal documents it answers 403 with the list in
// data.required_documents. The consent and account-deletion routes use plain
// RequireUser, so a user can always accept, or leave.
func (h *AuthHandler) RequireConsentedUser(gate func(ctx context.Context, userID string) error) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		state := h.requireUser(ctx)
		if state == nil {
			return
		}
		if err := gate(ctx, state.UserID); err != nil {
			failErr(ctx, err) // ConsentRequiredError: 403 with data.required_documents
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// RequirePlatformAdmin is RequireUser plus a PLATFORM_ADMIN_IDS check (403).
// One middleware, so the admin check can never run without the session check.
func (h *AuthHandler) RequirePlatformAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		state := h.requireUser(ctx)
		if state == nil {
			return
		}
		h.adminsMu.RLock()
		allowed := h.platformAdmins[state.IAMUserID]
		h.adminsMu.RUnlock()
		if !allowed {
			fail(ctx, http.StatusForbidden, fmt.Errorf("not allowed"))
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// requireUser resolves the session or writes the error response and aborts.
func (h *AuthHandler) requireUser(ctx *gin.Context) *domainauth.SessionState {
	sessionID, err := ctx.Cookie(sessionCookie)
	if err != nil || sessionID == "" {
		fail(ctx, http.StatusUnauthorized, domainauth.ErrUnauthorized)
		ctx.Abort()
		return nil
	}
	state, err := h.service.Authenticate(ctx, sessionID)
	if err != nil {
		if domainauth.IsUnauthorized(err) {
			h.setCookie(ctx, "", -1)
		}
		failErr(ctx, err)
		ctx.Abort()
		return nil
	}

	if state.UserID == "" {
		h.setCookie(ctx, "", -1)
		fail(ctx, http.StatusUnauthorized, domainauth.ErrUnauthorized)
		ctx.Abort()
		return nil
	}
	ctx.Set(userIDKey, state.UserID)
	return state
}

func (h *AuthHandler) setCookie(ctx *gin.Context, value string, maxAge int) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(sessionCookie, value, maxAge, "/", "", h.isSecure(ctx), true)
}

// isSecure decides the cookie's Secure attribute at runtime rather than relying
// only on the flag captured at construction. It trusts X-Forwarded-Proto only
// when the request arrived through a configured trusted proxy (defense in
// depth: an untrusted client cannot forge it), otherwise it uses the TLS state
// of the connection, and finally falls back to the configured secureCookie
// default so production stays Secure even if proxy config is missing.
func (h *AuthHandler) isSecure(ctx *gin.Context) bool {
	// Behind a trusted TLS-terminating proxy the original scheme arrives here.
	if h.isFromTrustedProxy(ctx) {
		if proto := ctx.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
			// May be a list (e.g. "https, http"); the first value is the client-facing scheme.
			first := strings.TrimSpace(strings.Split(proto, ",")[0])
			return strings.EqualFold(first, "https")
		}
	}
	// Direct TLS connection (Go terminated it itself).
	if ctx.Request.TLS != nil {
		return true
	}
	// No runtime signal available: keep the configured default (true in prod).
	return h.secureCookie
}

// isFromTrustedProxy reports whether the direct peer (the TCP connection) is
// within the configured trusted proxy CIDRs/IPs. Empty list: no proxies trusted.
func (h *AuthHandler) isFromTrustedProxy(ctx *gin.Context) bool {
	if len(h.trustedProxies) == 0 {
		return false
	}
	peerIP := net.ParseIP(ctx.RemoteIP())
	if peerIP == nil {
		return false
	}
	for _, proxy := range h.trustedProxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		// A CIDR (e.g. 10.0.0.0/8) or a plain IP (e.g. 10.0.0.1).
		if _, ipnet, err := net.ParseCIDR(proxy); err == nil {
			if ipnet.Contains(peerIP) {
				return true
			}
			continue
		}
		if ip := net.ParseIP(proxy); ip != nil && ip.Equal(peerIP) {
			return true
		}
	}
	return false
}
