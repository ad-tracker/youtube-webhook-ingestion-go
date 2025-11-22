package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"

	"github.com/google/uuid"
)

// SponsorHandler handles REST API operations for sponsors and sponsor detection.
type SponsorHandler struct {
	sponsorRepo repository.SponsorDetectionRepository
	videoRepo   repository.VideoRepository
	logger      *slog.Logger
}

// NewSponsorHandler creates a new SponsorHandler.
func NewSponsorHandler(
	sponsorRepo repository.SponsorDetectionRepository,
	videoRepo repository.VideoRepository,
	logger *slog.Logger,
) *SponsorHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SponsorHandler{
		sponsorRepo: sponsorRepo,
		videoRepo:   videoRepo,
		logger:      logger,
	}
}

// ServeHTTP routes sponsor-related requests.
func (h *SponsorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sponsors")

	// GET /api/v1/sponsors
	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			h.handleListSponsors(w, r)
			return
		}
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		return
	}

	// GET /api/v1/sponsors/{id}
	// GET /api/v1/sponsors/{id}/videos
	if strings.HasPrefix(path, "/") {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		sponsorID := parts[0]

		// Parse and validate sponsor ID
		sponsorUUID, err := uuid.Parse(sponsorID)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid sponsor ID", "sponsor ID must be a valid UUID", nil)
			return
		}

		// GET /api/v1/sponsors/{id}/videos
		if len(parts) == 2 && parts[1] == "videos" {
			if r.Method == http.MethodGet {
				h.handleGetSponsorVideos(w, r, sponsorUUID)
				return
			}
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
			return
		}

		// GET /api/v1/sponsors/{id}
		if len(parts) == 1 {
			if r.Method == http.MethodGet {
				h.handleGetSponsor(w, r, sponsorUUID)
				return
			}
			sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
			return
		}
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

