package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const checkTimeout = 2 * time.Second

// Handler is the HTTP layer of the health feature.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires the feature's endpoints onto the given router.
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	r.GET("/health", h.check)
}

type response struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
	Kafka    string `json:"kafka,omitempty"`
}

func (h *Handler) check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), checkTimeout)
	defer cancel()

	report := h.service.Check(ctx)

	status := http.StatusOK
	if !report.Healthy() {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, response{
		Status:   report.Status,
		Database: report.Database,
		Kafka:    report.Kafka,
	})
}
