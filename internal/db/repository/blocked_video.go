package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
)

// BlockedVideoRepository defines operations for managing blocked videos.
type BlockedVideoRepository interface {
	CreateBlockedVideo(ctx context.Context, videoID, reason string, createdBy *string) (*models.BlockedVideo, error)
	DeleteBlockedVideo(ctx context.Context, videoID string) error
	GetBlockedVideo(ctx context.Context, videoID string) (*models.BlockedVideo, error)
	ListBlockedVideos(ctx context.Context, limit, offset int) ([]*models.BlockedVideo, int, error)
	GetAllBlockedVideoIDs(ctx context.Context) ([]string, error)
}

type blockedVideoRepository struct {
	pool *pgxpool.Pool
}

// NewBlockedVideoRepository creates a new BlockedVideoRepository.
func NewBlockedVideoRepository(pool *pgxpool.Pool) BlockedVideoRepository {
	return &blockedVideoRepository{pool: pool}
}

// CreateBlockedVideo adds a video to the block list.
func (r *blockedVideoRepository) CreateBlockedVideo(ctx context.Context, videoID, reason string, createdBy *string) (*models.BlockedVideo, error) {
	query := `
		INSERT INTO blocked_videos (video_id, reason, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, video_id, reason, created_at, created_by
	`

	var bv models.BlockedVideo
	var createdByNull sql.NullString
	if createdBy != nil {
		createdByNull = sql.NullString{String: *createdBy, Valid: true}
	}

	err := r.pool.QueryRow(ctx, query, videoID, reason, createdByNull).Scan(
		&bv.ID,
		&bv.VideoID,
		&bv.Reason,
		&bv.CreatedAt,
		&bv.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blocked video: %w", err)
	}

	return &bv, nil
}

// DeleteBlockedVideo removes a video from the block list.
func (r *blockedVideoRepository) DeleteBlockedVideo(ctx context.Context, videoID string) error {
	query := `DELETE FROM blocked_videos WHERE video_id = $1`

	result, err := r.pool.Exec(ctx, query, videoID)
	if err != nil {
		return fmt.Errorf("failed to delete blocked video: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("blocked video not found: %s", videoID)
	}

	return nil
}

// GetBlockedVideo retrieves a single blocked video by video ID.
func (r *blockedVideoRepository) GetBlockedVideo(ctx context.Context, videoID string) (*models.BlockedVideo, error) {
	query := `
		SELECT id, video_id, reason, created_at, created_by
		FROM blocked_videos
		WHERE video_id = $1
	`

	var bv models.BlockedVideo
	err := r.pool.QueryRow(ctx, query, videoID).Scan(
		&bv.ID,
		&bv.VideoID,
		&bv.Reason,
		&bv.CreatedAt,
		&bv.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked video: %w", err)
	}

	return &bv, nil
}

// ListBlockedVideos retrieves a paginated list of blocked videos.
func (r *blockedVideoRepository) ListBlockedVideos(ctx context.Context, limit, offset int) ([]*models.BlockedVideo, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM blocked_videos`
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count blocked videos: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, video_id, reason, created_at, created_by
		FROM blocked_videos
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list blocked videos: %w", err)
	}
	defer rows.Close()

	var blockedVideos []*models.BlockedVideo
	for rows.Next() {
		var bv models.BlockedVideo
		err := rows.Scan(
			&bv.ID,
			&bv.VideoID,
			&bv.Reason,
			&bv.CreatedAt,
			&bv.CreatedBy,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan blocked video: %w", err)
		}
		blockedVideos = append(blockedVideos, &bv)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating blocked videos: %w", err)
	}

	return blockedVideos, total, nil
}

// GetAllBlockedVideoIDs retrieves all blocked video IDs (for cache loading).
func (r *blockedVideoRepository) GetAllBlockedVideoIDs(ctx context.Context) ([]string, error) {
	query := `SELECT video_id FROM blocked_videos`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all blocked video IDs: %w", err)
	}
	defer rows.Close()

	var videoIDs []string
	for rows.Next() {
		var videoID string
		if err := rows.Scan(&videoID); err != nil {
			return nil, fmt.Errorf("failed to scan video ID: %w", err)
		}
		videoIDs = append(videoIDs, videoID)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video IDs: %w", err)
	}

	return videoIDs, nil
}
