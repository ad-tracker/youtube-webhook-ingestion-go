package handler

import (
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

// VideoHandler handles CRUD operations for videos.
type VideoHandler struct {
	repo   repository.VideoRepository
	logger *slog.Logger
}

// NewVideoHandler creates a new VideoHandler.
func NewVideoHandler(repo repository.VideoRepository, logger *slog.Logger) *VideoHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &VideoHandler{
		repo:   repo,
		logger: logger,
	}
}

// CreateVideoRequest represents the request to create a video.
type CreateVideoRequest struct {
	VideoID     string `json:"video_id"`
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	VideoURL    string `json:"video_url"`
	PublishedAt string `json:"published_at"`
}

// UpdateVideoRequest represents the request to update a video.
type UpdateVideoRequest struct {
	Title       string `json:"title"`
	VideoURL    string `json:"video_url"`
	PublishedAt string `json:"published_at"`
}

// ServeHTTP routes video requests.
func (h *VideoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/videos")

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
		videoID := strings.TrimPrefix(path, "/")

		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, videoID)
		case http.MethodPut:
			h.handleUpdate(w, r, videoID)
		case http.MethodDelete:
			h.handleDelete(w, r, videoID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

func (h *VideoHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.VideoID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "video_id is required", nil)
		return
	}

	if req.ChannelID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "channel_id is required", nil)
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "title is required", nil)
		return
	}

	if req.VideoURL == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "video_url is required", nil)
		return
	}

	if req.PublishedAt == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at is required", nil)
		return
	}

	publishedAt, err := time.Parse(time.RFC3339, req.PublishedAt)
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at must be in RFC3339 format", nil)
		return
	}

	video := models.NewVideo(req.VideoID, req.ChannelID, req.Title, req.VideoURL, publishedAt)

	if err := h.repo.Create(r.Context(), video); err != nil {
		if db.IsDuplicateKey(err) {
			sendError(w, http.StatusConflict, "conflict", "video already exists", nil)
			return
		}
		if db.IsForeignKeyViolation(err) {
			sendError(w, http.StatusBadRequest, "validation failed", "referenced channel does not exist", map[string]interface{}{
				"field": "channel_id",
				"value": req.ChannelID,
			})
			return
		}
		h.logger.Error("failed to create video", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to create video", nil)
		return
	}

	sendJSON(w, http.StatusCreated, video)
}

func (h *VideoHandler) handleGet(w http.ResponseWriter, r *http.Request, videoID string) {
	video, err := h.repo.GetVideoByID(r.Context(), videoID)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("video with id '%s' not found", videoID), nil)
			return
		}
		h.logger.Error("failed to get video", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve video", nil)
		return
	}

	sendJSON(w, http.StatusOK, video)
}

func (h *VideoHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	publishedAfter, err := parseTimestamp(r, "published_after")
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", err.Error(), nil)
		return
	}

	publishedBefore, err := parseTimestamp(r, "published_before")
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", err.Error(), nil)
		return
	}

	filters := &repository.VideoFilters{
		Limit:           limit,
		Offset:          offset,
		ChannelID:       r.URL.Query().Get("channel_id"),
		Title:           r.URL.Query().Get("title"),
		PublishedAfter:  publishedAfter,
		PublishedBefore: publishedBefore,
		OrderBy:         r.URL.Query().Get("order_by"),
		OrderDir:        getOrderDir(r),
	}

	if filters.OrderBy == "" {
		filters.OrderBy = "published_at"
	}

	videos, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list videos", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list videos", nil)
		return
	}

	response := map[string]interface{}{
		"items":  videos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

func (h *VideoHandler) handleUpdate(w http.ResponseWriter, r *http.Request, videoID string) {
	var req UpdateVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "title is required", nil)
		return
	}

	if req.VideoURL == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "video_url is required", nil)
		return
	}

	if req.PublishedAt == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at is required", nil)
		return
	}

	publishedAt, err := time.Parse(time.RFC3339, req.PublishedAt)
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at must be in RFC3339 format", nil)
		return
	}

	video := &models.Video{
		VideoID:     videoID,
		Title:       req.Title,
		VideoURL:    req.VideoURL,
		PublishedAt: publishedAt,
	}

	if err := h.repo.Update(r.Context(), video); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("video with id '%s' not found", videoID), nil)
			return
		}
		h.logger.Error("failed to update video", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to update video", nil)
		return
	}

	sendJSON(w, http.StatusOK, video)
}

