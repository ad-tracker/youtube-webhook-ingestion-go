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

// VideoRepository defines operations for managing videos.
type VideoRepository interface {
	// UpsertVideo creates a new video or updates an existing one.
	UpsertVideo(ctx context.Context, video *models.Video) error

	// GetVideoByID retrieves a single video by ID.
	GetVideoByID(ctx context.Context, videoID string) (*models.Video, error)

	// GetVideosByChannelID retrieves all videos for a specific channel.
	GetVideosByChannelID(ctx context.Context, channelID string, limit int) ([]*models.Video, error)

	// ListVideos retrieves all videos with pagination.
	ListVideos(ctx context.Context, limit, offset int) ([]*models.Video, error)

	// GetVideosByPublishedDate retrieves videos published since the given time.
	GetVideosByPublishedDate(ctx context.Context, since time.Time, limit int) ([]*models.Video, error)
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
