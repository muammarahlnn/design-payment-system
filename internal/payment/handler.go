package payment

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// idempotencyHeader is where the client puts its key. A header rather than a
// body field, because the key describes the request, not the charge. This is
// what Stripe, Adyen, and most gateways use.
const idempotencyHeader = "Idempotency-Key"

// Handler is the HTTP layer of the payment feature.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires the feature's endpoints onto the given router.
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	r.POST("/payments", h.create)
}

type createPaymentRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	// Amount is WHOLE RUPIAH, not cents: 1050 means Rp 1.050. IDR is a
	// zero-decimal currency in practice, so there is no sub-unit to carry.
	// An integer also avoids the rounding errors a float brings to money.
	Amount int64 `json:"amount" binding:"required,gt=0"`
	// Currency is optional and defaults to IDR, the only currency this
	// project supports. Sending anything else is rejected rather than
	// silently converted.
	Currency      string `json:"currency" binding:"omitempty,len=3,alpha"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

type paymentResponse struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (h *Handler) create(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader(idempotencyHeader))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": idempotencyHeader + " header is required",
		})
		return
	}

	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalize before validating and hashing, so "idr" and "IDR" are one
	// currency and not two different request fingerprints.
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = CurrencyIDR
	}
	if currency != CurrencyIDR {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "only " + CurrencyIDR + " is supported",
		})
		return
	}

	p, created, err := h.service.Create(c.Request.Context(), CreateInput{
		IdempotencyKey: key,
		CustomerID:     req.CustomerID,
		Amount:         req.Amount,
		Currency:       currency,
		PaymentMethod:  req.PaymentMethod,
	})
	if errors.Is(err, ErrIdempotencyKeyReused) {
		c.JSON(http.StatusConflict, gin.H{
			"error": "this " + idempotencyHeader + " was already used for a different payment",
		})
		return
	}
	if err != nil {
		log.Printf("payment: create failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}

	// 201 for a payment this request created, 200 for a replay of one that
	// already existed. Both return the same body, so a retrying client does
	// not need to care which it got.
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, toResponse(p))
}

func toResponse(p *Payment) paymentResponse {
	return paymentResponse{
		ID:            p.ID,
		CustomerID:    p.CustomerID,
		Amount:        p.Amount,
		Currency:      p.Currency,
		PaymentMethod: p.PaymentMethod,
		Status:        p.Status,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}
