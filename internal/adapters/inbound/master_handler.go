package inbound

import (
	appMasters "findJobs/internal/application/masters"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and delegates to the application service.
type MastersHandler struct {
	service appMasters.MasterService
}

// NewHandler creates a new HTTP handler with dependency injection.
func NewMastersHandler(service appMasters.MasterService) *MastersHandler {
	return &MastersHandler{service: service}
}

// RegisterRoutes registers Master HTTP routes under the provided Gin group.
func RegisterMastersRoutes(rg *gin.RouterGroup, handler *MastersHandler) {}
