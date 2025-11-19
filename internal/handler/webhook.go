package handler

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/service"
)

// WebhookHandler handles YouTube PubSubHubbub webhook requests.
type WebhookHandler struct {
	processor service.EventProcessor
	secret    string
	logger    *slog.Logger
}

// NewWebhookHandler creates a new webhook handler with the given processor and secret.
// The secret is required and used for HMAC signature verification of all webhook notifications.
func NewWebhookHandler(processor service.EventProcessor, secret string, logger *slog.Logger) *WebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebhookHandler{
		processor: processor,
		secret:    secret,
		logger:    logger,
	}
}

// ServeHTTP handles both subscription verification (GET) and notification (POST) requests.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleVerification(w, r)
	case http.MethodPost:
		h.handleNotification(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerification handles GET requests for subscription verification.
// YouTube sends a hub.challenge parameter that must be echoed back.
func (h *WebhookHandler) handleVerification(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("hub.challenge")
	if challenge == "" {
		h.logger.Warn("verification request missing hub.challenge parameter")
		http.Error(w, "Missing hub.challenge parameter", http.StatusBadRequest)
		return
	}

	// Log the verification request
	h.logger.Info("subscription verification request",
		"hub.mode", r.URL.Query().Get("hub.mode"),
		"hub.topic", r.URL.Query().Get("hub.topic"),
		"hub.lease_seconds", r.URL.Query().Get("hub.lease_seconds"),
	)

	// Return the challenge to confirm subscription
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(challenge))
}

// handleNotification handles POST requests containing Atom feed notifications.
func (h *WebhookHandler) handleNotification(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify HMAC signature (always required)
	if err := h.verifySignature(r, body); err != nil {
		h.logger.Warn("signature verification failed", "error", err)
		http.Error(w, "Signature verification failed", http.StatusUnauthorized)
		return
	}

	// Process the event
	if err := h.processor.ProcessEvent(r.Context(), string(body)); err != nil {
		h.logger.Error("failed to process event", "error", err)
		http.Error(w, "Failed to process event", http.StatusInternalServerError)
		return
	}

	h.logger.Info("successfully processed webhook notification")
	w.WriteHeader(http.StatusOK)
}

// verifySignature verifies the X-Hub-Signature header using HMAC-SHA1.
// The signature format is "sha1={hex-encoded-signature}".
func (h *WebhookHandler) verifySignature(r *http.Request, body []byte) error {
	signature := r.Header.Get("X-Hub-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Hub-Signature header")
	}

	// Extract the hex signature (format: "sha1={signature}")
	if !strings.HasPrefix(signature, "sha1=") {
		return fmt.Errorf("invalid signature format: must start with 'sha1='")
	}
	expectedSig := strings.TrimPrefix(signature, "sha1=")

	// Compute HMAC-SHA1 of the body
	mac := hmac.New(sha1.New, []byte(h.secret))
	mac.Write(body)
	computedSig := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures using constant-time comparison
	if !hmac.Equal([]byte(computedSig), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
