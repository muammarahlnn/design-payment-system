package payment

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/muammarahlnn/design-payment-system/internal/webhook"
)

// maxWebhookBody caps what we will read from an unauthenticated caller. The
// signature is only checked after the body is in memory, so without a limit
// anyone who can reach this endpoint could exhaust it.
const maxWebhookBody = 64 << 10 // 64 KiB

// WebhookHandler is the HTTP layer for gateway callbacks.
type WebhookHandler struct {
	service *WebhookService
	secret  string
}

func NewWebhookHandler(service *WebhookService, secret string) *WebhookHandler {
	return &WebhookHandler{service: service, secret: secret}
}

func (h *WebhookHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/webhook/payment", h.receive)
}

type webhookRequest struct {
	EventID   string `json:"event_id"`
	PaymentID string `json:"payment_id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

func (h *WebhookHandler) receive(c *gin.Context) {
	// Read the raw bytes first. The signature covers exactly what was sent,
	// so decoding and re-encoding would produce different bytes and a
	// mismatch on a request that is perfectly valid.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	if err := webhook.Verify(
		h.secret,
		c.GetHeader(webhook.HeaderTimestamp),
		c.GetHeader(webhook.HeaderSignature),
		body,
		time.Now(),
		webhook.DefaultTolerance,
	); err != nil {
		// Log the reason, return a bare 401. Telling an unauthenticated
		// caller whether it failed on the timestamp or the digest hands them
		// a tool for probing.
		log.Printf("webhook: rejected delivery: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var req webhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed json"})
		return
	}
	if req.EventID == "" || req.PaymentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id and payment_id are required"})
		return
	}

	err = h.service.Handle(c.Request.Context(), WebhookEvent{
		EventID:   req.EventID,
		PaymentID: req.PaymentID,
		Reference: req.Reference,
		Status:    req.Status,
	})
	switch {
	case errors.Is(err, ErrUnknownStatus):
		// A status we do not model will never become one. Answering 400 stops
		// the gateway retrying something that cannot succeed.
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case err != nil:
		// Our fault. A 5xx tells the gateway to deliver it again, which is
		// what we want — the delivery was valid, we just could not store it.
		log.Printf("webhook: handling delivery %s failed: %v", req.EventID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process webhook"})
		return
	}

	// 200 means "received and settled". Gateways stop retrying on 2xx, so we
	// only say this once the transaction has committed.
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
