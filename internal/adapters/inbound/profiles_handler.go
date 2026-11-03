package inbound

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	appprofiles "findJobs/internal/application/profiles"
	domainprofiles "findJobs/internal/domain/profiles"

	"github.com/gin-gonic/gin"
)

// ProfileHandler handles the candidate profile's HTTP requests.
type ProfileHandler struct {
	service   appprofiles.ProfileService
	analytics appprofiles.AnalyticsService
}

// NewProfileHandler creates the HTTP handler for profiles.
func NewProfileHandler(service appprofiles.ProfileService, analytics appprofiles.AnalyticsService) *ProfileHandler {
	return &ProfileHandler{service: service, analytics: analytics}
}

// RegisterProfileRoutes registers profile routes on api (the /api/v1 group).
// Everything under /profile/me is the signed-in user's own profile.
func RegisterProfileRoutes(api *gin.RouterGroup, h *ProfileHandler, optionalUser, requireUser gin.HandlerFunc) {
	me := api.Group("/profile/me", requireUser)
	me.GET("", h.Mine)
	me.PUT("", h.Save)
	me.GET("/slug-availability", h.SlugAvailability)
	me.PUT("/photo", h.Photo)
	me.DELETE("/photo", h.DeletePhoto)
	me.POST("/experiences/:index/verification", h.RequestVerification)
	me.GET("/skills", h.Skills)
	me.POST("/skills", h.AddSkill)
	me.DELETE("/skills/:id", h.RemoveSkill)

	// The optional session lets a view by the profile's owner go uncounted.
	api.GET("/profiles/:slug", optionalUser, h.Public)
	api.GET("/profiles/:slug/analytics", h.Analytics)
}

type skillRequest struct {
	SkillID int64 `json:"skill_id"`
}

// Skills handles GET /profile/me/skills?signature=...: only the signature
// comes back when it matches, the full list otherwise.
//
//	@Summary	Sync the client's cached skill list
//	@ID			syncMySkills
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Param		signature	query	string	false	"Signature of the cached list; only the signature returns when it matches"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.SkillSet}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/skills [get]
func (h *ProfileHandler) Skills(ctx *gin.Context) {
	set, err := h.service.Skills(ctx, ctx.GetString(userIDKey), ctx.Query("signature"))
	respond(ctx, http.StatusOK, "Skills retrieved successfully", set, err)
}

// AddSkill handles POST /profile/me/skills with {"skill_id": n}.
//
//	@Summary	One-click add of an existing skill
//	@ID			addMySkill
//	@Tags		profiles
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	skillRequest	true	"Skill to add"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.SkillSet}
//	@Failure	400,401,404,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/skills [post]
func (h *ProfileHandler) AddSkill(ctx *gin.Context) {
	var in skillRequest
	if err := ctx.ShouldBindJSON(&in); err != nil || in.SkillID <= 0 {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("give a skill_id"))
		return
	}
	set, err := h.service.AddSkill(ctx, ctx.GetString(userIDKey), in.SkillID)
	respond(ctx, http.StatusOK, "Skill added", set, err)
}

// RemoveSkill handles DELETE /profile/me/skills/:id.
//
//	@Summary	Remove a skill from the profile
//	@ID			removeMySkill
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Param		id	path	int	true	"Skill term id"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.SkillSet}
//	@Failure	401,404,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/skills/{id} [delete]
func (h *ProfileHandler) RemoveSkill(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(ctx, http.StatusNotFound, fmt.Errorf("skill not found"))
		return
	}
	set, err := h.service.RemoveSkill(ctx, ctx.GetString(userIDKey), id)
	respond(ctx, http.StatusOK, "Skill removed", set, err)
}

// Mine handles GET /profile/me (a starter with saved=false before the first save).
//
//	@Summary	The signed-in user's full profile
//	@ID			getMyProfile
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	401,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me [get]
func (h *ProfileHandler) Mine(ctx *gin.Context) {
	p, err := h.service.Mine(ctx, ctx.GetString(userIDKey))
	respond(ctx, http.StatusOK, "Profile retrieved successfully", p, err)
}

// SlugAvailability handles GET /profile/me/slug-availability?slug=: whether a
// profile handle is free (the user's own counts as free). 400 for a bad or
// reserved handle.
//
//	@Summary	Is a profile handle free?
//	@ID			checkProfileSlug
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Param		slug	query	string	true	"Handle to check"
//	@Success	200	{object}	adapters.Response{data=slugAvailability}
//	@Failure	400,401,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/slug-availability [get]
func (h *ProfileHandler) SlugAvailability(ctx *gin.Context) {
	available, err := h.service.SlugAvailable(ctx, ctx.GetString(userIDKey), ctx.Query("slug"))
	respond(ctx, http.StatusOK, "Slug checked", slugAvailability{Available: available}, err)
}

