package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

const (
	defaultLimit = 50
	maxLimit     = 1000
)

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PaginatedResponse contains common pagination metadata.
type PaginatedResponse struct {
	Count  int `json:"count"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Helper functions

func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

func sendError(w http.ResponseWriter, statusCode int, error string, message string, details map[string]interface{}) {
	sendJSON(w, statusCode, ErrorResponse{
		Error:   error,
		Message: message,
		Details: details,
	})
}

func parseLimit(r *http.Request) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}

func parseOffset(r *http.Request) int {
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		return 0
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}

	return offset
}

func parseBool(r *http.Request, key string) (*bool, error) {
	val := r.URL.Query().Get(key)
	if val == "" {
		return nil, nil
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		return nil, fmt.Errorf("invalid boolean value for %s", key)
	}

	return &b, nil
}

func parseTimestamp(r *http.Request, key string) (*time.Time, error) {
	val := r.URL.Query().Get(key)
	if val == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format for %s (expected RFC3339)", key)
	}

	return &t, nil
}

func getOrderDir(r *http.Request) string {
	orderDir := strings.ToUpper(r.URL.Query().Get("order"))
	if orderDir != "ASC" && orderDir != "DESC" {
		return "DESC"
	}
	return orderDir
}

// WebhookEventHandler handles CRUD operations for webhook events.
type WebhookEventHandler struct {
	repo   repository.WebhookEventRepository
	logger *slog.Logger
}

// NewWebhookEventHandler creates a new WebhookEventHandler.
func NewWebhookEventHandler(repo repository.WebhookEventRepository, logger *slog.Logger) *WebhookEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebhookEventHandler{
		repo:   repo,
		logger: logger,
	}
}

// CreateWebhookEventRequest represents the request to create a webhook event.
type CreateWebhookEventRequest struct {
	RawXML      string `json:"raw_xml"`
	ContentHash string `json:"content_hash"`
	VideoID     string `json:"video_id,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
}

// UpdateWebhookEventRequest represents the request to update a webhook event's processing status.
type UpdateWebhookEventRequest struct {
	Processed       *bool   `json:"processed,omitempty"`
	ProcessedAt     *string `json:"processed_at,omitempty"`
	ProcessingError *string `json:"processing_error,omitempty"`
}

// ServeHTTP routes webhook event requests.
func (h *WebhookEventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webhook-events")

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
		eventID := strings.TrimPrefix(path, "/")
		id, err := strconv.ParseInt(eventID, 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid event ID", "event ID must be a valid integer", nil)
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, id)
		case http.MethodPatch:
			h.handleUpdate(w, r, id)
		case http.MethodDelete:
			h.handleDelete(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

func (h *WebhookEventHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateWebhookEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.RawXML == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "raw_xml is required", nil)
		return
	}

	if req.ContentHash == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "content_hash is required", nil)
		return
	}

	if len(req.ContentHash) != 64 {
		sendError(w, http.StatusBadRequest, "validation failed", "content_hash must be 64 characters (SHA-256)", nil)
		return
	}

	now := time.Now()
	event := &models.WebhookEvent{
		RawXML:      req.RawXML,
		ContentHash: req.ContentHash,
		ReceivedAt:  now,
		Processed:   false,
		VideoID:     sql.NullString{String: req.VideoID, Valid: req.VideoID != ""},
		ChannelID:   sql.NullString{String: req.ChannelID, Valid: req.ChannelID != ""},
		CreatedAt:   now,
	}

	if err := h.repo.Create(r.Context(), event); err != nil {
		if db.IsDuplicateKey(err) {
			sendError(w, http.StatusConflict, "conflict", "webhook event with this content_hash already exists", nil)
			return
		}
		h.logger.Error("failed to create webhook event", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to create webhook event", nil)
		return
	}

	sendJSON(w, http.StatusCreated, event)
}

func (h *WebhookEventHandler) handleGet(w http.ResponseWriter, r *http.Request, id int64) {
	event, err := h.repo.GetEventByID(r.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("webhook event with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to get webhook event", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve webhook event", nil)
		return
	}

	sendJSON(w, http.StatusOK, event)
}

func (h *WebhookEventHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	processed, err := parseBool(r, "processed")
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", err.Error(), nil)
		return
	}

	filters := &repository.WebhookEventFilters{
		Limit:     limit,
		Offset:    offset,
		Processed: processed,
		VideoID:   r.URL.Query().Get("video_id"),
		ChannelID: r.URL.Query().Get("channel_id"),
		OrderBy:   r.URL.Query().Get("order_by"),
		OrderDir:  getOrderDir(r),
	}

	if filters.OrderBy == "" {
		filters.OrderBy = "received_at"
	}

	events, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list webhook events", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list webhook events", nil)
		return
	}

	response := map[string]interface{}{
		"webhook_events": events,
		"count":          len(events),
		"total":          total,
		"limit":          limit,
		"offset":         offset,
	}

	sendJSON(w, http.StatusOK, response)
}

