package profiles

import (
	resonse "findJobs/internal/adapters"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProfileController struct {
	profileService ProfileService
}

func NewProfileController(profileService ProfileService) *ProfileController {
	return &ProfileController{profileService: profileService}
}

func (h *ProfileController) GetProfile(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Param("id")
	var userId int64 = -1
	var err error
	if id != "" {
		userId, err = strconv.ParseInt(id, 10, 64)
	}
	if err != nil {
		// log.ErrorContext(ctx, "Error Parsing int to string", "error", err)
		c.JSON(http.StatusBadRequest, resonse.BuildErrorResponse(fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
	}

	result, err := h.profileService.GetProfile(ctx, userId)

	if err != nil {
		// log.ErrorContext(ctx, "Error Getting User", "error", err)
		c.JSON(http.StatusBadRequest, resonse.BuildErrorResponse(fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, resonse.BuildResponse("Message", result, nil))
}

func (h *ProfileController) UpdateProfile(c *gin.Context) {
	ctx := c.Request.Context()

	var request ProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		// log.ErrorContext(ctx, "Error Binding Json", "error", err)
		c.JSON(http.StatusBadRequest, resonse.BuildErrorResponse(fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	// role := c.GetString("role")
	// if role != string(config.Admin) {
	// 	userId, _ := c.Get("userId")
	// 	request.ID = userId.(int64)
	// }

	result, err := h.profileService.UpdateProfile(ctx, &request)
	if err != nil {
		// log.ErrorContext(ctx, "Error Updating User", "error", err)
		c.JSON(http.StatusBadRequest, resonse.BuildErrorResponse(fmt.Sprintf("%d %s", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)), nil, err))
		return
	}

	c.JSON(http.StatusOK, resonse.BuildResponse("Message", result, nil))
}
