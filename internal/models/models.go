// Package models contains the data models and DTOs for the YouTube webhook ingestion service.
package models

import (
	"time"

	"github.com/google/uuid"
)

type ProcessingStatus string

const (
	ProcessingStatusPending   ProcessingStatus = "PENDING"
	ProcessingStatusCompleted ProcessingStatus = "COMPLETED"
	ProcessingStatusFailed    ProcessingStatus = "FAILED"
)

type SubscriptionStatus string

const (
	SubscriptionStatusPending SubscriptionStatus = "PENDING"
	SubscriptionStatusActive  SubscriptionStatus = "ACTIVE"
	SubscriptionStatusExpired SubscriptionStatus = "EXPIRED"
	SubscriptionStatusFailed  SubscriptionStatus = "FAILED"
)

// WebhookEvent represents transient webhook processing state
type WebhookEvent struct {
	ID               uuid.UUID        `json:"id"`
	VideoID          string           `json:"video_id"`
	ChannelID        string           `json:"channel_id"`
	EventType        string           `json:"event_type"`
	Payload          string           `json:"payload"`
	SourceIP         string           `json:"source_ip"`
	UserAgent        string           `json:"user_agent"`
	Processed        bool             `json:"processed"`
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	ErrorMessage     *string          `json:"error_message"`
	RetryCount       int              `json:"retry_count"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	ProcessedAt      *time.Time       `json:"processed_at"`
}

// Event represents immutable audit trail
type Event struct {
	ID          uuid.UUID `json:"id"`
	EventType   string    `json:"event_type"`
	ChannelID   string    `json:"channel_id"`
	VideoID     string    `json:"video_id"`
	RawXML      string    `json:"raw_xml"`
	EventHash   string    `json:"event_hash"`
	ReceivedAt  time.Time `json:"received_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// Subscription represents PubSubHubbub subscription management
type Subscription struct {
	ID                 uuid.UUID          `json:"id"`
	ChannelID          string             `json:"channel_id"`
	TopicURL           string             `json:"topic_url"`
	CallbackURL        string             `json:"callback_url"`
	SubscriptionStatus SubscriptionStatus `json:"subscription_status"`
	LeaseSeconds       int                `json:"lease_seconds"`
	LeaseExpiresAt     *time.Time         `json:"lease_expires_at"`
	NextRenewalAt      *time.Time         `json:"next_renewal_at"`
	LastRenewedAt      *time.Time         `json:"last_renewed_at"`
	RenewalAttempts    int                `json:"renewal_attempts"`
	LastRenewalError   *string            `json:"last_renewal_error"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// WebhookPayloadDTO represents the webhook request
type WebhookPayloadDTO struct {
	VideoID   string `json:"videoId" binding:"required,max=50"`
	ChannelID string `json:"channelId" binding:"required,max=50"`
	EventType string `json:"eventType" binding:"required,max=50"`
	Content   string `json:"content"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp" binding:"required"`
}

// WebhookResponseDTO represents the webhook response
type WebhookResponseDTO struct {
	EventID    uuid.UUID `json:"eventId"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Status    int       `json:"status"`
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
}