// Save handles PUT /profile/me: the whole profile document.
//
//	@Summary	Save the whole profile
//	@ID			saveMyProfile
//	@Tags		profiles
//	@Accept		json
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	domainprofiles.Profile	true	"The whole profile document"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	400,401,409,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me [put]
func (h *ProfileHandler) Save(ctx *gin.Context) {
	var in domainprofiles.Profile
	if err := ctx.ShouldBindJSON(&in); err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("invalid profile"))
		return
	}
	p, err := h.service.Save(ctx, ctx.GetString(userIDKey), &in)
	respond(ctx, http.StatusOK, "Profile saved successfully", p, err)
}

// Photo handles PUT /profile/me/photo with a raw image body.
//
//	@Summary	Upload the profile photo
//	@ID			setMyPhoto
//	@Tags		profiles
//	@Accept		image/jpeg,image/png,image/webp
//	@Produce	json
//	@Security	sessionCookie
//	@Param		body	body	string	true	"Raw image bytes"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	400,401,413,415,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/photo [put]
func (h *ProfileHandler) Photo(ctx *gin.Context) {
	data, err := io.ReadAll(ctx.Request.Body)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || len(data) > appprofiles.MaxPhotoBytes {
		fail(ctx, http.StatusRequestEntityTooLarge, fmt.Errorf("the photo is too large"))
		return
	}
	if err != nil {
		fail(ctx, http.StatusBadRequest, fmt.Errorf("could not read the upload"))
		return
	}
	p, err := h.service.SetPhoto(ctx, ctx.GetString(userIDKey), ctx.ContentType(), data)
	respond(ctx, http.StatusOK, "Photo updated successfully", p, err)
}

// DeletePhoto handles DELETE /profile/me/photo. Idempotent: without a photo it returns the profile unchanged.
//
//	@Summary	Remove the profile photo
//	@ID			deleteMyPhoto
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	401,404,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/photo [delete]
func (h *ProfileHandler) DeletePhoto(ctx *gin.Context) {
	p, err := h.service.DeletePhoto(ctx, ctx.GetString(userIDKey))
	respond(ctx, http.StatusOK, "Photo removed successfully", p, err)
}

// RequestVerification handles POST /profile/me/experiences/:index/verification.
//
//	@Summary	Ask the experience's company to verify it
//	@ID			requestExperienceVerification
//	@Tags		profiles
//	@Produce	json
//	@Security	sessionCookie
//	@Param		index	path	int	true	"Experience position, from 0"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	400,401,404,500	{object}	adapters.Response
//	@Router		/api/v1/profile/me/experiences/{index}/verification [post]
func (h *ProfileHandler) RequestVerification(ctx *gin.Context) {
	index, err := strconv.Atoi(ctx.Param("index"))
	if err != nil || index < 0 || index >= domainprofiles.MaxExperiences {
		fail(ctx, http.StatusNotFound, fmt.Errorf("experience not found"))
		return
	}
	p, err := h.service.RequestVerification(ctx, ctx.GetString(userIDKey), index)
	respond(ctx, http.StatusOK, "Verification requested", p, err)
}

// Public handles GET /profiles/:slug?src=search|direct (404 when missing or
// private). Each successful load counts as a profile view (non-unique;
// bots and the owner aren't counted). src=search marks views from the
// platform's search and discovery pages; anything else counts as direct.
//
//	@Summary	Public profile (session optional)
//	@ID			getPublicProfile
//	@Tags		profiles
//	@Produce	json
//	@Param		slug	path	string	true	"Profile slug"
//	@Param		src	query	string	false	"search marks a view from search; anything else counts as direct"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Profile}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/profiles/{slug} [get]
func (h *ProfileHandler) Public(ctx *gin.Context) {
	p, err := h.service.Public(ctx, ctx.Param("slug"))
	if err == nil {
		h.analytics.RecordView(ctx, p.OwnerID, ctx.GetString(userIDKey), ctx.Query("src"), ctx.GetHeader("User-Agent"))
	}
	respond(ctx, http.StatusOK, "Profile retrieved successfully", p, err)
}

// Analytics handles GET /profiles/:slug/analytics: the public Activity &
// Analytics tab, aggregates only (404 when missing or private).
//
//	@Summary	Profile Activity & Analytics
//	@ID			getProfileAnalytics
//	@Tags		profiles
//	@Produce	json
//	@Param		slug	path	string	true	"Profile slug"
//	@Success	200	{object}	adapters.Response{data=domainprofiles.Analytics}
//	@Failure	404,500	{object}	adapters.Response
//	@Router		/api/v1/profiles/{slug}/analytics [get]
func (h *ProfileHandler) Analytics(ctx *gin.Context) {
	a, err := h.analytics.Analytics(ctx, ctx.Param("slug"))
	respond(ctx, http.StatusOK, "Profile analytics retrieved successfully", a, err)
}
