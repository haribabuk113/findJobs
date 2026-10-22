package inbound

import (
	"errors"
	"findJobs/internal/adapters"
	appMasters "findJobs/internal/application/masters"
	models "findJobs/internal/domain/master"
	domainProfiles "findJobs/internal/domain/profiles"
	apperror "findJobs/internal/utils/errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MastersHandler handles HTTP requests and delegates to the application service.
type MastersHandler struct {
	service appMasters.MasterService
}

// NewMastersHandler creates a new HTTP handler with dependency injection.
func NewMastersHandler(service appMasters.MasterService) *MastersHandler {
	return &MastersHandler{service: service}
}

// RegisterRoutes registers Master HTTP routes under the provided Gin group.
func RegisterMastersRoutes(rg *gin.RouterGroup, handler *MastersHandler) {
	// rg.GET("/", handler.GetProfile)
	// rg.PATCH("/update", handler.UpdateProfile)

	// // ---- company admin
	// company := rg.Group("/company") //, middleware.Require(auth, middleware.RoleCompanyAdmin)
	// company.POST("/timeline/:id/verify", handler.Timeline.Review)

	// // ---- service-to-service
	// internal := rg.Group("/internal") // , middleware.Require(auth, middleware.RoleInternal)
	// internal.POST("/resume-grants", handler.Resume.CreateGrant)

	// ---- platform admin
	admin := rg.Group("/masters") //, middleware.Require(auth, middleware.RoleAdmin)
	{

		admin.POST("/:kind", handler.CreateMaster)        //y
		admin.GET("/:kind", handler.SearchMaster)         //y
		admin.GET("/:kind/pending", handler.ListPending)  //y
		admin.POST("/:kind/:id/approve", handler.Approve) //y
		admin.PUT("/:kind/:id", handler.Rename)           //y
		admin.DELETE("/:kind/:id", handler.Delete)        //y

		admin.GET("/companies", handler.SearchCompany)
		admin.POST("/companies", handler.CreateCompany)       //y
		admin.PUT("/companies/:id", handler.UpdateCompany)    //y
		admin.DELETE("/companies/:id", handler.DeleteCompany) //y

		admin.GET("/locations", handler.ListLocation)
		admin.POST("/locations", handler.CreateLocation)       //y
		admin.PUT("/locations/:id", handler.UpdateLocation)    //y
		admin.DELETE("/locations/:id", handler.DeleteLocation) //y

		admin.GET("/fx-rates", handler.GetFx)
		admin.PUT("/fx-rates", handler.PutFx)
	}
}

// // GetProfile handles GET /profiles/:id
// func (h *MastersHandler) GetProfile(c *gin.Context) {
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
// func (h *MastersHandler) UpdateProfile(c *gin.Context) {
// 	ctx := c.Request.Context()

// 	var request domainProfiles.ProfileRequest
// 	if err := c.ShouldBindJSON(&request); err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	result, err := h.service.UpdateProfile(ctx, &request)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
// 			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
// 		return
// 	}

// 	c.JSON(http.StatusOK, adapters.BuildResponse("Message", result, nil))
// }

// ---------------------------------------------------------------- designations / skills (:kind)
//
// GET /masters/:kind?q=fig&limit=20  (public typeahead, approved only)
func (h *MastersHandler) SearchMaster(c *gin.Context) {
	// kind, ok := masterKind(c)
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}

	var req models.MasterListQuery

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.SearchMaster(c.Request.Context(), kind, req.Q, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

// POST /masters/:kind  (candidate types a custom value -> pending review)
func (h *MastersHandler) CreateMaster(c *gin.Context) {
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}
	var req models.MasterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.CreateMaster(c.Request.Context(), kind, candidateID(c), req)

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

func (h *MastersHandler) ListPending(c *gin.Context) {
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}
	out, err := h.service.ListPending(c.Request.Context(), kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *MastersHandler) Approve(c *gin.Context) {
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}

	err = h.service.Approve(c.Request.Context(), kind, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *MastersHandler) Rename(c *gin.Context) {
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	var req models.MasterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.Rename(c.Request.Context(), kind, id, req)
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

func (h *MastersHandler) Delete(c *gin.Context) {
	kind, ok := models.ParseMasterKind(c.Param("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidMasterType))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	err = h.service.Delete(c.Request.Context(), kind, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- companies
func (h *MastersHandler) SearchCompany(c *gin.Context) {
	var q models.CompanySearchQuery

	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	out, err := h.service.SearchCompany(c.Request.Context(), q.Q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *MastersHandler) CreateCompany(c *gin.Context) {
	var req models.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.CreateCompany(c.Request.Context(), req)
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

func (h *MastersHandler) UpdateCompany(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	var req models.CompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateCompany(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

func (h *MastersHandler) DeleteCompany(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	err = h.service.DeleteCompany(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- locations
// / GET /locations?level=CITY&parent_id=12&q=beng
func (h *MastersHandler) ListLocation(c *gin.Context) {
	var q models.LocationListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.ListLocation(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", out, nil))
}

func (h *MastersHandler) CreateLocation(c *gin.Context) {
	var req models.LocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.CreateLocation(c.Request.Context(), req)
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

func (h *MastersHandler) UpdateLocation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	var req models.LocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err = h.service.UpdateLocation(c.Request.Context(), id, req)
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

func (h *MastersHandler) DeleteLocation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil,
			domainProfiles.ErrInvalidID))
		return
	}
	err = h.service.DeleteLocation(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}

// ---------------------------------------------------------------- fx rates
//
// GET /fx-rates[?date=YYYY-MM-DD]  (latest per currency when no date)
func (h *MastersHandler) GetFx(c *gin.Context) {
	var q models.FxQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	out, err := h.service.GetFx(c.Request.Context(), q.Date)
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

// PUT /admin/fx-rates  (daily job pushes the rates.json contents)
func (h *MastersHandler) PutFx(c *gin.Context) {
	var req models.PutFxRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}
	err := h.service.PutFx(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse(
			fmt.Sprintf("%d %s", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)), nil, err))
		return
	}

	c.JSON(http.StatusOK, adapters.BuildResponse("Message", "", nil))
}
