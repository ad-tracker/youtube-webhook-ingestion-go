package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
)

var (
	videoIDRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	channelIDRegex = regexp.MustCompile(`^UC[a-zA-Z0-9_-]{22}$`)
	eventTypeRegex = regexp.MustCompile(`^[A-Z_]+$`)
)

type Validator struct {
	maxPayloadSize    int64
	validationEnabled bool
}

func New(maxPayloadSize int64, enabled bool) *Validator {
	return &Validator{
		maxPayloadSize:    maxPayloadSize,
		validationEnabled: enabled,
	}
}

func (v *Validator) ValidatePayload(payload *models.WebhookPayloadDTO) error {
	if !v.validationEnabled {
		return nil
	}

	// Validate video ID format
	if payload.VideoID != "" && !videoIDRegex.MatchString(payload.VideoID) {
		return fmt.Errorf("invalid video ID format: %s", payload.VideoID)
	}

	// Validate channel ID format
	if !channelIDRegex.MatchString(payload.ChannelID) {
		return fmt.Errorf("invalid channel ID format: %s", payload.ChannelID)
	}

	// Validate event type format
	if !eventTypeRegex.MatchString(payload.EventType) {
		return fmt.Errorf("invalid event type format: %s", payload.EventType)
	}

	// Validate timestamp (not in future, not too old)
	now := time.Now().Unix()
	payloadTime := payload.Timestamp / 1000 // Convert from milliseconds

	if payloadTime > now+60 {
		return fmt.Errorf("timestamp is in the future")
	}

	// Allow events up to 7 days old
	if payloadTime < now-7*24*60*60 {
		return fmt.Errorf("timestamp is too old (>7 days)")
	}

	// Validate content size
	if int64(len(payload.Content)) > v.maxPayloadSize {
		return fmt.Errorf("payload content exceeds maximum size of %d bytes", v.maxPayloadSize)
	}

	return nil
}

func (v *Validator) IsValidVideoID(videoID string) bool {
	return videoIDRegex.MatchString(videoID)
}

func (v *Validator) IsValidChannelID(channelID string) bool {
	return channelIDRegex.MatchString(channelID)
}

func (v *Validator) IsValidEventType(eventType string) bool {
	return eventTypeRegex.MatchString(eventType)
}
