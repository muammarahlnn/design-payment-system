// Package fakegateway stands in for a real payment provider (Midtrans, Xendit,
// Stripe). It exists so the worker has something to charge against, and so we
// can simulate the failures that matter: declines, errors, and timeouts.
//
// It is deliberately idempotent, because real gateways are. Charging twice
// with the same Idempotency-Key returns the first charge rather than taking
// the money again — which is what makes the worker safe to retry.
package fakegateway

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// idempotencyHeader matches what real gateways use, and what our own API
// accepts. The worker sends the payment id here.
const idempotencyHeader = "Idempotency-Key"

// Fail modes, wired from GATEWAY_FAIL_MODE. Empty means behave normally.
const (
	FailNone    = ""
	FailDecline = "decline" // the gateway refuses the charge (402)
	FailError   = "error"   // the gateway is broken (500) — retryable
	FailTimeout = "timeout" // the gateway hangs — exercises the client timeout
)

// Charge is a charge this gateway has accepted, remembered by idempotency key.
type Charge struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

// Handler is the HTTP layer of the fake gateway. Charges live in memory: this
// is a stub, and losing them on restart is fine.
type Handler struct {
	mu       sync.RWMutex
	failMode string

	store    *store
	notifier *Notifier
}

func NewHandler(failMode, storePath string, notifier *Notifier) *Handler {
	if failMode != FailNone {
		log.Printf("fake-gateway: fail mode %q active", failMode)
	}
	ensureDir(storePath)
	return &Handler{
		store:    newStore(storePath),
		failMode: failMode,
		notifier: notifier,
	}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	r.POST("/charges", h.charge)
	r.POST("/charges/:key/settle", h.settle)
	r.GET("/charges/:key", h.getCharge)
	r.GET("/config", h.getConfig)
	r.PUT("/config", h.setConfig)
}

// mode reads the current fail mode under the lock.
func (h *Handler) mode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.failMode
}

type chargeRequest struct {
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	Currency      string `json:"currency" binding:"required,len=3"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

func (h *Handler) charge(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader(idempotencyHeader))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": idempotencyHeader + " header is required"})
		return
	}

	var req chargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Idempotent replay: the same key always gets the first answer back, and
	// the money is never taken twice. This is what lets the worker retry.
	existing, found := h.store.get(key)
	if found {
		log.Printf("fake-gateway: replaying charge %s for key %s", existing.Reference, key)
		c.JSON(http.StatusOK, existing)
		return
	}

	switch h.mode() {
	case FailTimeout:
		// Hang past any sane client timeout. The client gives up first.
		time.Sleep(60 * time.Second)
		return
	case FailError:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gateway unavailable"})
		return
	case FailDecline:
		// A decline is a real answer, not a failure. Record it so a retry
		// replays the same decline instead of trying the charge again.
		declined := Charge{
			Reference: "gw_" + uuid.NewString(),
			Status:    "DECLINED",
			Amount:    req.Amount,
			Currency:  req.Currency,
		}
		h.store.put(key, declined)
		c.JSON(http.StatusPaymentRequired, declined)
		return
	}

	accepted := Charge{
		Reference: "gw_" + uuid.NewString(),
		Status:    "ACCEPTED",
		Amount:    req.Amount,
		Currency:  req.Currency,
	}
	h.store.put(key, accepted)

	log.Printf("fake-gateway: accepted charge %s (key %s, %d %s) — awaiting payment",
		accepted.Reference, key, accepted.Amount, accepted.Currency)

	// No callback yet. Accepting a charge only creates the instruction: a VA
	// number exists, or a QR code exists, and the customer has not paid. The
	// settle endpoint below is where that happens.
	c.JSON(http.StatusCreated, accepted)
}

type settleRequest struct {
	// Status is SUCCEEDED or FAILED. Empty means SUCCEEDED, so the common
	// case — "the customer paid" — needs no body at all.
	Status string `json:"status"`
}

type settleResponse struct {
	PaymentID   string `json:"payment_id"`
	Reference   string `json:"reference"`
	Status      string `json:"status"`
	EventID     string `json:"webhook_event_id"`
	WebhookCode int    `json:"webhook_http_status"`
}

// settle stands in for the customer completing the payment: scanning the QRIS
// code, transferring to the virtual account, confirming in their banking app.
//
// Doing it by hand rather than on a timer is the point. A real payment sits in
// PROCESSING for as long as the customer takes, which may be never — and being
// able to hold it there, or settle it on demand, is what makes the state
// testable.
//
// key is the merchant's payment id, the same value they sent as
// Idempotency-Key when creating the charge.
func (h *Handler) settle(c *gin.Context) {
	key := c.Param("key")

	charge, found := h.store.get(key)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "no charge for key " + key})
		return
	}

	var req settleRequest
	// An empty body is fine and means SUCCEEDED.
	_ = c.ShouldBindJSON(&req)

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "SUCCEEDED"
	}
	if status != "SUCCEEDED" && status != "FAILED" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be SUCCEEDED or FAILED"})
		return
	}

	if h.notifier == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no webhook url configured"})
		return
	}

	eventID, code, err := h.notifier.Notify(c.Request.Context(), key, charge, status)
	if err != nil {
		log.Printf("fake-gateway: settle %s failed: %v", key, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "webhook_event_id": eventID})
		return
	}

	charge.Status = status
	h.store.put(key, charge)

	c.JSON(http.StatusOK, settleResponse{
		PaymentID:   key,
		Reference:   charge.Reference,
		Status:      status,
		EventID:     eventID,
		WebhookCode: code,
	})
}

// getCharge lets you see what the gateway thinks of a charge, by payment id.
func (h *Handler) getCharge(c *gin.Context) {
	key := c.Param("key")

	charge, found := h.store.get(key)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "no charge for key " + key})
		return
	}
	c.JSON(http.StatusOK, charge)
}

type configBody struct {
	FailMode string `json:"fail_mode"`
}

func (h *Handler) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"fail_mode": h.mode(),
		"modes":     []string{FailNone, FailDecline, FailError, FailTimeout},
		"charges":   h.store.count(),
	})
}

// setConfig flips the fail mode at runtime.
//
// The env var still sets the starting value, but changing behaviour used to
// mean recreating the container, which made the failure paths tedious to
// demonstrate. This is a stub for learning; a real gateway would never expose
// such a thing.
func (h *Handler) setConfig(c *gin.Context) {
	var body configBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode := strings.ToLower(strings.TrimSpace(body.FailMode))
	switch mode {
	case FailNone, FailDecline, FailError, FailTimeout:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown fail_mode " + mode})
		return
	}

	h.mu.Lock()
	h.failMode = mode
	h.mu.Unlock()

	if mode == FailNone {
		log.Printf("fake-gateway: fail mode cleared — behaving normally")
	} else {
		log.Printf("fake-gateway: fail mode set to %q", mode)
	}
	c.JSON(http.StatusOK, gin.H{"fail_mode": mode})
}
