package repository

import (
	"context"
	"fmt"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChannelRepository defines operations for managing channels.
type ChannelRepository interface {
	// UpsertChannel creates a new channel or updates an existing one.
	UpsertChannel(ctx context.Context, channel *models.Channel) error

	// GetChannelByID retrieves a single channel by ID.
	GetChannelByID(ctx context.Context, channelID string) (*models.Channel, error)

	// ListChannels retrieves all channels with pagination.
	ListChannels(ctx context.Context, limit, offset int) ([]*models.Channel, error)

	// GetChannelsByLastUpdated retrieves channels that have been updated since the given time.
	GetChannelsByLastUpdated(ctx context.Context, since time.Time, limit int) ([]*models.Channel, error)
}

type channelRepository struct {
	pool *pgxpool.Pool
}

// NewChannelRepository creates a new ChannelRepository.
func NewChannelRepository(pool *pgxpool.Pool) ChannelRepository {
	return &channelRepository{pool: pool}
}

func (r *channelRepository) UpsertChannel(ctx context.Context, channel *models.Channel) error {
	query := `
		INSERT INTO channels (channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (channel_id) DO UPDATE
		SET title = EXCLUDED.title,
		    channel_url = EXCLUDED.channel_url,
		    last_updated_at = EXCLUDED.last_updated_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING first_seen_at, last_updated_at, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		channel.ChannelID,
		channel.Title,
		channel.ChannelURL,
		channel.FirstSeenAt,
		channel.LastUpdatedAt,
		channel.CreatedAt,
		channel.UpdatedAt,
	).Scan(
		&channel.FirstSeenAt,
		&channel.LastUpdatedAt,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "upsert channel")
	}

	return nil
}

func (r *channelRepository) GetChannelByID(ctx context.Context, channelID string) (*models.Channel, error) {
	query := `
		SELECT channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at
		FROM channels
		WHERE channel_id = $1
	`

	channel := &models.Channel{}
	err := r.pool.QueryRow(ctx, query, channelID).Scan(
		&channel.ChannelID,
		&channel.Title,
		&channel.ChannelURL,
		&channel.FirstSeenAt,
		&channel.LastUpdatedAt,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "get channel by id")
	}

	return channel, nil
}

func (r *channelRepository) ListChannels(ctx context.Context, limit, offset int) ([]*models.Channel, error) {
	query := `
		SELECT channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at
		FROM channels
		ORDER BY last_updated_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "list channels")
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (r *channelRepository) GetChannelsByLastUpdated(ctx context.Context, since time.Time, limit int) ([]*models.Channel, error) {
	query := `
		SELECT channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at
		FROM channels
		WHERE last_updated_at >= $1
		ORDER BY last_updated_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, since, limit)
	if err != nil {
		return nil, db.WrapError(err, "get channels by last updated")
	}
	defer rows.Close()

	return scanChannels(rows)
}

// Helper function to scan multiple channels from query results
func scanChannels(rows pgx.Rows) ([]*models.Channel, error) {
	var channels []*models.Channel

	for rows.Next() {
		channel := &models.Channel{}
		err := rows.Scan(
			&channel.ChannelID,
			&channel.Title,
			&channel.ChannelURL,
			&channel.FirstSeenAt,
			&channel.LastUpdatedAt,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, channel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}

	return channels, nil
}