// handleListSponsors handles GET /api/v1/sponsors
func (h *SponsorHandler) handleListSponsors(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	// Parse sort_by parameter (default: video_count)
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "video_count"
	}

	// Validate sort_by parameter
	validSortOptions := map[string]string{
		"video_count": "video_count",
		"name":        "name",
		"last_seen":   "last_seen",
		"created":     "created",
	}
	if _, ok := validSortOptions[sortBy]; !ok {
		sendError(w, http.StatusBadRequest, "validation failed",
			fmt.Sprintf("invalid sort_by value '%s' (valid: video_count, name, last_seen, created)", sortBy), nil)
		return
	}

	// Parse order parameter (default: desc for video_count, asc for name)
	order := strings.ToLower(r.URL.Query().Get("order"))
	if order == "" {
		if sortBy == "name" {
			order = "asc"
		} else {
			order = "desc"
		}
	}

	// Validate order parameter
	if order != "asc" && order != "desc" {
		sendError(w, http.StatusBadRequest, "validation failed",
			"invalid order value (valid: asc, desc)", nil)
		return
	}

	// Get optional category filter
	category := r.URL.Query().Get("category")

	// Fetch sponsors from repository
	sponsors, err := h.sponsorRepo.ListSponsors(r.Context(), sortBy, limit, offset)
	if err != nil {
		h.logger.Error("failed to list sponsors", "error", err)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list sponsors", nil)
		return
	}

	// Filter by category if specified
	filteredSponsors := sponsors
	if category != "" {
		filteredSponsors = make([]*models.Sponsor, 0)
		for _, s := range sponsors {
			if s.Category != nil && strings.EqualFold(*s.Category, category) {
				filteredSponsors = append(filteredSponsors, s)
			}
		}
	}

	// Reverse order if asc (repository always returns desc for video_count, name uses ASC in repo)
	if (sortBy == "video_count" || sortBy == "last_seen" || sortBy == "created") && order == "asc" {
		// Reverse the slice
		for i, j := 0, len(filteredSponsors)-1; i < j; i, j = i+1, j-1 {
			filteredSponsors[i], filteredSponsors[j] = filteredSponsors[j], filteredSponsors[i]
		}
	} else if sortBy == "name" && order == "desc" {
		// Reverse for name when desc is requested (repo returns ASC by default)
		for i, j := 0, len(filteredSponsors)-1; i < j; i, j = i+1, j-1 {
			filteredSponsors[i], filteredSponsors[j] = filteredSponsors[j], filteredSponsors[i]
		}
	}

	response := map[string]interface{}{
		"items":  filteredSponsors,
		"total":  len(filteredSponsors),
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

// handleGetSponsor handles GET /api/v1/sponsors/{id}
func (h *SponsorHandler) handleGetSponsor(w http.ResponseWriter, r *http.Request, sponsorID uuid.UUID) {
	sponsor, err := h.sponsorRepo.GetSponsorByID(r.Context(), sponsorID)
	if err != nil {
		h.logger.Error("failed to get sponsor", "error", err, "sponsor_id", sponsorID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve sponsor", nil)
		return
	}

	if sponsor == nil {
		sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("sponsor with id '%s' not found", sponsorID), nil)
		return
	}

	sendJSON(w, http.StatusOK, sponsor)
}

// handleGetSponsorVideos handles GET /api/v1/sponsors/{id}/videos
func (h *SponsorHandler) handleGetSponsorVideos(w http.ResponseWriter, r *http.Request, sponsorID uuid.UUID) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	// Verify sponsor exists first
	sponsor, err := h.sponsorRepo.GetSponsorByID(r.Context(), sponsorID)
	if err != nil {
		h.logger.Error("failed to get sponsor", "error", err, "sponsor_id", sponsorID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve sponsor", nil)
		return
	}

	if sponsor == nil {
		sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("sponsor with id '%s' not found", sponsorID), nil)
		return
	}

	// Get video-sponsor relationships
	videoSponsors, err := h.sponsorRepo.GetSponsorVideos(r.Context(), sponsorID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get sponsor videos", "error", err, "sponsor_id", sponsorID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve sponsor videos", nil)
		return
	}

	// Enrich with video details and sponsor info
	videoSponsorDetails := make([]map[string]interface{}, 0, len(videoSponsors))
	for _, vs := range videoSponsors {
		// Get video details
		video, err := h.videoRepo.GetVideoByID(r.Context(), vs.VideoID)
		if err != nil {
			h.logger.Warn("failed to get video details for sponsor video",
				"error", err,
				"video_id", vs.VideoID,
				"sponsor_id", sponsorID,
			)
			// Continue with basic info if video lookup fails
			videoSponsorDetails = append(videoSponsorDetails, map[string]interface{}{
				"id":               vs.ID,
				"video_id":         vs.VideoID,
				"sponsor_id":       vs.SponsorID,
				"sponsor_name":     sponsor.Name,
				"sponsor_category": sponsor.Category,
				"confidence":       vs.Confidence,
				"evidence":         vs.Evidence,
				"detected_at":      vs.DetectedAt,
			})
			continue
		}

		detail := map[string]interface{}{
			"id":               vs.ID,
			"video_id":         vs.VideoID,
			"video_title":      video.Title,
			"video_url":        video.VideoURL,
			"channel_id":       video.ChannelID,
			"published_at":     video.PublishedAt,
			"sponsor_id":       vs.SponsorID,
			"sponsor_name":     sponsor.Name,
			"sponsor_category": sponsor.Category,
			"confidence":       vs.Confidence,
			"evidence":         vs.Evidence,
			"detected_at":      vs.DetectedAt,
		}
		videoSponsorDetails = append(videoSponsorDetails, detail)
	}

	response := map[string]interface{}{
		"items":  videoSponsorDetails,
		"total":  len(videoSponsorDetails),
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

// VideoSponsorHandler handles REST API operations for video-sponsor relationships.
type VideoSponsorHandler struct {
	sponsorRepo repository.SponsorDetectionRepository
	logger      *slog.Logger
}

// NewVideoSponsorHandler creates a new VideoSponsorHandler.
func NewVideoSponsorHandler(
	sponsorRepo repository.SponsorDetectionRepository,
	logger *slog.Logger,
) *VideoSponsorHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &VideoSponsorHandler{
		sponsorRepo: sponsorRepo,
		logger:      logger,
	}
}

// HandleGetVideoSponsors handles GET /api/v1/videos/{id}/sponsors
func (h *VideoSponsorHandler) HandleGetVideoSponsors(w http.ResponseWriter, r *http.Request, videoID string) {
	videoSponsorDetails, err := h.sponsorRepo.GetVideoSponsorsWithDetails(r.Context(), videoID)
	if err != nil {
		h.logger.Error("failed to get video sponsors", "error", err, "video_id", videoID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve video sponsors", nil)
		return
	}

	response := map[string]interface{}{
		"items": videoSponsorDetails,
		"total": len(videoSponsorDetails),
	}

	sendJSON(w, http.StatusOK, response)
}

// ChannelSponsorHandler handles getting sponsors for a channel.
type ChannelSponsorHandler struct {
	sponsorRepo repository.SponsorDetectionRepository
	videoRepo   repository.VideoRepository
	logger      *slog.Logger
}

// NewChannelSponsorHandler creates a new ChannelSponsorHandler.
func NewChannelSponsorHandler(
	sponsorRepo repository.SponsorDetectionRepository,
	videoRepo repository.VideoRepository,
	logger *slog.Logger,
) *ChannelSponsorHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelSponsorHandler{
		sponsorRepo: sponsorRepo,
		videoRepo:   videoRepo,
		logger:      logger,
	}
}

