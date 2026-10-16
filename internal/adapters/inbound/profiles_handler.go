package inbound

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"findJobs/internal/adapters"
	appProfiles "findJobs/internal/application/profiles"
	domainProfiles "findJobs/internal/domain/profiles"
	utilProfiles "findJobs/internal/utils/profiles"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler handles HTTP requests and delegates to the application service.
type Handler struct {
	service appProfiles.ProfileService
}

// NewHandler creates a new HTTP handler with dependency injection.
func NewHandler(service appProfiles.ProfileService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers profile HTTP routes under the provided Gin group.
func RegisterRoutes(rg *gin.RouterGroup, handler *Handler) {
	rg.GET("/", handler.GetProfile)
	rg.PATCH("/update", handler.UpdateProfile)

	rg.GET("/profile/:value", handler.GetCandidateProfile)
	rg.PATCH("/profile", handler.UpdateCandidateProfile)

	// Profile Photo
	rg.GET("/photo/:id", handler.UpdateProfile)
	rg.POST("/photo", handler.UpdateProfile)
	// rg.PATCH("/photo", handler.UpdateProfile)

	// // Profile Slug
	// rg.GET("/slug/:id", handler.GetProfileSlug)
	// rg.POST("/slug/:id", handler.CreateProfileSlug)
	// rg.PATCH("/slug/:id", handler.UpdateProfileSlug)

}

// GetProfile handles GET /profiles/:id
func (h *Handler) GetProfile(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	result, err := h.service.GetProfile(ctx, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// UpdateProfile handles PUT /profiles/:id
func (h *Handler) UpdateProfile(c *gin.Context) {
	ctx := c.Request.Context()

	var request domainProfiles.ProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	result, err := h.service.UpdateProfile(ctx, &request)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

func (h *Handler) GetCandidateProfile(c *gin.Context) {
	ctx := c.Request.Context()
	var err error
	var result domainProfiles.ProfileDetails

	value := c.Param("value")
	userID, err := strconv.ParseInt(value, 10, 64)
	if err == nil && userID > 0 {
		result, err = h.service.GetCandidateProfile(ctx, userID, "")
	}

	result, err = h.service.GetCandidateProfile(ctx, -1, value)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

func (h *Handler) UpdateCandidateProfile(c *gin.Context) {
	ctx := c.Request.Context()

	var request domainProfiles.ProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	result, err := h.service.UpdateCandidateProfile(ctx, &request)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// GetProfilePhoto handles GET /photo/:id
func (h *Handler) GetProfilePhoto(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	result, err := h.service.GetProfilePhoto(ctx, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// UpdateProfilePhoto handles POST /photo/:id
func (h *Handler) CreateProfilePhoto(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	// 1. Cap the request body so nobody can stream gigabytes at you
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, utilProfiles.MaxPhotoSize+(1<<20))

	// 2. Read the "photo" field from the multipart form
	fh, err := c.FormFile("photo")
	if err != nil {
		err := fmt.Errorf("Photo file is required")
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	if fh.Size > utilProfiles.MaxPhotoSize {
		err := fmt.Errorf("photo must be 5MB or smaller")
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	file, err := fh.Open()
	if err != nil {
		err := fmt.Errorf("Could not read file")
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	defer file.Close()

	// 3. Detect the real type from the bytes, never trust the filename or header
	head := make([]byte, 512)
	numbytes, _ := io.ReadFull(file, head)
	contentType := http.DetectContentType(head[:numbytes])
	ext, ok := utilProfiles.AllowedTypes[contentType]
	if !ok {
		err := fmt.Errorf("Only JPEG, PNG or WebP allowed")
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil { // rewind after sniffing
		err := fmt.Errorf("Could not read file")
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	// 4. Server-generated name: avoids path traversal and overwrites
	key := fmt.Sprintf("photos/%s%s", uuid.NewString(), ext)

	result, err := h.service.CreateProfilePhoto(ctx, key, file, contentType, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// // UpdateProfilePhoto handles PUT /photo
// func (h *Handler) UpdateProfilePhoto(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	id := c.Param("id")
// 	userID, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil || userID <= 0 {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	result, err := h.service.UpdateProfilePhoto(ctx, userID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// GetProfileSlug handles GET /slug/:id
func (h *Handler) GetProfileSlug(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 { // !utilProfiles.SlugPattern.MatchString(slug)
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	result, err := h.service.GetProfileSlug(ctx, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// CreateProfileSlug handles POST /slug/:id
func (h *Handler) CreateProfileSlug(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidProfileID))
		return
	}

	result, err := h.service.CreateProfileSlug(ctx, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
}

// // UpdateProfileSlug handles PUT /slug/:id
// func (h *Handler) UpdateProfileSlug(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	slug := c.Param("slug")
// 	if !utilProfiles.SlugPattern.MatchString(slug) {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
// 			domainProfiles.ErrInvalidProfileID))
// 		return
// 	}

// 	result, err := h.service.UpdateProfileSlug(ctx, slug)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }
