package repository

import (
	"context"
	"fmt"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VideoUpdateRepository defines operations for managing video updates.
// Note: video_updates table is immutable - only inserts are allowed, no updates or deletes.
type VideoUpdateRepository interface {
	// CreateVideoUpdate inserts a new video update record.
	CreateVideoUpdate(ctx context.Context, update *models.VideoUpdate) error

	// GetUpdatesByVideoID retrieves update history for a specific video.
	GetUpdatesByVideoID(ctx context.Context, videoID string, limit int) ([]*models.VideoUpdate, error)

	// GetUpdatesByChannelID retrieves update history for a specific channel.
	GetUpdatesByChannelID(ctx context.Context, channelID string, limit int) ([]*models.VideoUpdate, error)

	// GetRecentUpdates retrieves the most recent video updates across all videos.
	GetRecentUpdates(ctx context.Context, limit int) ([]*models.VideoUpdate, error)
}

type videoUpdateRepository struct {
	pool *pgxpool.Pool
}

// NewVideoUpdateRepository creates a new VideoUpdateRepository.
func NewVideoUpdateRepository(pool *pgxpool.Pool) VideoUpdateRepository {
	return &videoUpdateRepository{pool: pool}
}

func (r *videoUpdateRepository) CreateVideoUpdate(ctx context.Context, update *models.VideoUpdate) error {
	query := `
		INSERT INTO video_updates (webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		update.WebhookEventID,
		update.VideoID,
		update.ChannelID,
		update.Title,
		update.PublishedAt,
		update.FeedUpdatedAt,
		update.UpdateType,
		update.CreatedAt,
	).Scan(&update.ID, &update.CreatedAt)

	if err != nil {
		return db.WrapError(err, "create video update")
	}

	return nil
}

func (r *videoUpdateRepository) GetUpdatesByVideoID(ctx context.Context, videoID string, limit int) ([]*models.VideoUpdate, error) {
	query := `
		SELECT id, webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at
		FROM video_updates
		WHERE video_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, videoID, limit)
	if err != nil {
		return nil, db.WrapError(err, "get updates by video id")
	}
	defer rows.Close()

	return scanVideoUpdates(rows)
}

func (r *videoUpdateRepository) GetUpdatesByChannelID(ctx context.Context, channelID string, limit int) ([]*models.VideoUpdate, error) {
	query := `
		SELECT id, webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at
		FROM video_updates
		WHERE channel_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, channelID, limit)
	if err != nil {
		return nil, db.WrapError(err, "get updates by channel id")
	}
	defer rows.Close()

	return scanVideoUpdates(rows)
}

func (r *videoUpdateRepository) GetRecentUpdates(ctx context.Context, limit int) ([]*models.VideoUpdate, error) {
	query := `
		SELECT id, webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at
		FROM video_updates
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, db.WrapError(err, "get recent updates")
	}
	defer rows.Close()

	return scanVideoUpdates(rows)
}

// Helper function to scan multiple video updates from query results
func scanVideoUpdates(rows pgx.Rows) ([]*models.VideoUpdate, error) {
	var updates []*models.VideoUpdate

	for rows.Next() {
		update := &models.VideoUpdate{}
		err := rows.Scan(
			&update.ID,
			&update.WebhookEventID,
			&update.VideoID,
			&update.ChannelID,
			&update.Title,
			&update.PublishedAt,
			&update.FeedUpdatedAt,
			&update.UpdateType,
			&update.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan video update: %w", err)
		}
		updates = append(updates, update)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video updates: %w", err)
	}

	return updates, nil
}
