package payment

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// QueryHandler serves the read side: what the UI needs to show state.
type QueryHandler struct {
	payments *Repository
}

func NewQueryHandler(payments *Repository) *QueryHandler {
	return &QueryHandler{payments: payments}
}

func (h *QueryHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/payments", h.list)
	r.GET("/payments/summary", h.summary)
	r.GET("/payments/:id", h.detail)
}

func (h *QueryHandler) list(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		limit = 50
	}

	items, err := h.payments.List(c.Request.Context(), limit, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list payments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": items})
}

func (h *QueryHandler) summary(c *gin.Context) {
	counts, err := h.payments.Counts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not summarize payments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"counts": counts})
}

func (h *QueryHandler) detail(c *gin.Context) {
	d, err := h.payments.GetDetail(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load payment"})
		return
	}
	c.JSON(http.StatusOK, d)
}
