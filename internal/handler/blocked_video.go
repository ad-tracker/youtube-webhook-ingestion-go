package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"
)

// BlockedVideoHandler handles CRUD operations for blocked videos.
type BlockedVideoHandler struct {
	repo         repository.BlockedVideoRepository
	blockedCache *service.BlockedVideoCache
	logger       *slog.Logger
}

// NewBlockedVideoHandler creates a new blocked video handler.
func NewBlockedVideoHandler(repo repository.BlockedVideoRepository, blockedCache *service.BlockedVideoCache, logger *slog.Logger) *BlockedVideoHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BlockedVideoHandler{
		repo:         repo,
		blockedCache: blockedCache,
		logger:       logger,
	}
}

// CreateBlockedVideoRequest represents a request to block a video.
type CreateBlockedVideoRequest struct {
	VideoID   string  `json:"video_id"`
	Reason    string  `json:"reason"`
	CreatedBy *string `json:"created_by,omitempty"`
}

// ServeHTTP routes blocked video requests.
func (h *BlockedVideoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/blocked-videos")

	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodPost:
			h.CreateBlockedVideo(w, r)
		case http.MethodGet:
			h.ListBlockedVideos(w, r)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	if strings.HasPrefix(path, "/") {
		videoID := strings.TrimPrefix(path, "/")

		switch r.Method {
		case http.MethodGet:
			h.GetBlockedVideoByPath(w, r, videoID)
		case http.MethodDelete:
			h.DeleteBlockedVideoByPath(w, r, videoID)
		default:
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		}
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

// CreateBlockedVideo handles POST /api/v1/blocked-videos
func (h *BlockedVideoHandler) CreateBlockedVideo(w http.ResponseWriter, r *http.Request) {
	var req CreateBlockedVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error(), nil)
		return
	}

	// Validate required fields
	if req.VideoID == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "video_id is required", nil)
		return
	}
	if req.Reason == "" {
		sendError(w, http.StatusBadRequest, "validation failed", "reason is required", nil)
		return
	}

	// Create blocked video in database
	blockedVideo, err := h.repo.CreateBlockedVideo(r.Context(), req.VideoID, req.Reason, req.CreatedBy)
	if err != nil {
		h.logger.Error("failed to create blocked video", "error", err, "video_id", req.VideoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to create blocked video", nil)
		return
	}

	// Add to cache
	if err := h.blockedCache.Add(r.Context(), req.VideoID); err != nil {
		h.logger.Error("failed to add video to blocked cache", "error", err, "video_id", req.VideoID)
		// Don't return error - database write succeeded, cache will sync eventually
	}

	h.logger.Info("blocked video created", "video_id", req.VideoID, "id", blockedVideo.ID)

	sendJSON(w, http.StatusCreated, blockedVideo)
}

// ListBlockedVideos handles GET /api/v1/blocked-videos
func (h *BlockedVideoHandler) ListBlockedVideos(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	// Get blocked videos from database
	blockedVideos, total, err := h.repo.ListBlockedVideos(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list blocked videos", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list blocked videos", nil)
		return
	}

	// Build response
	response := map[string]interface{}{
		"data":   blockedVideos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

// GetBlockedVideoByPath handles GET /api/v1/blocked-videos/{video_id}
func (h *BlockedVideoHandler) GetBlockedVideoByPath(w http.ResponseWriter, r *http.Request, videoID string) {
	blockedVideo, err := h.repo.GetBlockedVideo(r.Context(), videoID)
	if err != nil {
		h.logger.Error("failed to get blocked video", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to get blocked video", nil)
		return
	}

	if blockedVideo == nil {
		sendError(w, http.StatusNotFound, "not found", "blocked video not found", nil)
		return
	}

	sendJSON(w, http.StatusOK, blockedVideo)
}

// DeleteBlockedVideoByPath handles DELETE /api/v1/blocked-videos/{video_id}
func (h *BlockedVideoHandler) DeleteBlockedVideoByPath(w http.ResponseWriter, r *http.Request, videoID string) {
	// Delete from database
	if err := h.repo.DeleteBlockedVideo(r.Context(), videoID); err != nil {
		h.logger.Error("failed to delete blocked video", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to delete blocked video", nil)
		return
	}

	// Remove from cache
	if err := h.blockedCache.Remove(r.Context(), videoID); err != nil {
		h.logger.Error("failed to remove video from blocked cache", "error", err, "video_id", videoID)
		// Don't return error - database deletion succeeded, cache will sync eventually
	}

	h.logger.Info("blocked video deleted", "video_id", videoID)

	w.WriteHeader(http.StatusNoContent)
}
