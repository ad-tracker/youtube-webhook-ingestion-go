// Package handler provides HTTP request handlers for the application.
package handler

import (
	"net/http"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/repository"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/service"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	repo      *repository.Repository
	publisher *service.MessagePublisher
}

// NewHealthHandler creates a new HealthHandler instance.
func NewHealthHandler(repo *repository.Repository, publisher *service.MessagePublisher) *HealthHandler {
	return &HealthHandler{
		repo:      repo,
		publisher: publisher,
	}
}

// LivenessProbe checks if the application is running.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "UP",
		"time":   time.Now(),
	})
}

// ReadinessProbe checks if the application is ready to serve traffic.
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database connectivity
	if err := h.repo.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "DOWN",
			"database": "unhealthy",
			"error":    err.Error(),
			"time":     time.Now(),
		})
		return
	}

	// Check RabbitMQ connectivity
	if !h.publisher.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "DOWN",
			"rabbitmq": "unhealthy",
			"time":     time.Now(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "UP",
		"database": "healthy",
		"rabbitmq": "healthy",
		"time":     time.Now(),
	})
}
