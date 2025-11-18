package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"
)

var (
	// YouTubeChannelIDRegex validates YouTube channel IDs (UC followed by 22 characters).
	YouTubeChannelIDRegex = regexp.MustCompile(`^UC[a-zA-Z0-9_-]{22}$`)
)

// SubscriptionHandler handles HTTP requests for managing PubSubHubbub subscriptions.
type SubscriptionHandler struct {
	repo          repository.SubscriptionRepository
	hubService    service.PubSubHub
	webhookSecret string
	logger        *slog.Logger
}

// NewSubscriptionHandler creates a new SubscriptionHandler.
func NewSubscriptionHandler(
	repo repository.SubscriptionRepository,
	hubService service.PubSubHub,
	webhookSecret string,
	logger *slog.Logger,
) *SubscriptionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SubscriptionHandler{
		repo:          repo,
		hubService:    hubService,
		webhookSecret: webhookSecret,
		logger:        logger,
	}
}

// CreateSubscriptionRequest represents the request body for creating a subscription.
type CreateSubscriptionRequest struct {
	ChannelID    string  `json:"channel_id"`
	CallbackURL  string  `json:"callback_url"`
	LeaseSeconds int     `json:"lease_seconds,omitempty"`
	Secret       *string `json:"secret,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ServeHTTP handles subscription-related HTTP requests.
func (h *SubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed", "")
	}
}

// handleCreate handles POST requests to create a new subscription.
func (h *SubscriptionHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("failed to decode request body", "error", err)
		h.sendError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate request
	if err := h.validateCreateRequest(&req); err != nil {
		h.logger.Warn("invalid create request", "error", err)
		h.sendError(w, http.StatusBadRequest, "validation failed", err.Error())
		return
	}

	// Set default lease seconds if not provided
	if req.LeaseSeconds == 0 {
		req.LeaseSeconds = 432000 // 5 days default
	}

	// Use configured webhook secret if not explicitly provided in request
	secret := req.Secret
	if (secret == nil || *secret == "") && h.webhookSecret != "" {
		secret = &h.webhookSecret
	}

	// Create subscription model
	sub := models.NewSubscription(req.ChannelID, req.CallbackURL, req.LeaseSeconds, secret)

	// Subscribe via PubSubHubbub
	hubReq := &service.SubscribeRequest{
		HubURL:       sub.HubURL,
		TopicURL:     sub.TopicURL,
		CallbackURL:  sub.CallbackURL,
		LeaseSeconds: sub.LeaseSeconds,
		Secret:       sub.Secret,
	}

	h.logger.Info("attempting to subscribe to PubSubHub",
		"channel_id", req.ChannelID,
		"callback_url", req.CallbackURL,
	)

	hubResp, err := h.hubService.Subscribe(r.Context(), hubReq)
	if err != nil {
		h.logger.Error("failed to subscribe to hub",
			"error", err,
			"channel_id", req.ChannelID,
		)

		// Determine status code based on error type
		statusCode := http.StatusInternalServerError
		if errors.Is(err, service.ErrSubscriptionFailed) {
			statusCode = http.StatusBadRequest
		}

		h.sendError(w, statusCode, "failed to subscribe to hub", err.Error())
		return
	}

	// If subscription was accepted, mark as active
	if hubResp.Accepted {
		sub.MarkActive()
	} else {
		sub.MarkFailed()
	}

	// Save subscription to database
	if err := h.repo.Create(r.Context(), sub); err != nil {
		h.logger.Error("failed to save subscription to database",
			"error", err,
			"channel_id", req.ChannelID,
		)

		// Check if duplicate
		if db.IsDuplicateKey(err) {
			h.sendError(w, http.StatusConflict, "subscription already exists", "a subscription for this channel and callback URL already exists")
			return
		}

		h.sendError(w, http.StatusInternalServerError, "failed to save subscription", err.Error())
		return
	}

	h.logger.Info("subscription created successfully",
		"subscription_id", sub.ID,
		"channel_id", sub.ChannelID,
		"status", sub.Status,
	)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sub)
}

// validateCreateRequest validates the create subscription request.
func (h *SubscriptionHandler) validateCreateRequest(req *CreateSubscriptionRequest) error {
	if req.ChannelID == "" {
		return errors.New("channel_id is required")
	}

	// Validate channel ID format
	if !YouTubeChannelIDRegex.MatchString(req.ChannelID) {
		return errors.New("invalid channel_id format (must start with 'UC' followed by 22 characters)")
	}

	if req.CallbackURL == "" {
		return errors.New("callback_url is required")
	}

	// Validate callback URL format
	if !strings.HasPrefix(req.CallbackURL, "http://") && !strings.HasPrefix(req.CallbackURL, "https://") {
		return errors.New("callback_url must be a valid HTTP or HTTPS URL")
	}

	if req.LeaseSeconds < 0 {
		return errors.New("lease_seconds must be non-negative")
	}

	// Validate lease seconds range (YouTube typically requires between 1 hour and 10 days)
	if req.LeaseSeconds > 864000 { // 10 days
		return errors.New("lease_seconds cannot exceed 864000 (10 days)")
	}

	return nil
}

// sendError sends a JSON error response.
func (h *SubscriptionHandler) sendError(w http.ResponseWriter, statusCode int, error string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error:   error,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode error response", "error", err)
	}
}

// GetSubscriptionHandler handles GET requests for retrieving subscriptions.
type GetSubscriptionHandler struct {
	repo   repository.SubscriptionRepository
	logger *slog.Logger
}

// NewGetSubscriptionHandler creates a new GetSubscriptionHandler.
func NewGetSubscriptionHandler(repo repository.SubscriptionRepository, logger *slog.Logger) *GetSubscriptionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GetSubscriptionHandler{
		repo:   repo,
		logger: logger,
	}
}

// ServeHTTP handles GET requests for subscriptions.
func (h *GetSubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "method not allowed"})
		return
	}

	// Get channel_id from query parameters
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "channel_id query parameter is required"})
		return
	}

	// Retrieve subscriptions for the channel
	subscriptions, err := h.repo.GetByChannelID(r.Context(), channelID)
	if err != nil {
		h.logger.Error("failed to retrieve subscriptions", "error", err, "channel_id", channelID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "failed to retrieve subscriptions"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subscriptions": subscriptions,
		"count":         len(subscriptions),
	})
}

// HealthCheckHandler provides a simple health check endpoint.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"status":"ok"}`)
}
