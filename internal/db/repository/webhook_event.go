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

	// Create inserts a new webhook event (for API).
	Create(ctx context.Context, event *models.WebhookEvent) error

	// GetUnprocessedEvents retrieves unprocessed webhook events, ordered by received_at.
	GetUnprocessedEvents(ctx context.Context, limit int) ([]*models.WebhookEvent, error)

	// MarkEventProcessed marks a webhook event as processed with an optional error message.
	// This is the only update operation allowed on webhook events.
	MarkEventProcessed(ctx context.Context, eventID int64, processingError string) error

	// UpdateProcessingStatus updates only the processing-related fields of a webhook event.
	UpdateProcessingStatus(ctx context.Context, eventID int64, processed bool, processingError string) error

	// GetEventByID retrieves a single webhook event by ID.
	GetEventByID(ctx context.Context, eventID int64) (*models.WebhookEvent, error)

	// GetEventsByVideoID retrieves all webhook events for a specific video.
	GetEventsByVideoID(ctx context.Context, videoID string) ([]*models.WebhookEvent, error)

	// List retrieves webhook events with filters and pagination.
	List(ctx context.Context, filters *WebhookEventFilters) ([]*models.WebhookEvent, int, error)
}

// WebhookEventFilters contains filter options for listing webhook events.
type WebhookEventFilters struct {
	Limit     int
	Offset    int
	Processed *bool
	VideoID   string
	ChannelID string
	OrderBy   string
	OrderDir  string
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

func (r *webhookEventRepository) Create(ctx context.Context, event *models.WebhookEvent) error {
	if event.ContentHash == "" {
		event.ContentHash = db.GenerateContentHash(event.RawXML)
	}

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
		return db.WrapError(err, "create webhook event")
	}

	return nil
}

func (r *webhookEventRepository) UpdateProcessingStatus(ctx context.Context, eventID int64, processed bool, processingError string) error {
	query := `
		UPDATE webhook_events
		SET processed = $1,
		    processed_at = CASE WHEN $1 = true THEN NOW() ELSE processed_at END,
		    processing_error = CASE WHEN $2 != '' THEN $2 ELSE NULL END
		WHERE id = $3
	`

	cmdTag, err := r.pool.Exec(ctx, query, processed, processingError, eventID)
	if err != nil {
		return db.WrapError(err, "update processing status")
	}

	if cmdTag.RowsAffected() == 0 {
		return db.WrapError(pgx.ErrNoRows, "update processing status")
	}

	return nil
}

func (r *webhookEventRepository) List(ctx context.Context, filters *WebhookEventFilters) ([]*models.WebhookEvent, int, error) {
	args := []interface{}{}
	argPos := 1

	whereClause := ""
	whereClauses := []string{}

	if filters.Processed != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("processed = $%d", argPos))
		args = append(args, *filters.Processed)
		argPos++
	}

	if filters.VideoID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("video_id = $%d", argPos))
		args = append(args, filters.VideoID)
		argPos++
	}

	if filters.ChannelID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("channel_id = $%d", argPos))
		args = append(args, filters.ChannelID)
		argPos++
	}

	if len(whereClauses) > 0 {
		whereClause = "WHERE " + fmt.Sprintf("%s", whereClauses[0])
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	orderBy := "received_at"
	if filters.OrderBy != "" {
		orderBy = filters.OrderBy
	}

	orderDir := "DESC"
	if filters.OrderDir != "" {
		orderDir = filters.OrderDir
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM webhook_events %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, db.WrapError(err, "count webhook events")
	}

	query := fmt.Sprintf(`
		SELECT id, raw_xml, content_hash, received_at, processed, processed_at,
		       processing_error, video_id, channel_id, created_at
		FROM webhook_events
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDir, argPos, argPos+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, db.WrapError(err, "list webhook events")
	}
	defer rows.Close()

	events, err := scanWebhookEvents(rows)
	if err != nil {
		return nil, 0, err
	}

	return events, total, nil
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
