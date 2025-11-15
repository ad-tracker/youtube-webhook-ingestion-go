package models

import (
	"database/sql"
	"time"
)

// WebhookEvent represents a raw webhook notification event from YouTube PubSubHubbub.
// This table is immutable - events can only be created and marked as processed.
type WebhookEvent struct {
	ID              int64          `db:"id"`
	RawXML          string         `db:"raw_xml"`
	ContentHash     string         `db:"content_hash"`
	ReceivedAt      time.Time      `db:"received_at"`
	Processed       bool           `db:"processed"`
	ProcessedAt     sql.NullTime   `db:"processed_at"`
	ProcessingError sql.NullString `db:"processing_error"`
	VideoID         sql.NullString `db:"video_id"`
	ChannelID       sql.NullString `db:"channel_id"`
	CreatedAt       time.Time      `db:"created_at"`
}

// NewWebhookEvent creates a new WebhookEvent with the given raw XML and content hash.
// The videoID and channelID are extracted from the XML for indexing purposes.
func NewWebhookEvent(rawXML, contentHash, videoID, channelID string) *WebhookEvent {
	now := time.Now()
	return &WebhookEvent{
		RawXML:      rawXML,
		ContentHash: contentHash,
		ReceivedAt:  now,
		Processed:   false,
		VideoID:     sqlNullString(videoID),
		ChannelID:   sqlNullString(channelID),
		CreatedAt:   now,
	}
}

// MarkProcessed updates the event as processed with an optional error message.
func (w *WebhookEvent) MarkProcessed(processingError string) {
	w.Processed = true
	w.ProcessedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if processingError != "" {
		w.ProcessingError = sql.NullString{String: processingError, Valid: true}
	}
}

// Helper function to create sql.NullString
func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