func (h *WebhookEventHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var req UpdateWebhookEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	processed := false
	if req.Processed != nil {
		processed = *req.Processed
	}

	processingError := ""
	if req.ProcessingError != nil {
		processingError = *req.ProcessingError
	}

	if err := h.repo.UpdateProcessingStatus(r.Context(), id, processed, processingError); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("webhook event with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to update webhook event", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to update webhook event", nil)
		return
	}

	event, err := h.repo.GetEventByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get updated webhook event", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve updated webhook event", nil)
		return
	}

	sendJSON(w, http.StatusOK, event)
}

func (h *WebhookEventHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	sendError(w, http.StatusForbidden, "Forbidden", "Deleting webhook events is not allowed - events are immutable", nil)
}

// ChannelHandler handles CRUD operations for channels.
type ChannelHandler struct {
	repo   repository.ChannelRepository
	logger *slog.Logger
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(repo repository.ChannelRepository, logger *slog.Logger) *ChannelHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelHandler{
		repo:   repo,
		logger: logger,
	}
}

// CreateChannelRequest represents the request to create a channel.
type CreateChannelRequest struct {
	ChannelID  string `json:"channel_id"`
	Title      string `json:"title"`
	ChannelURL string `json:"channel_url"`
}

// UpdateChannelRequest represents the request to update a channel.
type UpdateChannelRequest struct {
	Title      string `json:"title"`
	ChannelURL string `json:"channel_url"`
}

// ServeHTTP routes channel requests.
func (h *ChannelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/channels")

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
		channelID := strings.TrimPrefix(path, "/")

		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, channelID)
		case http.MethodPut:
			h.handleUpdate(w, r, channelID)
		case http.MethodDelete:
			h.handleDelete(w, r, channelID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

func (h *ChannelHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.ChannelID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "channel_id is required", nil)
		return
	}

	if !YouTubeChannelIDRegex.MatchString(req.ChannelID) {
		sendError(w, http.StatusBadRequest, "validation failed", "invalid channel_id format (must start with 'UC' followed by 22 characters)", nil)
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "title is required", nil)
		return
	}

	if req.ChannelURL == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "channel_url is required", nil)
		return
	}

	channel := models.NewChannel(req.ChannelID, req.Title, req.ChannelURL)

	if err := h.repo.Create(r.Context(), channel); err != nil {
		if db.IsDuplicateKey(err) {
			sendError(w, http.StatusConflict, "conflict", "channel already exists", nil)
			return
		}
		h.logger.Error("failed to create channel", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to create channel", nil)
		return
	}

	sendJSON(w, http.StatusCreated, channel)
}

func (h *ChannelHandler) handleGet(w http.ResponseWriter, r *http.Request, channelID string) {
	channel, err := h.repo.GetChannelByID(r.Context(), channelID)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("channel with id '%s' not found", channelID), nil)
			return
		}
		h.logger.Error("failed to get channel", "error", err, "channel_id", channelID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve channel", nil)
		return
	}

	sendJSON(w, http.StatusOK, channel)
}

func (h *ChannelHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	filters := &repository.ChannelFilters{
		Limit:    limit,
		Offset:   offset,
		Title:    r.URL.Query().Get("title"),
		OrderBy:  r.URL.Query().Get("order_by"),
		OrderDir: getOrderDir(r),
	}

	if filters.OrderBy == "" {
		filters.OrderBy = "last_updated_at"
	}

	channels, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list channels", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list channels", nil)
		return
	}

	response := map[string]interface{}{
		"channels": channels,
		"count":    len(channels),
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}

	sendJSON(w, http.StatusOK, response)
}

func (h *ChannelHandler) handleUpdate(w http.ResponseWriter, r *http.Request, channelID string) {
	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "title is required", nil)
		return
	}

	if req.ChannelURL == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "channel_url is required", nil)
		return
	}

	channel := &models.Channel{
		ChannelID:  channelID,
		Title:      req.Title,
		ChannelURL: req.ChannelURL,
	}

	if err := h.repo.Update(r.Context(), channel); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("channel with id '%s' not found", channelID), nil)
			return
		}
		h.logger.Error("failed to update channel", "error", err, "channel_id", channelID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to update channel", nil)
		return
	}

	sendJSON(w, http.StatusOK, channel)
}

func (h *ChannelHandler) handleDelete(w http.ResponseWriter, r *http.Request, channelID string) {
	if err := h.repo.Delete(r.Context(), channelID); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("channel with id '%s' not found", channelID), nil)
			return
		}
		h.logger.Error("failed to delete channel", "error", err, "channel_id", channelID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to delete channel", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
