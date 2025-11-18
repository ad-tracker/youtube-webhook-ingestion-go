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

	// Create creates a new channel.
	Create(ctx context.Context, channel *models.Channel) error

	// Update updates an existing channel.
	Update(ctx context.Context, channel *models.Channel) error

	// Delete deletes a channel by ID.
	Delete(ctx context.Context, channelID string) error

	// GetChannelByID retrieves a single channel by ID.
	GetChannelByID(ctx context.Context, channelID string) (*models.Channel, error)

	// ListChannels retrieves all channels with pagination.
	ListChannels(ctx context.Context, limit, offset int) ([]*models.Channel, error)

	// List retrieves channels with filters and pagination.
	List(ctx context.Context, filters *ChannelFilters) ([]*models.Channel, int, error)

	// GetChannelsByLastUpdated retrieves channels that have been updated since the given time.
	GetChannelsByLastUpdated(ctx context.Context, since time.Time, limit int) ([]*models.Channel, error)
}

// ChannelFilters contains filter options for listing channels.
type ChannelFilters struct {
	Limit    int
	Offset   int
	Title    string
	OrderBy  string
	OrderDir string
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

func (r *channelRepository) Create(ctx context.Context, channel *models.Channel) error {
	query := `
		INSERT INTO channels (channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
		return db.WrapError(err, "create channel")
	}

	return nil
}

func (r *channelRepository) Update(ctx context.Context, channel *models.Channel) error {
	now := time.Now()
	query := `
		UPDATE channels
		SET title = $1,
		    channel_url = $2,
		    last_updated_at = $3,
		    updated_at = $4
		WHERE channel_id = $5
		RETURNING first_seen_at, last_updated_at, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		channel.Title,
		channel.ChannelURL,
		now,
		now,
		channel.ChannelID,
	).Scan(
		&channel.FirstSeenAt,
		&channel.LastUpdatedAt,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "update channel")
	}

	return nil
}

func (r *channelRepository) Delete(ctx context.Context, channelID string) error {
	query := `DELETE FROM channels WHERE channel_id = $1`

	result, err := r.pool.Exec(ctx, query, channelID)
	if err != nil {
		return db.WrapError(err, "delete channel")
	}

	if result.RowsAffected() == 0 {
		return db.WrapError(pgx.ErrNoRows, "delete channel")
	}

	return nil
}

func (r *channelRepository) List(ctx context.Context, filters *ChannelFilters) ([]*models.Channel, int, error) {
	args := []interface{}{}
	argPos := 1

	whereClause := ""
	if filters.Title != "" {
		whereClause = fmt.Sprintf("WHERE title ILIKE $%d", argPos)
		args = append(args, "%"+filters.Title+"%")
		argPos++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM channels %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, db.WrapError(err, "count channels")
	}

	orderBy := "last_updated_at"
	if filters.OrderBy != "" {
		orderBy = filters.OrderBy
	}

	orderDir := "DESC"
	if filters.OrderDir != "" {
		orderDir = filters.OrderDir
	}

	query := fmt.Sprintf(`
		SELECT channel_id, title, channel_url, first_seen_at, last_updated_at, created_at, updated_at
		FROM channels
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDir, argPos, argPos+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, db.WrapError(err, "list channels")
	}
	defer rows.Close()

	channels, err := scanChannels(rows)
	if err != nil {
		return nil, 0, err
	}

	return channels, total, nil
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