func (h *VideoHandler) handleDelete(w http.ResponseWriter, r *http.Request, videoID string) {
	if err := h.repo.Delete(r.Context(), videoID); err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("video with id '%s' not found", videoID), nil)
			return
		}
		h.logger.Error("failed to delete video", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to delete video", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VideoUpdateHandler handles CRUD operations for video updates.
type VideoUpdateHandler struct {
	repo   repository.VideoUpdateRepository
	logger *slog.Logger
}

// NewVideoUpdateHandler creates a new VideoUpdateHandler.
func NewVideoUpdateHandler(repo repository.VideoUpdateRepository, logger *slog.Logger) *VideoUpdateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &VideoUpdateHandler{
		repo:   repo,
		logger: logger,
	}
}

// CreateVideoUpdateRequest represents the request to create a video update.
type CreateVideoUpdateRequest struct {
	WebhookEventID int64  `json:"webhook_event_id"`
	VideoID        string `json:"video_id"`
	ChannelID      string `json:"channel_id"`
	Title          string `json:"title"`
	PublishedAt    string `json:"published_at"`
	FeedUpdatedAt  string `json:"feed_updated_at"`
	UpdateType     string `json:"update_type"`
}

// ServeHTTP routes video update requests.
func (h *VideoUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/video-updates")

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
		updateID := strings.TrimPrefix(path, "/")
		id, err := strconv.ParseInt(updateID, 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid update ID", "update ID must be a valid integer", nil)
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, id)
		case http.MethodPatch:
			h.handlePatch(w, r)
		case http.MethodDelete:
			h.handleDelete(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

func (h *VideoUpdateHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateVideoUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	if req.WebhookEventID == 0 {
		sendError(w, http.StatusBadRequest, "validation failed", "webhook_event_id is required", nil)
		return
	}

	if req.VideoID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "video_id is required", nil)
		return
	}

	if req.ChannelID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "channel_id is required", nil)
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "title is required", nil)
		return
	}

	if req.PublishedAt == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at is required", nil)
		return
	}

	if req.FeedUpdatedAt == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "feed_updated_at is required", nil)
		return
	}

	if req.UpdateType == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "update_type is required", nil)
		return
	}

	publishedAt, err := time.Parse(time.RFC3339, req.PublishedAt)
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", "published_at must be in RFC3339 format", nil)
		return
	}

	feedUpdatedAt, err := time.Parse(time.RFC3339, req.FeedUpdatedAt)
	if err != nil {
		sendError(w, http.StatusBadRequest, "validation failed", "feed_updated_at must be in RFC3339 format", nil)
		return
	}

	update := models.NewVideoUpdate(
		req.WebhookEventID,
		req.VideoID,
		req.ChannelID,
		req.Title,
		publishedAt,
		feedUpdatedAt,
		models.UpdateType(req.UpdateType),
	)

	if err := h.repo.CreateVideoUpdate(r.Context(), update); err != nil {
		if db.IsForeignKeyViolation(err) {
			sendError(w, http.StatusBadRequest, "validation failed", "referenced foreign key does not exist", nil)
			return
		}
		h.logger.Error("failed to create video update", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to create video update", nil)
		return
	}

	sendJSON(w, http.StatusCreated, update)
}

func (h *VideoUpdateHandler) handleGet(w http.ResponseWriter, r *http.Request, id int64) {
	update, err := h.repo.GetUpdateByID(r.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("video update with id %d not found", id), nil)
			return
		}
		h.logger.Error("failed to get video update", "error", err, "id", id)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve video update", nil)
		return
	}

	sendJSON(w, http.StatusOK, update)
}

func (h *VideoUpdateHandler) handleList(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	webhookEventID := int64(0)
	if wid := r.URL.Query().Get("webhook_event_id"); wid != "" {
		id, err := strconv.ParseInt(wid, 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "validation failed", "webhook_event_id must be a valid integer", nil)
			return
		}
		webhookEventID = id
	}

	filters := &repository.VideoUpdateFilters{
		Limit:          limit,
		Offset:         offset,
		VideoID:        r.URL.Query().Get("video_id"),
		ChannelID:      r.URL.Query().Get("channel_id"),
		WebhookEventID: webhookEventID,
		UpdateType:     r.URL.Query().Get("update_type"),
		OrderBy:        r.URL.Query().Get("order_by"),
		OrderDir:       getOrderDir(r),
	}

	if filters.OrderBy == "" {
		filters.OrderBy = "created_at"
	}

	updates, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list video updates", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list video updates", nil)
		return
	}

	response := map[string]interface{}{
		"items":  updates,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

func (h *VideoUpdateHandler) handlePatch(w http.ResponseWriter, r *http.Request) {
	sendError(w, http.StatusForbidden, "Forbidden", "Video updates are immutable - they serve as an audit trail and cannot be modified", nil)
}

func (h *VideoUpdateHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	sendError(w, http.StatusForbidden, "Forbidden", "Video updates are immutable - they serve as an audit trail and cannot be deleted", nil)
}
