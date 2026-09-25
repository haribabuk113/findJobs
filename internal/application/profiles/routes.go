package profiles

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, ctrl *ProfileController) {

	// Public Routes

	// Protected Routes
	// rg.Use(security.TokenValidator)ß

	rg.GET("/", ctrl.GetProfile)
	rg.PATCH("/update", ctrl.UpdateProfile)

}
