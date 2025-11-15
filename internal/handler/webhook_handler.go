package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/service"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebhookHandler handles webhook-related HTTP requests.
type WebhookHandler struct {
	webhookService *service.WebhookService
}

// NewWebhookHandler creates a new WebhookHandler instance.
func NewWebhookHandler(webhookService *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
	}
}

// HandleYouTubeWebhook processes incoming YouTube webhook notifications.
func (h *WebhookHandler) HandleYouTubeWebhook(c *gin.Context) {
	var payload models.WebhookPayloadDTO

	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log.Warn("Invalid request payload",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Status:    http.StatusBadRequest,
			Error:     "Bad Request",
			Message:   "Invalid request payload: " + err.Error(),
			Timestamp: time.Now(),
			Path:      c.Request.URL.Path,
		})
		return
	}

	// Extract client IP
	sourceIP := h.getClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	logger.Log.Info("Received webhook",
		zap.String("channelId", payload.ChannelID),
		zap.String("videoId", payload.VideoID),
		zap.String("eventType", payload.EventType),
		zap.String("sourceIp", sourceIP),
	)

	// Process webhook
	response, err := h.webhookService.ProcessWebhook(c.Request.Context(), &payload, sourceIP, userAgent)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, response)
}

// HealthCheck provides a simple health check endpoint for the webhook handler.
func (h *WebhookHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "Webhook receiver is healthy",
		"time":    time.Now(),
	})
}

func (h *WebhookHandler) getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := c.GetHeader("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return c.ClientIP()
}

func (h *WebhookHandler) handleError(c *gin.Context, err error) {
	switch err.(type) {
	case *service.ValidationError:
		logger.Log.Warn("Validation error",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Status:    http.StatusBadRequest,
			Error:     "Bad Request",
			Message:   err.Error(),
			Timestamp: time.Now(),
			Path:      c.Request.URL.Path,
		})
	case *service.ProcessingError:
		logger.Log.Error("Processing error",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Status:    http.StatusInternalServerError,
			Error:     "Internal Server Error",
			Message:   "Failed to process webhook event",
			Timestamp: time.Now(),
			Path:      c.Request.URL.Path,
		})
	default:
		logger.Log.Error("Unexpected error",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Status:    http.StatusInternalServerError,
			Error:     "Internal Server Error",
			Message:   "An unexpected error occurred",
			Timestamp: time.Now(),
			Path:      c.Request.URL.Path,
		})
	}
}