// HandleGetChannelSponsors handles GET /api/v1/channels/{id}/sponsors
func (h *ChannelSponsorHandler) HandleGetChannelSponsors(w http.ResponseWriter, r *http.Request, channelID string) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	// Get sponsors for this channel using the new repository method
	sponsors, err := h.sponsorRepo.GetSponsorsByChannelID(r.Context(), channelID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get channel sponsors", "error", err, "channel_id", channelID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve channel sponsors", nil)
		return
	}

	response := map[string]interface{}{
		"items":  sponsors,
		"total":  len(sponsors),
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

// SponsorDetectionJobHandler handles REST API operations for sponsor detection jobs.
type SponsorDetectionJobHandler struct {
	sponsorRepo repository.SponsorDetectionRepository
	logger      *slog.Logger
}

// NewSponsorDetectionJobHandler creates a new SponsorDetectionJobHandler.
func NewSponsorDetectionJobHandler(
	sponsorRepo repository.SponsorDetectionRepository,
	logger *slog.Logger,
) *SponsorDetectionJobHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SponsorDetectionJobHandler{
		sponsorRepo: sponsorRepo,
		logger:      logger,
	}
}

// ServeHTTP routes sponsor detection job requests.
func (h *SponsorDetectionJobHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sponsor-detection-jobs")

	// GET /api/v1/sponsor-detection-jobs
	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			h.handleListJobs(w, r)
			return
		}
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		return
	}

	// GET /api/v1/sponsor-detection-jobs/{id}
	if strings.HasPrefix(path, "/") {
		jobID := strings.TrimPrefix(path, "/")

		// Parse and validate job ID
		jobUUID, err := uuid.Parse(jobID)
		if err != nil {
			sendError(w, http.StatusBadRequest, "invalid job ID", "job ID must be a valid UUID", nil)
			return
		}

		if r.Method == http.MethodGet {
			h.handleGetJob(w, r, jobUUID)
			return
		}
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		return
	}

	sendError(w, http.StatusNotFound, "not found", "", nil)
}

// handleListJobs handles GET /api/v1/sponsor-detection-jobs
func (h *SponsorDetectionJobHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	offset := parseOffset(r)

	// Get optional filters
	videoID := r.URL.Query().Get("video_id")
	status := r.URL.Query().Get("status")

	// Validate status if provided
	if status != "" {
		validStatuses := map[string]bool{
			"pending":   true,
			"completed": true,
			"failed":    true,
			"skipped":   true,
		}
		if !validStatuses[status] {
			sendError(w, http.StatusBadRequest, "validation failed",
				"invalid status value (valid: pending, completed, failed, skipped)", nil)
			return
		}
	}

	var jobs []*models.SponsorDetectionJob
	var err error

	// If video_id filter is provided, use GetDetectionJobsByVideoID
	if videoID != "" {
		jobs, err = h.sponsorRepo.GetDetectionJobsByVideoID(r.Context(), videoID)
		if err != nil {
			h.logger.Error("failed to get detection jobs by video ID", "error", err, "video_id", videoID)
			sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve detection jobs", nil)
			return
		}

		// Apply status filter if specified
		if status != "" {
			filteredJobs := make([]*models.SponsorDetectionJob, 0)
			for _, job := range jobs {
				if job.Status == status {
					filteredJobs = append(filteredJobs, job)
				}
			}
			jobs = filteredJobs
		}

		// Apply pagination manually for filtered results
		start := offset
		end := offset + limit
		if start > len(jobs) {
			jobs = []*models.SponsorDetectionJob{}
		} else {
			if end > len(jobs) {
				end = len(jobs)
			}
			jobs = jobs[start:end]
		}
	} else {
		// For now, without a generic List method, we'll return an error or empty list
		// This would require a new repository method to support full listing with filters
		h.logger.Warn("listing all detection jobs without video_id filter is not implemented")
		jobs = []*models.SponsorDetectionJob{}
	}

	response := map[string]interface{}{
		"items":  jobs,
		"total":  len(jobs),
		"limit":  limit,
		"offset": offset,
	}

	sendJSON(w, http.StatusOK, response)
}

// handleGetJob handles GET /api/v1/sponsor-detection-jobs/{id}
func (h *SponsorDetectionJobHandler) handleGetJob(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	job, err := h.sponsorRepo.GetDetectionJobByID(r.Context(), jobID)
	if err != nil {
		h.logger.Error("failed to get detection job", "error", err, "job_id", jobID)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to retrieve detection job", nil)
		return
	}

	if job == nil {
		sendError(w, http.StatusNotFound, "not found", fmt.Sprintf("detection job with id '%s' not found", jobID), nil)
		return
	}

	sendJSON(w, http.StatusOK, job)
}
