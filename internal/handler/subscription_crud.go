package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"
)

// SubscriptionCRUDHandler handles full CRUD operations for subscriptions.
type SubscriptionCRUDHandler struct {
	repo          repository.SubscriptionRepository
	hubService    service.PubSubHub
	webhookSecret string
	logger        *slog.Logger
}

// NewSubscriptionCRUDHandler creates a new SubscriptionCRUDHandler.
func NewSubscriptionCRUDHandler(
	repo repository.SubscriptionRepository,
	hubService service.PubSubHub,
	webhookSecret string,
	logger *slog.Logger,
) *SubscriptionCRUDHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SubscriptionCRUDHandler{
		repo:          repo,
		hubService:    hubService,
		webhookSecret: webhookSecret,
		logger:        logger,
	}
}

// UpdateSubscriptionRequest represents the request to update a subscription.
type UpdateSubscriptionRequest struct {
	LeaseSeconds   *int    `json:"lease_seconds,omitempty"`
	Status         *string `json:"status,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	LastVerifiedAt *string `json:"last_verified_at,omitempty"`
}

// ServeHTTP routes subscription requests.
func (h *SubscriptionCRUDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions")

	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodPost:
			h.handleCreate(w, r)
		case http.MethodGet:
			h.handleList(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	if strings.HasPrefix(path, "/") {
		idStr := strings.TrimPrefix(path, "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid subscription ID", "subscription ID must be a valid integer", nil)
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, id)
		case http.MethodPut:
			h.handleUpdate(w, r, id)
		case http.MethodDelete:
			h.handleDelete(w, r, id)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

func (h *SubscriptionCRUDHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("failed to decode request body", "error", err)
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if err := h.validateCreateRequest(&req); err != nil {
		h.logger.Warn("invalid create request", "error", err)
		sendError(w, http.StatusBadRequest, "validation failed", err.Error(), nil)
		return
	}

	if req.LeaseSeconds == 0 {
		req.LeaseSeconds = 432000
	}

	secret := req.Secret
	if (secret == nil || *secret == "") && h.webhookSecret != "" {
		secret = &h.webhookSecret
	}

	sub := models.NewSubscription(req.ChannelID, req.CallbackURL, req.LeaseSeconds, secret)

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

		statusCode := http.StatusInternalServerError
		if errors.Is(err, service.ErrSubscriptionFailed) {
			statusCode = http.StatusBadRequest
		}

		sendError(w, statusCode, "failed to subscribe to hub", err.Error(), nil)
		return
	}

	if hubResp.Accepted {
		sub.MarkActive()
	} else {
		sub.MarkFailed()
	}

	if err := h.repo.Create(r.Context(), sub); err != nil {
		h.logger.Error("failed to save subscription to database",
			"error", err,
			"channel_id", req.ChannelID,
		)

		if db.IsDuplicateKey(err) {
			sendError(w, http.StatusConflict, "subscription already exists", "a subscription for this channel and callback URL already exists", nil)
			return
		}

		sendError(w, http.StatusInternalServerError, "failed to save subscription", err.Error(), nil)
		return
	}

	h.logger.Info("subscription created successfully",
		"subscription_id", sub.ID,
		"channel_id", sub.ChannelID,
		"status", sub.Status,
	)

	sendJSON(w, http.StatusCreated, sub)
}

func (h *SubscriptionCRUDHandler) handleGet(w http.ResponseWriter, r *http.Request, id int64) {
	sub, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("subscription with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to get subscription", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve subscription", nil)
		return
	}

	sendJSON(w, http.StatusOK, sub)
}

func (h *SubscriptionCRUDHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	expiresBefore, err := parseTimestamp(r, "expires_before")
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", err.Error(), nil)
		return
	}

	filters := &repository.SubscriptionFilters{
		Limit:         limit,
		Offset:        offset,
		ChannelID:     r.URL.Query().Get("channel_id"),
		Status:        r.URL.Query().Get("status"),
		ExpiresBefore: expiresBefore,
	}

	subscriptions, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list subscriptions", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list subscriptions", nil)
		return
	}

	response := map[string]interface{}{
		"items":  subscriptions,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

func (h *SubscriptionCRUDHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	sub, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("subscription with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to get subscription", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve subscription", nil)
		return
	}

	if req.LeaseSeconds != nil {
		sub.LeaseSeconds = *req.LeaseSeconds
	}

	if req.Status != nil {
		sub.Status = *req.Status
	}

	if req.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			sendError(w, http.StatusBadRequest, "validation failed", "expires_at must be in RFC3339 format", nil)
			return
		}
		sub.ExpiresAt = expiresAt
	}

	if req.LastVerifiedAt != nil {
		lastVerifiedAt, err := time.Parse(time.RFC3339, *req.LastVerifiedAt)
		if err != nil {
			sendError(w, http.StatusBadRequest, "validation failed", "last_verified_at must be in RFC3339 format", nil)
			return
		}
		sub.LastVerifiedAt = &lastVerifiedAt
	}

	if err := h.repo.Update(r.Context(), sub); err != nil {
		h.logger.Error("failed to update subscription", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to update subscription", nil)
		return
	}

	sendJSON(w, http.StatusOK, sub)
}

func (h *SubscriptionCRUDHandler) handleDelete(w http.ResponseWriter, r *http.Request, id int64) {
	unsubscribe, _ := parseBool(r, "unsubscribe")

	if unsubscribe != nil && *unsubscribe {
		sub, err := h.repo.GetByID(r.Context(), id)
		if err != nil && !db.IsNotFound(err) {
			h.logger.Error("failed to get subscription for unsubscribe", "error", err, "id", id)
		}

		if err == nil {
			unsubReq := &service.SubscribeRequest{
				HubURL:      sub.HubURL,
				TopicURL:    sub.TopicURL,
				CallbackURL: sub.CallbackURL,
			}

			_, err := h.hubService.Unsubscribe(r.Context(), unsubReq)
			if err != nil {
				h.logger.Warn("failed to unsubscribe from hub (continuing with deletion)",
					"error", err,
					"subscription_id", id,
				)
			} else {
				h.logger.Info("successfully unsubscribed from hub", "subscription_id", id)
			}
		}
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("subscription with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to delete subscription", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to delete subscription", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SubscriptionCRUDHandler) validateCreateRequest(req *CreateSubscriptionRequest) error {
	if req.ChannelID == "" {
		return errors.New("channel_id is required")
	}

	if !YouTubeChannelIDRegex.MatchString(req.ChannelID) {
		return errors.New("invalid channel_id format (must start with 'UC' followed by 22 characters)")
	}

	if req.CallbackURL == "" {
		return errors.New("callback_url is required")
	}

	if !strings.HasPrefix(req.CallbackURL, "http://") && !strings.HasPrefix(req.CallbackURL, "https://") {
		return errors.New("callback_url must be a valid HTTP or HTTPS URL")
	}

	if req.LeaseSeconds < 0 {
		return errors.New("lease_seconds must be non-negative")
	}

	if req.LeaseSeconds > 864000 {
		return errors.New("lease_seconds cannot exceed 864000 (10 days)")
	}

	return nil
}
