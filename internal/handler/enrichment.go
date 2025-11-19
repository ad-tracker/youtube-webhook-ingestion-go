package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

// EnrichmentHandler handles operations for video enrichments
type EnrichmentHandler struct {
	videoRepo   repository.EnrichmentRepository
	channelRepo repository.ChannelEnrichmentRepository
	logger      *slog.Logger
}

// NewEnrichmentHandler creates a new EnrichmentHandler
func NewEnrichmentHandler(
	videoRepo repository.EnrichmentRepository,
	channelRepo repository.ChannelEnrichmentRepository,
	logger *slog.Logger,
) *EnrichmentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrichmentHandler{
		videoRepo:   videoRepo,
		channelRepo: channelRepo,
		logger:      logger,
	}
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
		videoID := strings.TrimPrefix(path, "/videos/")
		if videoID != "" && !strings.Contains(videoID, "/") {
			h.getVideoEnrichment(w, r, videoID)
			return
		}
	case path == "/videos/batch" && r.Method == http.MethodPost:
		h.getBatchVideoEnrichments(w, r)
		return
	case strings.HasPrefix(path, "/channels/"):
		channelID := strings.TrimPrefix(path, "/channels/")
		if channelID != "" && !strings.Contains(channelID, "/") {
			h.getChannelEnrichment(w, r, channelID)
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
