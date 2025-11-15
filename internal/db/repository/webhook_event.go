package repository

import (
	"context"
	"fmt"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WebhookEventRepository defines operations for managing webhook events.
// Note: webhook_events table is immutable - only inserts and marking as processed are allowed.
type WebhookEventRepository interface {
	// CreateWebhookEvent inserts a new webhook event.
	CreateWebhookEvent(ctx context.Context, rawXML, videoID, channelID string) (*models.WebhookEvent, error)

	// GetUnprocessedEvents retrieves unprocessed webhook events, ordered by received_at.
	GetUnprocessedEvents(ctx context.Context, limit int) ([]*models.WebhookEvent, error)

	// MarkEventProcessed marks a webhook event as processed with an optional error message.
	// This is the only update operation allowed on webhook events.
	MarkEventProcessed(ctx context.Context, eventID int64, processingError string) error

	// GetEventByID retrieves a single webhook event by ID.
	GetEventByID(ctx context.Context, eventID int64) (*models.WebhookEvent, error)

	// GetEventsByVideoID retrieves all webhook events for a specific video.
	GetEventsByVideoID(ctx context.Context, videoID string) ([]*models.WebhookEvent, error)
}

type webhookEventRepository struct {
	pool *pgxpool.Pool
}

// NewWebhookEventRepository creates a new WebhookEventRepository.
func NewWebhookEventRepository(pool *pgxpool.Pool) WebhookEventRepository {
	return &webhookEventRepository{pool: pool}
}

func (r *webhookEventRepository) CreateWebhookEvent(ctx context.Context, rawXML, videoID, channelID string) (*models.WebhookEvent, error) {
	contentHash := db.GenerateContentHash(rawXML)
	event := models.NewWebhookEvent(rawXML, contentHash, videoID, channelID)

	query := `
		INSERT INTO webhook_events (raw_xml, content_hash, received_at, processed, video_id, channel_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, received_at, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		event.RawXML,
		event.ContentHash,
		event.ReceivedAt,
		event.Processed,
		event.VideoID,
		event.ChannelID,
		event.CreatedAt,
	).Scan(&event.ID, &event.ReceivedAt, &event.CreatedAt)

	if err != nil {
		return nil, db.WrapError(err, "create webhook event")
	}

	return event, nil
}

func (r *webhookEventRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]*models.WebhookEvent, error) {
	query := `
		SELECT id, raw_xml, content_hash, received_at, processed, processed_at,
		       processing_error, video_id, channel_id, created_at
		FROM webhook_events
		WHERE NOT processed
		ORDER BY received_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, db.WrapError(err, "get unprocessed events")
	}
	defer rows.Close()

	return scanWebhookEvents(rows)
}

func (r *webhookEventRepository) MarkEventProcessed(ctx context.Context, eventID int64, processingError string) error {
	query := `
		UPDATE webhook_events
		SET processed = true,
		    processed_at = NOW(),
		    processing_error = CASE WHEN $2 != '' THEN $2 ELSE NULL END
		WHERE id = $1
	`

	cmdTag, err := r.pool.Exec(ctx, query, eventID, processingError)
	if err != nil {
		return db.WrapError(err, "mark event processed")
	}

	if cmdTag.RowsAffected() == 0 {
		return db.WrapError(pgx.ErrNoRows, "mark event processed")
	}

	return nil
}

func (r *webhookEventRepository) GetEventByID(ctx context.Context, eventID int64) (*models.WebhookEvent, error) {
	query := `
		SELECT id, raw_xml, content_hash, received_at, processed, processed_at,
		       processing_error, video_id, channel_id, created_at
		FROM webhook_events
		WHERE id = $1
	`

	event := &models.WebhookEvent{}
	err := r.pool.QueryRow(ctx, query, eventID).Scan(
		&event.ID,
		&event.RawXML,
		&event.ContentHash,
		&event.ReceivedAt,
		&event.Processed,
		&event.ProcessedAt,
		&event.ProcessingError,
		&event.VideoID,
		&event.ChannelID,
		&event.CreatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "get event by id")
	}

	return event, nil
}

func (r *webhookEventRepository) GetEventsByVideoID(ctx context.Context, videoID string) ([]*models.WebhookEvent, error) {
	query := `
		SELECT id, raw_xml, content_hash, received_at, processed, processed_at,
		       processing_error, video_id, channel_id, created_at
		FROM webhook_events
		WHERE video_id = $1
		ORDER BY received_at DESC
	`

	rows, err := r.pool.Query(ctx, query, videoID)
	if err != nil {
		return nil, db.WrapError(err, "get events by video id")
	}
	defer rows.Close()

	return scanWebhookEvents(rows)
}

// Helper function to scan multiple webhook events from query results
func scanWebhookEvents(rows pgx.Rows) ([]*models.WebhookEvent, error) {
	var events []*models.WebhookEvent

	for rows.Next() {
		event := &models.WebhookEvent{}
		err := rows.Scan(
			&event.ID,
			&event.RawXML,
			&event.ContentHash,
			&event.ReceivedAt,
			&event.Processed,
			&event.ProcessedAt,
			&event.ProcessingError,
			&event.VideoID,
			&event.ChannelID,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook events: %w", err)
	}

	return events, nil
}
