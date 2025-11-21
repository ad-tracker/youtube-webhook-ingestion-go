package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

// EnrichmentHandler handles operations for video enrichments
type EnrichmentHandler struct {
	videoRepo       repository.EnrichmentRepository
	channelRepo     repository.ChannelEnrichmentRepository
	videoLookupRepo repository.VideoRepository
	queueClient     QueueClient
	logger          *slog.Logger
}

// QueueClient interface for enqueueing enrichment tasks
type QueueClient interface {
	EnqueueChannelEnrichment(ctx context.Context, channelID string) error
	EnqueueVideoEnrichment(ctx context.Context, videoID, channelID string, priority int) error
}

// NewEnrichmentHandler creates a new EnrichmentHandler
func NewEnrichmentHandler(
	videoRepo repository.EnrichmentRepository,
	channelRepo repository.ChannelEnrichmentRepository,
	videoLookupRepo repository.VideoRepository,
	logger *slog.Logger,
) *EnrichmentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrichmentHandler{
		videoRepo:       videoRepo,
		channelRepo:     channelRepo,
		videoLookupRepo: videoLookupRepo,
		logger:          logger,
	}
}

// SetQueueClient sets the queue client for enqueueing enrichment tasks
func (h *EnrichmentHandler) SetQueueClient(queueClient QueueClient) {
	h.queueClient = queueClient
}

// BatchEnrichmentRequest represents a request for multiple enrichments
type BatchEnrichmentRequest struct {
	IDs []string `json:"ids"`
}

// ServeHTTP routes enrichment requests
func (h *EnrichmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/enrichments")

	switch {
	case strings.HasPrefix(path, "/videos/"):
		// Handle both GET /videos/{id} and POST /videos/{id}/enqueue
		pathAfterVideos := strings.TrimPrefix(path, "/videos/")
		parts := strings.Split(pathAfterVideos, "/")

		if len(parts) == 1 && parts[0] != "" {
			// GET /videos/{id}
			h.getVideoEnrichment(w, r, parts[0])
			return
		} else if len(parts) == 2 && parts[0] != "" && parts[1] == "enqueue" {
			// POST /videos/{id}/enqueue
			h.enqueueVideoEnrichment(w, r, parts[0])
			return
		}
	case path == "/videos/batch" && r.Method == http.MethodPost:
		h.getBatchVideoEnrichments(w, r)
		return
	case strings.HasPrefix(path, "/channels/"):
		// Handle both GET /channels/{id} and POST /channels/{id}/enqueue
		pathAfterChannels := strings.TrimPrefix(path, "/channels/")
		parts := strings.Split(pathAfterChannels, "/")

		if len(parts) == 1 && parts[0] != "" {
			// GET /channels/{id}
			h.getChannelEnrichment(w, r, parts[0])
			return
		} else if len(parts) == 2 && parts[0] != "" && parts[1] == "enqueue" {
			// POST /channels/{id}/enqueue
			h.enqueueChannelEnrichment(w, r, parts[0])
			return
		}
	case path == "/channels/batch" && r.Method == http.MethodPost:
		h.getBatchChannelEnrichments(w, r)
		return
	}

	http.NotFound(w, r)
}

// getVideoEnrichment returns the latest enrichment for a video
func (h *EnrichmentHandler) getVideoEnrichment(w http.ResponseWriter, r *http.Request, videoID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enrichment, err := h.videoRepo.GetLatestEnrichment(r.Context(), videoID)
	if err == db.ErrNotFound {
		http.Error(w, "Enrichment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("Failed to get video enrichment",
			"video_id", videoID,
			"error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enrichment)
}

// getBatchVideoEnrichments returns enrichments for multiple videos
func (h *EnrichmentHandler) getBatchVideoEnrichments(w http.ResponseWriter, r *http.Request) {
	var req BatchEnrichmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		http.Error(w, "IDs are required", http.StatusBadRequest)
		return
	}

	enrichments, err := h.videoRepo.GetBatchLatestEnrichments(r.Context(), req.IDs)
	if err != nil {
		h.logger.Error("Failed to get batch video enrichments",
			"video_ids", req.IDs,
			"error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enrichments)
}

// getChannelEnrichment returns the latest enrichment for a channel
func (h *EnrichmentHandler) getChannelEnrichment(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enrichment, err := h.channelRepo.GetLatest(r.Context(), channelID)
	if err == db.ErrNotFound {
		http.Error(w, "Enrichment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("Failed to get channel enrichment",
			"channel_id", channelID,
			"error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enrichment)
}

// getBatchChannelEnrichments returns enrichments for multiple channels
func (h *EnrichmentHandler) getBatchChannelEnrichments(w http.ResponseWriter, r *http.Request) {
	var req BatchEnrichmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		http.Error(w, "IDs are required", http.StatusBadRequest)
		return
	}

	enrichments, err := h.channelRepo.GetBatchLatest(r.Context(), req.IDs)
	if err != nil {
		h.logger.Error("Failed to get batch channel enrichments",
			"channel_ids", req.IDs,
			"error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enrichments)
}

// enqueueChannelEnrichment enqueues a channel enrichment job
func (h *EnrichmentHandler) enqueueChannelEnrichment(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if queue client is available
	if h.queueClient == nil {
		h.logger.Error("Queue client not configured",
			"channel_id", channelID)
		http.Error(w, "Enrichment queue not available", http.StatusServiceUnavailable)
		return
	}

	// Enqueue the enrichment job
	if err := h.queueClient.EnqueueChannelEnrichment(r.Context(), channelID); err != nil {
		h.logger.Error("Failed to enqueue channel enrichment",
			"channel_id", channelID,
			"error", err)
		http.Error(w, "Failed to enqueue enrichment job", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Channel enrichment job enqueued",
		"channel_id", channelID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "queued",
		"channel_id": channelID,
	})
}

// enqueueVideoEnrichment enqueues a video enrichment job
func (h *EnrichmentHandler) enqueueVideoEnrichment(w http.ResponseWriter, r *http.Request, videoID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if queue client is available
	if h.queueClient == nil {
		h.logger.Error("Queue client not configured",
			"video_id", videoID)
		http.Error(w, "Enrichment queue not available", http.StatusServiceUnavailable)
		return
	}

	// Look up the video to get its channel_id
	video, err := h.videoLookupRepo.GetVideoByID(r.Context(), videoID)
	if err == db.ErrNotFound {
		h.logger.Warn("Video not found for enrichment",
			"video_id", videoID)
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("Failed to look up video",
			"video_id", videoID,
			"error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Enqueue the enrichment job
	if err := h.queueClient.EnqueueVideoEnrichment(r.Context(), videoID, video.ChannelID, 0); err != nil {
		h.logger.Error("Failed to enqueue video enrichment",
			"video_id", videoID,
			"channel_id", video.ChannelID,
			"error", err)
		http.Error(w, "Failed to enqueue enrichment job", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Video enrichment job enqueued",
		"video_id", videoID,
		"channel_id", video.ChannelID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "queued",
		"video_id": videoID,
	})
}
