package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"ad-tracker/youtube-webhook-ingestion/internal/service"
)

// ChannelFromURLHandler handles creating channels from YouTube URLs
type ChannelFromURLHandler struct {
	resolverService *service.ChannelResolverService
	logger          *slog.Logger
}

// NewChannelFromURLHandler creates a new ChannelFromURLHandler
func NewChannelFromURLHandler(resolverService *service.ChannelResolverService, logger *slog.Logger) *ChannelFromURLHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelFromURLHandler{
		resolverService: resolverService,
		logger:          logger,
	}
}

// CreateChannelFromURLRequest represents the request to create a channel from a URL
type CreateChannelFromURLRequest struct {
	URL         string `json:"url"`
	CallbackURL string `json:"callback_url,omitempty"`
}

// CreateChannelFromURLResponse represents the response from creating a channel from URL
type CreateChannelFromURLResponse struct {
	Channel      interface{} `json:"channel"`
	Subscription interface{} `json:"subscription,omitempty"`
	Enrichment   interface{} `json:"enrichment,omitempty"`
	WasExisting  bool        `json:"was_existing"`
	Message      string      `json:"message,omitempty"`
}

// HandleCreateFromURL handles POST /api/v1/channels/from-url
func (h *ChannelFromURLHandler) HandleCreateFromURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	// Parse request body
	var req CreateChannelFromURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body", "error", err)
		sendError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON in request body", nil)
		return
	}

	// Validate URL
	if req.URL == "" {
		sendError(w, http.StatusBadRequest, "invalid_request", "URL is required", nil)
		return
	}

	h.logger.Info("Creating channel from URL", "url", req.URL, "callback_url", req.CallbackURL)

	// Resolve channel from URL
	serviceReq := service.ResolveChannelFromURLRequest{
		URL:         req.URL,
		CallbackURL: req.CallbackURL,
	}

	result, err := h.resolverService.ResolveChannelFromURL(r.Context(), serviceReq)
	if err != nil {
		h.logger.Error("Failed to resolve channel from URL", "error", err, "url", req.URL)

		// Determine appropriate error message
		errMsg := "Failed to resolve channel from URL"
		statusCode := http.StatusInternalServerError

		// Check for specific error types
		if err.Error() == "channel not found" || err.Error() == "not a YouTube URL" {
			statusCode = http.StatusNotFound
			errMsg = err.Error()
		} else if err.Error() == "unsupported YouTube URL format" {
			statusCode = http.StatusBadRequest
			errMsg = "Unsupported YouTube URL format"
		}

		sendError(w, statusCode, "resolution_failed", errMsg, map[string]interface{}{
			"url":   req.URL,
			"error": err.Error(),
		})
		return
	}

	// Build response
	response := CreateChannelFromURLResponse{
		Channel:      result.Channel,
		Subscription: result.Subscription,
		Enrichment:   result.Enrichment,
		WasExisting:  result.WasExisting,
	}

	if result.WasExisting {
		response.Message = "Channel already exists and was returned"
	} else {
		response.Message = "Channel created successfully"
	}

	h.logger.Info("Channel resolved successfully",
		"channel_id", result.Channel.ChannelID,
		"was_existing", result.WasExisting,
		"has_subscription", result.Subscription != nil,
	)

	sendJSON(w, http.StatusOK, response)
}
