package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VideoRepository defines operations for managing videos.
type VideoRepository interface {
	// UpsertVideo creates a new video or updates an existing one.
	UpsertVideo(ctx context.Context, video *models.Video) error

	// Create creates a new video.
	Create(ctx context.Context, video *models.Video) error

	// Update updates an existing video.
	Update(ctx context.Context, video *models.Video) error

	// Delete deletes a video by ID.
	Delete(ctx context.Context, videoID string) error

	// GetVideoByID retrieves a single video by ID.
	GetVideoByID(ctx context.Context, videoID string) (*models.Video, error)

	// GetVideosByChannelID retrieves all videos for a specific channel.
	GetVideosByChannelID(ctx context.Context, channelID string, limit int) ([]*models.Video, error)

	// ListVideos retrieves all videos with pagination.
	ListVideos(ctx context.Context, limit, offset int) ([]*models.Video, error)

	// List retrieves videos with filters and pagination.
	List(ctx context.Context, filters *VideoFilters) ([]*models.Video, int, error)

	// GetVideosByPublishedDate retrieves videos published since the given time.
	GetVideosByPublishedDate(ctx context.Context, since time.Time, limit int) ([]*models.Video, error)
}

// VideoFilters contains filter options for listing videos.
type VideoFilters struct {
	Limit           int
	Offset          int
	ChannelID       string
	Title           string
	PublishedAfter  *time.Time
	PublishedBefore *time.Time
	OrderBy         string
	OrderDir        string
}

type videoRepository struct {
	pool *pgxpool.Pool
}

// NewVideoRepository creates a new VideoRepository.
func NewVideoRepository(pool *pgxpool.Pool) VideoRepository {
	return &videoRepository{pool: pool}
}

func (r *videoRepository) UpsertVideo(ctx context.Context, video *models.Video) error {
	query := `
		INSERT INTO videos (video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (video_id) DO UPDATE
		SET title = EXCLUDED.title,
		    video_url = EXCLUDED.video_url,
		    published_at = EXCLUDED.published_at,
		    last_updated_at = EXCLUDED.last_updated_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING first_seen_at, last_updated_at, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		video.VideoID,
		video.ChannelID,
		video.Title,
		video.VideoURL,
		video.PublishedAt,
		video.FirstSeenAt,
		video.LastUpdatedAt,
		video.CreatedAt,
		video.UpdatedAt,
	).Scan(
		&video.FirstSeenAt,
		&video.LastUpdatedAt,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "upsert video")
	}

	return nil
}

func (r *videoRepository) GetVideoByID(ctx context.Context, videoID string) (*models.Video, error) {
	query := `
		SELECT video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at
		FROM videos
		WHERE video_id = $1
	`

	video := &models.Video{}
	err := r.pool.QueryRow(ctx, query, videoID).Scan(
		&video.VideoID,
		&video.ChannelID,
		&video.Title,
		&video.VideoURL,
		&video.PublishedAt,
		&video.FirstSeenAt,
		&video.LastUpdatedAt,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "get video by id")
	}

	return video, nil
}

func (r *videoRepository) GetVideosByChannelID(ctx context.Context, channelID string, limit int) ([]*models.Video, error) {
	query := `
		SELECT video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at
		FROM videos
		WHERE channel_id = $1
		ORDER BY published_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, channelID, limit)
	if err != nil {
		return nil, db.WrapError(err, "get videos by channel id")
	}
	defer rows.Close()

	return scanVideos(rows)
}

func (r *videoRepository) ListVideos(ctx context.Context, limit, offset int) ([]*models.Video, error) {
	query := `
		SELECT video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at
		FROM videos
		ORDER BY published_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "list videos")
	}
	defer rows.Close()

	return scanVideos(rows)
}

func (r *videoRepository) GetVideosByPublishedDate(ctx context.Context, since time.Time, limit int) ([]*models.Video, error) {
	query := `
		SELECT video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at
		FROM videos
		WHERE published_at >= $1
		ORDER BY published_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, since, limit)
	if err != nil {
		return nil, db.WrapError(err, "get videos by published date")
	}
	defer rows.Close()

	return scanVideos(rows)
}

func (r *videoRepository) Create(ctx context.Context, video *models.Video) error {
	query := `
		INSERT INTO videos (video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING first_seen_at, last_updated_at, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		video.VideoID,
		video.ChannelID,
		video.Title,
		video.VideoURL,
		video.PublishedAt,
		video.FirstSeenAt,
		video.LastUpdatedAt,
		video.CreatedAt,
		video.UpdatedAt,
	).Scan(
		&video.FirstSeenAt,
		&video.LastUpdatedAt,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create video")
	}

	return nil
}

func (r *videoRepository) Update(ctx context.Context, video *models.Video) error {
	now := time.Now()
	query := `
		UPDATE videos
		SET title = $1,
		    video_url = $2,
		    published_at = $3,
		    last_updated_at = $4,
		    updated_at = $5
		WHERE video_id = $6
		RETURNING channel_id, first_seen_at, last_updated_at, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		video.Title,
		video.VideoURL,
		video.PublishedAt,
		now,
		now,
		video.VideoID,
	).Scan(
		&video.ChannelID,
		&video.FirstSeenAt,
		&video.LastUpdatedAt,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "update video")
	}

	return nil
}

func (r *videoRepository) Delete(ctx context.Context, videoID string) error {
	query := `DELETE FROM videos WHERE video_id = $1`

	result, err := r.pool.Exec(ctx, query, videoID)
	if err != nil {
		return db.WrapError(err, "delete video")
	}

	if result.RowsAffected() == 0 {
		return db.WrapError(pgx.ErrNoRows, "delete video")
	}

	return nil
}

func (r *videoRepository) List(ctx context.Context, filters *VideoFilters) ([]*models.Video, int, error) {
	args := []interface{}{}
	argPos := 1
	whereClauses := []string{}

	if filters.ChannelID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("channel_id = $%d", argPos))
		args = append(args, filters.ChannelID)
		argPos++
	}

	if filters.Title != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("title ILIKE $%d", argPos))
		args = append(args, "%"+filters.Title+"%")
		argPos++
	}

	if filters.PublishedAfter != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("published_at >= $%d", argPos))
		args = append(args, *filters.PublishedAfter)
		argPos++
	}

	if filters.PublishedBefore != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("published_at <= $%d", argPos))
		args = append(args, *filters.PublishedBefore)
		argPos++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM videos %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, db.WrapError(err, "count videos")
	}

	orderBy := "published_at"
	if filters.OrderBy != "" {
		orderBy = filters.OrderBy
	}

	orderDir := "DESC"
	if filters.OrderDir != "" {
		orderDir = filters.OrderDir
	}

	query := fmt.Sprintf(`
		SELECT video_id, channel_id, title, video_url, published_at, first_seen_at, last_updated_at, created_at, updated_at
		FROM videos
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDir, argPos, argPos+1)

	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, db.WrapError(err, "list videos")
	}
	defer rows.Close()

	videos, err := scanVideos(rows)
	if err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

// Helper function to scan multiple videos from query results
func scanVideos(rows pgx.Rows) ([]*models.Video, error) {
	var videos []*models.Video

	for rows.Next() {
		video := &models.Video{}
		err := rows.Scan(
			&video.VideoID,
			&video.ChannelID,
			&video.Title,
			&video.VideoURL,
			&video.PublishedAt,
			&video.FirstSeenAt,
			&video.LastUpdatedAt,
			&video.CreatedAt,
			&video.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan video: %w", err)
		}
		videos = append(videos, video)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate videos: %w", err)
	}

	return videos, nil
}
