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

	// GetUpdateByID retrieves a single video update by ID.
	GetUpdateByID(ctx context.Context, id int64) (*models.VideoUpdate, error)

	// GetUpdatesByVideoID retrieves update history for a specific video.
	GetUpdatesByVideoID(ctx context.Context, videoID string, limit int) ([]*models.VideoUpdate, error)

	// GetUpdatesByChannelID retrieves update history for a specific channel.
	GetUpdatesByChannelID(ctx context.Context, channelID string, limit int) ([]*models.VideoUpdate, error)

	// GetRecentUpdates retrieves the most recent video updates across all videos.
	GetRecentUpdates(ctx context.Context, limit int) ([]*models.VideoUpdate, error)

	// List retrieves video updates with filters and pagination.
	List(ctx context.Context, filters *VideoUpdateFilters) ([]*models.VideoUpdate, int, error)
}

// VideoUpdateFilters contains filter options for listing video updates.
type VideoUpdateFilters struct {
	Limit          int
	Offset         int
	VideoID        string
	ChannelID      string
	WebhookEventID int64
	UpdateType     string
	OrderBy        string
	OrderDir       string
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

func (r *videoUpdateRepository) GetUpdateByID(ctx context.Context, id int64) (*models.VideoUpdate, error) {
	query := `
		SELECT id, webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at
		FROM video_updates
		WHERE id = $1
	`

	update := &models.VideoUpdate{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
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
		return nil, db.WrapError(err, "get video update by id")
	}

	return update, nil
}

func (r *videoUpdateRepository) List(ctx context.Context, filters *VideoUpdateFilters) ([]*models.VideoUpdate, int, error) {
	args := []interface{}{}
	argPos := 1
	whereClauses := []string{}

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

	if filters.WebhookEventID != 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("webhook_event_id = $%d", argPos))
		args = append(args, filters.WebhookEventID)
		argPos++
	}

	if filters.UpdateType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("update_type = $%d", argPos))
		args = append(args, filters.UpdateType)
		argPos++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM video_updates %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, db.WrapError(err, "count video updates")
	}

	orderBy := "created_at"
	if filters.OrderBy != "" {
		orderBy = filters.OrderBy
	}

	orderDir := "DESC"
	if filters.OrderDir != "" {
		orderDir = filters.OrderDir
	}

	query := fmt.Sprintf(`
		SELECT id, webhook_event_id, video_id, channel_id, title, published_at, feed_updated_at, update_type, created_at
		FROM video_updates
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDir, argPos, argPos+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, db.WrapError(err, "list video updates")
	}
	defer rows.Close()

	updates, err := scanVideoUpdates(rows)
	if err != nil {
		return nil, 0, err
	}

	return updates, total, nil
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
