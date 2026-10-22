package inbound

import (
	"errors"
	"findJobs/internal/adapters"
	appProfiles "findJobs/internal/application/profiles"
	"fmt"
	"net/http"
	"strconv"

	domainProfiles "findJobs/internal/domain/profiles"
	models "findJobs/internal/domain/profiles"
	apperror "findJobs/internal/utils/errors"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and delegates to the application service.
type ProfileHandler struct {
	service appProfiles.ProfileService
}

// NewHandler creates a new HTTP handler with dependency injection.
func NewHandler(service appProfiles.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// RegisterRoutes registers profile HTTP routes under the provided Gin group.
func RegisterRoutes(rg *gin.RouterGroup, handler *ProfileHandler) {
	// rg.GET("/", handler.GetProfile)
	// rg.PATCH("/update", handler.UpdateProfile)

	// rg.GET("/profile/:value", handler.GetCandidateProfile)
	// rg.PATCH("/profile", handler.UpdateCandidateProfile)

	// // Profile Photo
	// rg.GET("/photo/:id", handler.UpdateProfile)
	// rg.POST("/photo", handler.UpdateProfile)
	// rg.PATCH("/photo", handler.UpdateProfile)

	// // Profile Slug
	// rg.GET("/slug/:id", handler.GetProfileSlug)
	// rg.POST("/slug/:id", handler.CreateProfileSlug)
	// rg.PATCH("/slug/:id", handler.UpdateProfileSlug)
	//
	//
	//
	//
	// ---- public
	rg.GET("/:slug", handler.Get)                                              //y
	rg.POST("/:slug/social-handles/:platform/click", handler.TrackClickSocial) //y
	// rg.GET("/masters/:kind", handler.Masterg.Search)
	// rg.GET("/companies", handler.Company.Search)
	// rg.GET("/locations", handler.Location.List)
	// rg.GET("/fx-rates", handler.Fx.Get)

	// ---- authenticated candidate: everything is scoped to the caller's own candidate_id
	me := rg.Group("/me") //, middleware.Require(auth, middleware.RoleCandidate)
	{
		me.DELETE("", handler.Delete)        //y
		me.PATCH("/profile", handler.Update) //y

		me.GET("/pronouns", handler.GetPronouns) //y
		me.PUT("/pronouns", handler.PutPronouns) //y

		me.GET("/social-handles", handler.ListSocial)          //y
		me.POST("/social-handles", handler.CreateSocial)       //y
		me.PUT("/social-handles/:id", handler.UpdateSocial)    //y
		me.DELETE("/social-handles/:id", handler.DeleteSocial) //y

		me.GET("/salary", handler.GetSalary)       //y
		me.PUT("/salary", handler.PutSalary)       //y
		me.DELETE("/salary", handler.DeleteSalary) //y

		me.GET("/preferences", handler.GetPreference)           //y
		me.PUT("/preferences", handler.PutPreference)           //y
		me.PUT("/target-roles", handler.PutTargetRoles)         //y
		me.PUT("/target-locations", handler.PutTargetLocations) //y

		me.GET("/timeline", handler.ListTimeline)                                          //y
		me.POST("/timeline", handler.CreateTimeline)                                       //y
		me.PUT("/timeline/:id", handler.UpdateTimeline)                                    //y
		me.DELETE("/timeline/:id", handler.DeleteTimeline)                                 //y
		me.POST("/timeline/:id/request-verification", handler.RequestTimelineVerification) //y

		me.GET("/skill-groups", handler.ListSkillGroup)                     //y
		me.POST("/skill-groups", handler.CreateSkillGroup)                  //y
		me.PUT("/skill-groups/:id", handler.UpdateSkillGroup)               //y
		me.DELETE("/skill-groups/:id", handler.DeleteSkillGroup)            //y
		me.POST("/skill-groups/:id/skills", handler.AddSkill)               //y
		me.PUT("/skill-groups/:id/skills/:skillId", handler.UpdateSkill)    //y
		me.DELETE("/skill-groups/:id/skills/:skillId", handler.RemoveSkill) //y

		me.GET("/contact", handler.GetContact) //y
		me.PUT("/contact", handler.PutContact) //y

		me.GET("/resume", handler.GetResume)                       //y
		me.PUT("/resume", handler.PutResume)                       //y
		me.DELETE("/resume", handler.DeleteResume)                 //y
		me.GET("/resume-grants", handler.ListResumeGrants)         //y
		me.DELETE("/resume-grants/:id", handler.RevokeResumeGrant) //y

		// me.GET("/work-authorization", handler.WorkAuth.List)          //y
		// me.POST("/work-authorization", handler.WorkAuth.Create)       //y
		// me.PUT("/work-authorization/:id", handler.WorkAuth.Update)    //y
		// me.DELETE("/work-authorization/:id", handler.WorkAuth.Delete) //y

		// me.POST("/masters/:kind", handler.Master.Create) // custom entry -> pending review
	}

}

// // GetProfile handles GET /profiles/:id
// func (h *Handler) GetProfile(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	id := c.Param("id")
// 	userID, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil || userID <= 0 {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	result, err := h.service.GetProfile(ctx, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // UpdateProfile handles PUT /profiles/:id
// func (h *Handler) UpdateProfile(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	var request domainProfiles.ProfileRequest
// if err := c.ShouldBindJSON(&request); err != nil {
// 	c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 	return
// }

// 	result, err := h.service.UpdateProfile(ctx, &request)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// func (h *Handler) GetCandidateProfile(c *gin.Context) {
// 	ctx := c.Request.Context()
// 	var err error
// 	var result domainProfiles.ProfileDetails

// 	value := c.Param("value")
// 	userID, err := strconv.ParseInt(value, 10, 64)
// 	if err == nil && userID > 0 {
// 		result, err = h.service.GetCandidateProfile(ctx, userID, "")
// 	}

// 	result, err = h.service.GetCandidateProfile(ctx, -1, value)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// func (h *Handler) UpdateCandidateProfile(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	var request domainProfiles.ProfileRequest
// 	if err := c.ShouldBindJSON(&request); err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	result, err := h.service.UpdateCandidateProfile(ctx, &request)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // GetProfilePhoto handles GET /photo/:id
// func (h *Handler) GetProfilePhoto(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	id := c.Param("id")
// 	userID, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil || userID <= 0 {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	result, err := h.service.GetProfilePhoto(ctx, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // UpdateProfilePhoto handles POST /photo/:id
// func (h *Handler) CreateProfilePhoto(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	id := c.Param("id")
// 	userID, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil || userID <= 0 {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	// 1. Cap the request body so nobody can stream gigabytes at you
// 	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, utilProfiles.MaxPhotoSize+(1<<20))

// 	// 2. Read the "photo" field from the multipart form
// 	fh, err := c.FormFile("photo")
// 	if err != nil {
// 		err := fmt.Errorf("Photo file is required")
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}
// 	if fh.Size > utilProfiles.MaxPhotoSize {
// 		err := fmt.Errorf("photo must be 5MB or smaller")
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	file, err := fh.Open()
// 	if err != nil {
// 		err := fmt.Errorf("Could not read file")
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}
// 	defer file.Close()

