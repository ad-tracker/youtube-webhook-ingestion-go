package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

const (
	maxJobLimit = 500
)

// EnrichmentJobHandler handles operations for enrichment jobs
type EnrichmentJobHandler struct {
	repo   repository.EnrichmentJobRepository
	logger *slog.Logger
}

// NewEnrichmentJobHandler creates a new EnrichmentJobHandler
func NewEnrichmentJobHandler(repo repository.EnrichmentJobRepository, logger *slog.Logger) *EnrichmentJobHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrichmentJobHandler{
		repo:   repo,
		logger: logger,
	}
}

// ServeHTTP handles GET /api/v1/jobs requests
func (h *EnrichmentJobHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "", nil)
		return
	}

	h.handleList(w, r)
}

func (h *EnrichmentJobHandler) handleList(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := parseJobLimit(r)
	offset := parseOffset(r)
	status := r.URL.Query().Get("status")

	// Build filters
	filters := repository.JobFilters{
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	// Fetch jobs from repository
	jobs, total, err := h.repo.ListJobs(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list enrichment jobs",
			"error", err,
			"filters", filters,
		)
		sendError(w, http.StatusInternalServerError, "internal server error", "failed to list enrichment jobs", nil)
		return
	}

	// Return paginated response
	response := map[string]interface{}{
		"items":  jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}
	sendJSON(w, http.StatusOK, response)
}

// parseJobLimit parses the limit query parameter with a default of 100 and max of 500
func parseJobLimit(r *http.Request) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return 100
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return 100
	}

	if limit > maxJobLimit {
		return maxJobLimit
	}

	return limit
}
