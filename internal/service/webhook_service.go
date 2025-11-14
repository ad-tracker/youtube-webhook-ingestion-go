// Package service provides business logic for webhook processing.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/repository"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/validation"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WebhookService handles webhook processing business logic.
type WebhookService struct {
	repo      *repository.Repository
	publisher *MessagePublisher
	validator *validation.Validator
}

// NewWebhookService creates a new WebhookService instance.
func NewWebhookService(repo *repository.Repository, publisher *MessagePublisher, validator *validation.Validator) *WebhookService {
	return &WebhookService{
		repo:      repo,
		publisher: publisher,
		validator: validator,
	}
}

// ProcessWebhook processes an incoming webhook event through the full pipeline.
func (ws *WebhookService) ProcessWebhook(ctx context.Context, payload *models.WebhookPayloadDTO, sourceIP, userAgent string) (*models.WebhookResponseDTO, error) {
	// Step 1: Validate payload
	if err := ws.validator.ValidatePayload(payload); err != nil {
		logger.Log.Warn("Webhook validation failed",
			zap.Error(err),
			zap.String("channelId", payload.ChannelID),
			zap.String("videoId", payload.VideoID),
		)
		return nil, &ValidationError{Message: err.Error()}
	}

	// Step 2: Create webhook event
	eventID := uuid.New()
	webhookEvent := &models.WebhookEvent{
		ID:               eventID,
		VideoID:          payload.VideoID,
		ChannelID:        payload.ChannelID,
		EventType:        payload.EventType,
		Payload:          ws.serializePayload(payload),
		SourceIP:         sourceIP,
		UserAgent:        userAgent,
		Processed:        false,
		ProcessingStatus: models.ProcessingStatusPending,
		RetryCount:       0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Step 3: Persist webhook event
	if err := ws.repo.CreateWebhookEvent(ctx, webhookEvent); err != nil {
		logger.Log.Error("Failed to persist webhook event",
			zap.Error(err),
			zap.String("eventId", eventID.String()),
		)
		return nil, &ProcessingError{Message: "failed to persist event", Cause: err}
	}

	logger.Log.Info("Webhook event persisted",
		zap.String("eventId", eventID.String()),
		zap.String("channelId", payload.ChannelID),
		zap.String("videoId", payload.VideoID),
		zap.String("eventType", payload.EventType),
	)

	// Step 4: Create immutable event (audit trail)
	event := &models.Event{
		EventType:  payload.EventType,
		ChannelID:  payload.ChannelID,
		VideoID:    payload.VideoID,
		RawXML:     payload.Content,
		EventHash:  repository.ComputeEventHash(payload.Content),
		ReceivedAt: time.Now(),
	}

	// Check for duplicate
	exists, err := ws.repo.EventExistsByHash(ctx, event.EventHash)
	if err != nil {
		logger.Log.Error("Failed to check event hash",
			zap.Error(err),
			zap.String("eventId", eventID.String()),
		)
	}
	if err == nil && exists {
		logger.Log.Info("Duplicate event detected (skipping insert)",
			zap.String("eventHash", event.EventHash),
			zap.String("channelId", payload.ChannelID),
		)
	}
	if err == nil && !exists {
		// Insert if not duplicate
		if createErr := ws.repo.CreateEvent(ctx, event); createErr != nil {
			logger.Log.Error("Failed to create immutable event",
				zap.Error(createErr),
				zap.String("eventId", eventID.String()),
			)
		} else {
			logger.Log.Debug("Immutable event created",
				zap.String("eventId", event.ID.String()),
				zap.String("eventHash", event.EventHash),
			)
		}
	}

	// Step 5: Publish to RabbitMQ
	if err := ws.publisher.PublishEvent(ctx, webhookEvent); err != nil {
		logger.Log.Error("Failed to publish event to RabbitMQ",
			zap.Error(err),
			zap.String("eventId", eventID.String()),
		)

		// Update status to failed
		errMsg := fmt.Sprintf("Publishing failed: %v", err)
		if updateErr := ws.repo.UpdateWebhookEventStatus(ctx, eventID, models.ProcessingStatusFailed, &errMsg); updateErr != nil {
			logger.Log.Error("Failed to update webhook event status",
				zap.Error(updateErr),
				zap.String("eventId", eventID.String()),
			)
		}

		return nil, &ProcessingError{Message: "failed to publish event", Cause: err}
	}

	// Step 6: Update status to completed
	if err := ws.repo.UpdateWebhookEventStatus(ctx, eventID, models.ProcessingStatusCompleted, nil); err != nil {
		logger.Log.Error("Failed to update webhook event status",
			zap.Error(err),
			zap.String("eventId", eventID.String()),
		)
	}

	logger.Log.Info("Webhook processing completed",
		zap.String("eventId", eventID.String()),
		zap.String("channelId", payload.ChannelID),
		zap.String("status", string(models.ProcessingStatusCompleted)),
	)

	return &models.WebhookResponseDTO{
		EventID:    eventID,
		Status:     "ACCEPTED",
		Message:    "Webhook event processed successfully",
		ReceivedAt: time.Now(),
	}, nil
}

func (ws *WebhookService) serializePayload(payload *models.WebhookPayloadDTO) string {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error("Failed to serialize payload", zap.Error(err))
		return ""
	}
	return string(data)
}

// Custom errors

// ValidationError represents a webhook payload validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ProcessingError represents an error that occurred during webhook processing.
//
//nolint:govet // fieldalignment: Accept minor memory overhead for better readability
type ProcessingError struct {
	Message string
	Cause   error
}

func (e *ProcessingError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}