// 	// 3. Detect the real type from the bytes, never trust the filename or header
// 	head := make([]byte, 512)
// 	numbytes, _ := io.ReadFull(file, head)
// 	contentType := http.DetectContentType(head[:numbytes])
// 	ext, ok := utilProfiles.AllowedTypes[contentType]
// 	if !ok {
// 		err := fmt.Errorf("Only JPEG, PNG or WebP allowed")
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}
// 	if _, err := file.Seek(0, io.SeekStart); err != nil { // rewind after sniffing
// 		err := fmt.Errorf("Could not read file")
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	// 4. Server-generated name: avoids path traversal and overwrites
// 	key := fmt.Sprintf("photos/%s%s", uuid.NewString(), ext)

// 	result, err := h.service.CreateProfilePhoto(ctx, key, file, contentType, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // // UpdateProfilePhoto handles PUT /photo
// // func (h *Handler) UpdateProfilePhoto(c *gin.Context) {
// // 	ctx := c.Request.Context()

// // 	id := c.Param("id")
// // 	userID, err := strconv.ParseInt(id, 10, 64)
// // 	if err != nil || userID <= 0 {
// // 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// // 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// // 			domainProfiles.ErrInvalidProfileID))
// // 		return
// // 	}

// // 	result, err := h.service.UpdateProfilePhoto(ctx, userID)
// // 	if err != nil {
// // 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// // 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// // 		return
// // 	}

// // 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// // }

// // GetProfileSlug handles GET /slug/:id
// func (h *Handler) GetProfileSlug(c *gin.Context) {
// 	ctx := c.Request.Context()

// id := c.Param("id")
// userID, err := strconv.ParseInt(id, 10, 64)
// if err != nil || userID <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
// 	c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 		domainProfiles.ErrInvalidProfileID))
// 	return
// }

// 	result, err := h.service.GetProfileSlug(ctx, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // CreateProfileSlug handles POST /slug/:id
// func (h *Handler) CreateProfileSlug(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	id := c.Param("id")
// 	userID, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil || userID <= 0 {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	result, err := h.service.CreateProfileSlug(ctx, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// // // UpdateProfileSlug handles PUT /slug/:id
// // func (h *Handler) UpdateProfileSlug(c *gin.Context) {
// // 	ctx := c.Request.Context()

// // 	slug := c.Param("slug")
// // 	if !utilProfiles.SlugPattern.MatchString(slug) {
// // 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// // 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// // 			domainProfiles.ErrInvalidProfileID))
// // 		return
// // 	}

// result, err := h.service.UpdateProfileSlug(ctx, slug)
// if err != nil {
// 	c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 	return
// }

// c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// // }

// GET /profile/:slug  (public)
func (h *ProfileHandler) Get(c *gin.Context) {
	p, err := h.service.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", p, nil))
}

// PATCH /me/profile
func (h *ProfileHandler) Update(c *gin.Context) {
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.Update(c.Request.Context(), candidateID(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// DELETE /me
func (h *ProfileHandler) Delete(c *gin.Context) {
	err := h.service.Delete(c.Request.Context(), candidateID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// GET /me/pronouns
func (h *ProfileHandler) GetPronouns(c *gin.Context) {
	res, err := h.service.GetPronouns(c.Request.Context(), candidateID(c))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", res, nil))
}

// PUT /me/pronouns
func (h *ProfileHandler) PutPronouns(c *gin.Context) {
	var req models.PronounsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.ReplacePronouns(c.Request.Context(), candidateID(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) ListSocial(c *gin.Context) {
	out, err := h.service.ListSocial(c.Request.Context(), candidateID(c))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) CreateSocial(c *gin.Context) {
	var req models.SocialHandleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	id, err := h.service.CreateSocial(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusCreated, models.IDResponse{ID: id})
}

func (h *ProfileHandler) UpdateSocial(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	var req models.SocialHandleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateSocial(c.Request.Context(), candidateID(c), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) DeleteSocial(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	err = h.service.DeleteSocial(c.Request.Context(), candidateID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// POST /profile/:slug/social-handles/:platform/click  (public telemetry)
func (h *ProfileHandler) TrackClickSocial(c *gin.Context) {
	err := h.service.TrackClickSocial(c.Request.Context(), c.Param("slug"), c.Param("platform"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) GetSalary(c *gin.Context) {
	out, err := h.service.GetSalary(c.Request.Context(), candidateID(c))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) PutSalary(c *gin.Context) {
	var req models.SalaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutSalary(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) DeleteSalary(c *gin.Context) {
	err := h.service.DeleteSalary(c.Request.Context(), candidateID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) GetPreference(c *gin.Context) {
	out, err := h.service.GetPreference(c.Request.Context(), candidateID(c))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

// PUT /me/preferences  (open_to_remote + work_types)
func (h *ProfileHandler) PutPreference(c *gin.Context) {
	var req models.PreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutPreference(c.Request.Context(), candidateID(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) PutTargetRoles(c *gin.Context) {
	var req models.TargetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutTargetRoles(c.Request.Context(), candidateID(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) PutTargetLocations(c *gin.Context) {
	var req models.TargetLocationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutTargetLocations(c.Request.Context(), candidateID(c), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- timeline

func (h *ProfileHandler) ListTimeline(c *gin.Context) {
	out, err := h.service.ListTimeline(c.Request.Context(), candidateID(c))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) CreateTimeline(c *gin.Context) {
	var req models.TimelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	id, err := h.service.CreateTimeline(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusCreated, models.TimelineIDResponse{TimelineID: id})
}

func (h *ProfileHandler) UpdateTimeline(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	var req models.TimelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateTimeline(c.Request.Context(), candidateID(c), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) DeleteTimeline(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	err = h.service.DeleteTimeline(c.Request.Context(), candidateID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) RequestTimelineVerification(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	err = h.service.RequestTimelineVerification(c.Request.Context(), candidateID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// POST /company/timeline/:id/verify  (company admin)
func (h *ProfileHandler) ReviewTimeline(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	var req models.VerificationReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.ReviewTimeline(c.Request.Context(), id, req.Approve, userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- skill groups

func (h *ProfileHandler) ListSkillGroup(c *gin.Context) {
	out, err := h.service.ListSkillGroup(c.Request.Context(), candidateID(c))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) CreateSkillGroup(c *gin.Context) {
	var req models.SkillGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	id, err := h.service.CreateSkillGroup(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusCreated, models.SkillGroupIDResponse{GroupID: id})
}

func (h *ProfileHandler) UpdateSkillGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	var req models.SkillGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateSkillGroup(c.Request.Context(), candidateID(c), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) DeleteSkillGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	err = h.service.DeleteSkillGroup(c.Request.Context(), candidateID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) AddSkill(c *gin.Context) {

	gid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gid <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}

	var req models.AddSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.AddSkill(c.Request.Context(), candidateID(c), gid, req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *ProfileHandler) UpdateSkill(c *gin.Context) {

	gid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gid <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}

	sid, err := strconv.ParseInt(c.Param("skillId"), 10, 64)
	if err != nil || gid <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}

	var req models.UpdateSkillItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateSkill(c.Request.Context(), candidateID(c), gid, sid, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) RemoveSkill(c *gin.Context) {
	gid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || gid <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}

	sid, err := strconv.ParseInt(c.Param("skillId"), 10, 64)
	if err != nil || gid <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	err = h.service.RemoveSkill(c.Request.Context(), candidateID(c), gid, sid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- contact
func (h *ProfileHandler) GetContact(c *gin.Context) {
	out, err := h.service.GetContact(c.Request.Context(), candidateID(c))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) PutContact(c *gin.Context) {
	var req models.ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutContact(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- resume + grants
//

func (h *ProfileHandler) GetResume(c *gin.Context) {
	out, err := h.service.GetResume(c.Request.Context(), candidateID(c))
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) PutResume(c *gin.Context) {
	var req models.ResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutResume(c.Request.Context(), candidateID(c), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) DeleteResume(c *gin.Context) {
	err := h.service.DeleteResume(c.Request.Context(), candidateID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *ProfileHandler) ListResumeGrants(c *gin.Context) {
	out, err := h.service.ListResumeGrants(c.Request.Context(), candidateID(c))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}
	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *ProfileHandler) RevokeResumeGrant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}
	err = h.service.RevokeResumeGrant(c.Request.Context(), candidateID(c), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// POST /internal/resume-grants  (service-to-service)
func (h *ProfileHandler) CreateResumeGrant(c *gin.Context) {
	var req models.CreateGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.CreateResumeGrant(c.Request.Context(), req)
	if err != nil {
		var appErr *apperror.Error

		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(appErr.Status, adapters.BuildErrorResponse(
				fmt.Sprintf("%d %s", appErr.Status, http.StatusText(appErr.Status)), nil, err))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- work authorization

// func (h *ProfileHandler) ListWorkAuthorization(c *gin.Context) {
// 	out, err := h.service.ListWorkAuthorization(c.Request.Context(), candidateID(c))
// 	if err != nil {
// 		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
// 		return
// 	}
// 		c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
// }

// func (h *ProfileHandler) CreateWorkAuthorization(c *gin.Context) {
// 	var req models.WorkAuthorizationRequest
// 	if !bindJSON(c, &req) {
// 		return
// 	}
// 	id, err := h.service.CreateWorkAuthorization(c.Request.Context(), candidateID(c), req)
// 	if err != nil {
// 		c.AbortWithStatusJSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
// 		return
// 	}
// 	c.JSON(http.StatusCreated, dto.IDResponse{ID: id})
// }

// func (h *ProfileHandler) UpdateWorkAuthorization(c *gin.Context) {

// id, err := strconv.ParseInt(c.Param("id"), 10, 64)
// if err != nil || gid <= 0 {
// 	c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 		domainProfiles.ErrInvalidID))
// 	return
// }

// 	var req models.WorkAuthorizationRequest
// 	if !bindJSON(c, &req) {
// 		return
// 	}
// err := h.service.UpdateWorkAuthorization(c.Request.Context(), candidateID(c), id, req)
// if err != nil {
// 	c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
// 	return
// }

// c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
// }

// func (h *ProfileHandler) DeleteWorkAuthorization(c *gin.Context) {
// 	id, ok := pathID(c, "id")
// 	if !ok {
// 		return
// 	}
// 	if err != nil || gid <= 0 {
// 	c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 		domainProfiles.ErrInvalidID))
// 	return
// }
// err :=  h.service.DeleteWorkAuthorization(c.Request.Context(), candidateID(c), id)
// if err != nil {
// 	c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
// 		fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
// 	return
// }

// c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
// }
