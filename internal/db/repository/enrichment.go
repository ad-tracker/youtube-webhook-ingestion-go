package repository

import (
	"context"
	"encoding/json"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnrichmentRepository defines operations for managing video enrichments
type EnrichmentRepository interface {
	// CreateEnrichment stores a new video enrichment
	CreateEnrichment(ctx context.Context, enrichment *model.VideoEnrichment) error

	// GetLatestEnrichment retrieves the most recent enrichment for a video
	GetLatestEnrichment(ctx context.Context, videoID string) (*model.VideoEnrichment, error)

	// GetEnrichmentHistory retrieves all enrichments for a video
	GetEnrichmentHistory(ctx context.Context, videoID string, limit int) ([]*model.VideoEnrichment, error)

	// GetUnenrichedVideos retrieves videos that haven't been enriched yet
	GetUnenrichedVideos(ctx context.Context, limit int) ([]string, error)

	// GetVideosNeedingReenrichment retrieves videos whose enrichment is older than the given duration
	GetVideosNeedingReenrichment(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)

	// GetEnrichmentCount returns the total number of enrichments
	GetEnrichmentCount(ctx context.Context) (int64, error)

	// GetEnrichedVideoCount returns the number of unique videos that have been enriched
	GetEnrichedVideoCount(ctx context.Context) (int64, error)

	// GetBatchLatestEnrichments retrieves the most recent enrichment for multiple videos
	GetBatchLatestEnrichments(ctx context.Context, videoIDs []string) (map[string]*model.VideoEnrichment, error)
}

type enrichmentRepository struct {
	pool *pgxpool.Pool
}

// NewEnrichmentRepository creates a new EnrichmentRepository
func NewEnrichmentRepository(pool *pgxpool.Pool) EnrichmentRepository {
	return &enrichmentRepository{pool: pool}
}

func (r *enrichmentRepository) CreateEnrichment(ctx context.Context, enrichment *model.VideoEnrichment) error {
	query := `
		INSERT INTO video_api_enrichments (
			video_id, description, duration, dimension, definition, caption,
			licensed_content, projection,
			thumbnail_default_url, thumbnail_default_width, thumbnail_default_height,
			thumbnail_medium_url, thumbnail_medium_width, thumbnail_medium_height,
			thumbnail_high_url, thumbnail_high_width, thumbnail_high_height,
			thumbnail_standard_url, thumbnail_standard_width, thumbnail_standard_height,
			thumbnail_maxres_url, thumbnail_maxres_width, thumbnail_maxres_height,
			view_count, like_count, dislike_count, favorite_count, comment_count,
			category_id, tags, default_language, default_audio_language, topic_categories,
			privacy_status, license, embeddable, public_stats_viewable,
			made_for_kids, self_declared_made_for_kids,
			upload_status, failure_reason, rejection_reason,
			live_broadcast_content, scheduled_start_time, actual_start_time,
			actual_end_time, concurrent_viewers,
			location_description, location_latitude, location_longitude,
			content_rating, channel_title,
			enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23,
			$24, $25, $26, $27, $28,
			$29, $30, $31, $32, $33,
			$34, $35, $36, $37, $38, $39,
			$40, $41, $42,
			$43, $44, $45, $46, $47,
			$48, $49, $50,
			$51, $52,
			$53, $54, $55, $56, $57,
			$58, $59
		)
		RETURNING id, enriched_at, created_at, updated_at
	`

	// Convert arrays and maps to JSON
	tagsJSON, _ := json.Marshal(enrichment.Tags)
	topicCategoriesJSON, _ := json.Marshal(enrichment.TopicCategories)
	apiPartsJSON, _ := json.Marshal(enrichment.APIPartsRequested)
	contentRatingJSON, _ := json.Marshal(enrichment.ContentRating)
	rawAPIResponseJSON, _ := json.Marshal(enrichment.RawAPIResponse)

	now := time.Now()
	enrichedAt := now
	if !enrichment.EnrichedAt.IsZero() {
		enrichedAt = enrichment.EnrichedAt
	}

	err := r.pool.QueryRow(ctx, query,
		enrichment.VideoID,
		enrichment.Description,
		enrichment.Duration,
		enrichment.Dimension,
		enrichment.Definition,
		enrichment.Caption,
		enrichment.LicensedContent,
		enrichment.Projection,
		// Thumbnails
		enrichment.ThumbnailDefaultURL, enrichment.ThumbnailDefaultWidth, enrichment.ThumbnailDefaultHeight,
		enrichment.ThumbnailMediumURL, enrichment.ThumbnailMediumWidth, enrichment.ThumbnailMediumHeight,
		enrichment.ThumbnailHighURL, enrichment.ThumbnailHighWidth, enrichment.ThumbnailHighHeight,
		enrichment.ThumbnailStandardURL, enrichment.ThumbnailStandardWidth, enrichment.ThumbnailStandardHeight,
		enrichment.ThumbnailMaxresURL, enrichment.ThumbnailMaxresWidth, enrichment.ThumbnailMaxresHeight,
		// Engagement
		enrichment.ViewCount, enrichment.LikeCount, enrichment.DislikeCount,
		enrichment.FavoriteCount, enrichment.CommentCount,
		// Categorization
		enrichment.CategoryID, tagsJSON, enrichment.DefaultLanguage,
		enrichment.DefaultAudioLanguage, topicCategoriesJSON,
		// Content classification
		enrichment.PrivacyStatus, enrichment.License, enrichment.Embeddable,
		enrichment.PublicStatsViewable, enrichment.MadeForKids, enrichment.SelfDeclaredMadeForKids,
		// Upload details
		enrichment.UploadStatus, enrichment.FailureReason, enrichment.RejectionReason,
		// Live streaming
		enrichment.LiveBroadcastContent, enrichment.ScheduledStartTime, enrichment.ActualStartTime,
		enrichment.ActualEndTime, enrichment.ConcurrentViewers,
		// Location
		enrichment.LocationDescription, enrichment.LocationLatitude, enrichment.LocationLongitude,
		// Content rating and channel
		contentRatingJSON, enrichment.ChannelTitle,
		// API metadata
		enrichedAt, enrichment.APIResponseEtag, enrichment.QuotaCost,
		apiPartsJSON, rawAPIResponseJSON,
		// Timestamps
		now, now,
	).Scan(
		&enrichment.ID,
		&enrichment.EnrichedAt,
		&enrichment.CreatedAt,
		&enrichment.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create enrichment")
	}

	return nil
}

func (r *enrichmentRepository) GetLatestEnrichment(ctx context.Context, videoID string) (*model.VideoEnrichment, error) {
	query := `
		SELECT
			id, video_id, description, duration, dimension, definition, caption,
			licensed_content, projection,
			thumbnail_default_url, thumbnail_default_width, thumbnail_default_height,
			thumbnail_medium_url, thumbnail_medium_width, thumbnail_medium_height,
			thumbnail_high_url, thumbnail_high_width, thumbnail_high_height,
			thumbnail_standard_url, thumbnail_standard_width, thumbnail_standard_height,
			thumbnail_maxres_url, thumbnail_maxres_width, thumbnail_maxres_height,
			view_count, like_count, dislike_count, favorite_count, comment_count,
			category_id, tags, default_language, default_audio_language, topic_categories,
			privacy_status, license, embeddable, public_stats_viewable,
			made_for_kids, self_declared_made_for_kids,
			upload_status, failure_reason, rejection_reason,
			live_broadcast_content, scheduled_start_time, actual_start_time,
			actual_end_time, concurrent_viewers,
			location_description, location_latitude, location_longitude,
			content_rating, channel_title,
			enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
			created_at, updated_at
		FROM video_api_enrichments
		WHERE video_id = $1
		ORDER BY enriched_at DESC
		LIMIT 1
	`

	enrichment := &model.VideoEnrichment{}
	var tagsJSON, topicCategoriesJSON, apiPartsJSON, contentRatingJSON, rawAPIResponseJSON []byte

	err := r.pool.QueryRow(ctx, query, videoID).Scan(
		&enrichment.ID, &enrichment.VideoID,
		&enrichment.Description, &enrichment.Duration, &enrichment.Dimension,
		&enrichment.Definition, &enrichment.Caption, &enrichment.LicensedContent, &enrichment.Projection,
		// Thumbnails
		&enrichment.ThumbnailDefaultURL, &enrichment.ThumbnailDefaultWidth, &enrichment.ThumbnailDefaultHeight,
		&enrichment.ThumbnailMediumURL, &enrichment.ThumbnailMediumWidth, &enrichment.ThumbnailMediumHeight,
		&enrichment.ThumbnailHighURL, &enrichment.ThumbnailHighWidth, &enrichment.ThumbnailHighHeight,
		&enrichment.ThumbnailStandardURL, &enrichment.ThumbnailStandardWidth, &enrichment.ThumbnailStandardHeight,
		&enrichment.ThumbnailMaxresURL, &enrichment.ThumbnailMaxresWidth, &enrichment.ThumbnailMaxresHeight,
		// Engagement
		&enrichment.ViewCount, &enrichment.LikeCount, &enrichment.DislikeCount,
		&enrichment.FavoriteCount, &enrichment.CommentCount,
		// Categorization
		&enrichment.CategoryID, &tagsJSON, &enrichment.DefaultLanguage,
		&enrichment.DefaultAudioLanguage, &topicCategoriesJSON,
		// Content classification
		&enrichment.PrivacyStatus, &enrichment.License, &enrichment.Embeddable,
		&enrichment.PublicStatsViewable, &enrichment.MadeForKids, &enrichment.SelfDeclaredMadeForKids,
		// Upload details
		&enrichment.UploadStatus, &enrichment.FailureReason, &enrichment.RejectionReason,
		// Live streaming
		&enrichment.LiveBroadcastContent, &enrichment.ScheduledStartTime, &enrichment.ActualStartTime,
		&enrichment.ActualEndTime, &enrichment.ConcurrentViewers,
		// Location
		&enrichment.LocationDescription, &enrichment.LocationLatitude, &enrichment.LocationLongitude,
		// Content rating and channel
		&contentRatingJSON, &enrichment.ChannelTitle,
		// API metadata
		&enrichment.EnrichedAt, &enrichment.APIResponseEtag, &enrichment.QuotaCost,
		&apiPartsJSON, &rawAPIResponseJSON,
		// Timestamps
		&enrichment.CreatedAt, &enrichment.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, db.WrapError(err, "get latest enrichment")
	}

	// Unmarshal JSON fields
	json.Unmarshal(tagsJSON, &enrichment.Tags)
	json.Unmarshal(topicCategoriesJSON, &enrichment.TopicCategories)
	json.Unmarshal(apiPartsJSON, &enrichment.APIPartsRequested)
	json.Unmarshal(contentRatingJSON, &enrichment.ContentRating)
	json.Unmarshal(rawAPIResponseJSON, &enrichment.RawAPIResponse)

	return enrichment, nil
}

func (r *enrichmentRepository) GetEnrichmentHistory(ctx context.Context, videoID string, limit int) ([]*model.VideoEnrichment, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, video_id, enriched_at, quota_cost,
			view_count, like_count, comment_count,
			privacy_status, upload_status,
			created_at
		FROM video_api_enrichments
		WHERE video_id = $1
		ORDER BY enriched_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, videoID, limit)
	if err != nil {
		return nil, db.WrapError(err, "get enrichment history")
	}
	defer rows.Close()

	var enrichments []*model.VideoEnrichment
	for rows.Next() {
		e := &model.VideoEnrichment{}
		err := rows.Scan(
			&e.ID, &e.VideoID, &e.EnrichedAt, &e.QuotaCost,
			&e.ViewCount, &e.LikeCount, &e.CommentCount,
			&e.PrivacyStatus, &e.UploadStatus,
			&e.CreatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan enrichment history")
		}
		enrichments = append(enrichments, e)
	}

	return enrichments, nil
}

func (r *enrichmentRepository) GetUnenrichedVideos(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT v.video_id
		FROM videos v
		LEFT JOIN video_api_enrichments e ON v.video_id = e.video_id
		WHERE e.video_id IS NULL
		ORDER BY v.first_seen_at DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, db.WrapError(err, "get unenriched videos")
	}
	defer rows.Close()

	var videoIDs []string
	for rows.Next() {
		var videoID string
		if err := rows.Scan(&videoID); err != nil {
			return nil, db.WrapError(err, "scan video ID")
		}
		videoIDs = append(videoIDs, videoID)
	}

	return videoIDs, nil
}

func (r *enrichmentRepository) GetVideosNeedingReenrichment(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}

	cutoffTime := time.Now().Add(-olderThan)

	query := `
		SELECT DISTINCT ON (v.video_id) v.video_id
		FROM videos v
		INNER JOIN video_api_enrichments e ON v.video_id = e.video_id
		WHERE e.enriched_at < $1
		ORDER BY v.video_id, e.enriched_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, cutoffTime, limit)
	if err != nil {
		return nil, db.WrapError(err, "get videos needing re-enrichment")
	}
	defer rows.Close()

	var videoIDs []string
	for rows.Next() {
		var videoID string
		if err := rows.Scan(&videoID); err != nil {
			return nil, db.WrapError(err, "scan video ID")
		}
		videoIDs = append(videoIDs, videoID)
	}

	return videoIDs, nil
}

func (r *enrichmentRepository) GetEnrichmentCount(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM video_api_enrichments`
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, db.WrapError(err, "get enrichment count")
	}
	return count, nil
}

func (r *enrichmentRepository) GetEnrichedVideoCount(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(DISTINCT video_id) FROM video_api_enrichments`
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, db.WrapError(err, "get enriched video count")
	}
	return count, nil
}

func (r *enrichmentRepository) GetBatchLatestEnrichments(ctx context.Context, videoIDs []string) (map[string]*model.VideoEnrichment, error) {
	if len(videoIDs) == 0 {
		return make(map[string]*model.VideoEnrichment), nil
	}

	query := `
		WITH latest_enrichments AS (
			SELECT DISTINCT ON (video_id)
				id, video_id, description, duration, dimension, definition, caption,
				licensed_content, projection,
				thumbnail_default_url, thumbnail_default_width, thumbnail_default_height,
				thumbnail_medium_url, thumbnail_medium_width, thumbnail_medium_height,
				thumbnail_high_url, thumbnail_high_width, thumbnail_high_height,
				thumbnail_standard_url, thumbnail_standard_width, thumbnail_standard_height,
				thumbnail_maxres_url, thumbnail_maxres_width, thumbnail_maxres_height,
				view_count, like_count, dislike_count, favorite_count, comment_count,
				category_id, tags, default_language, default_audio_language, topic_categories,
				privacy_status, license, embeddable, public_stats_viewable,
				made_for_kids, self_declared_made_for_kids,
				upload_status, failure_reason, rejection_reason,
				live_broadcast_content, scheduled_start_time, actual_start_time,
				actual_end_time, concurrent_viewers,
				location_description, location_latitude, location_longitude,
				content_rating, channel_title,
				enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
				created_at, updated_at
			FROM video_api_enrichments
			WHERE video_id = ANY($1)
			ORDER BY video_id, enriched_at DESC
		)
		SELECT * FROM latest_enrichments
	`

	rows, err := r.pool.Query(ctx, query, videoIDs)
	if err != nil {
		return nil, db.WrapError(err, "get batch latest enrichments")
	}
	defer rows.Close()

	enrichments := make(map[string]*model.VideoEnrichment)
	for rows.Next() {
		enrichment := &model.VideoEnrichment{}
		var tagsJSON, topicCategoriesJSON, apiPartsJSON, contentRatingJSON, rawAPIResponseJSON []byte

		err := rows.Scan(
			&enrichment.ID, &enrichment.VideoID,
			&enrichment.Description, &enrichment.Duration, &enrichment.Dimension,
			&enrichment.Definition, &enrichment.Caption, &enrichment.LicensedContent, &enrichment.Projection,
			// Thumbnails
			&enrichment.ThumbnailDefaultURL, &enrichment.ThumbnailDefaultWidth, &enrichment.ThumbnailDefaultHeight,
			&enrichment.ThumbnailMediumURL, &enrichment.ThumbnailMediumWidth, &enrichment.ThumbnailMediumHeight,
			&enrichment.ThumbnailHighURL, &enrichment.ThumbnailHighWidth, &enrichment.ThumbnailHighHeight,
			&enrichment.ThumbnailStandardURL, &enrichment.ThumbnailStandardWidth, &enrichment.ThumbnailStandardHeight,
			&enrichment.ThumbnailMaxresURL, &enrichment.ThumbnailMaxresWidth, &enrichment.ThumbnailMaxresHeight,
			// Engagement
			&enrichment.ViewCount, &enrichment.LikeCount, &enrichment.DislikeCount,
			&enrichment.FavoriteCount, &enrichment.CommentCount,
			// Categorization
			&enrichment.CategoryID, &tagsJSON, &enrichment.DefaultLanguage,
			&enrichment.DefaultAudioLanguage, &topicCategoriesJSON,
			// Content classification
			&enrichment.PrivacyStatus, &enrichment.License, &enrichment.Embeddable,
			&enrichment.PublicStatsViewable, &enrichment.MadeForKids, &enrichment.SelfDeclaredMadeForKids,
			// Upload details
			&enrichment.UploadStatus, &enrichment.FailureReason, &enrichment.RejectionReason,
			// Live streaming
			&enrichment.LiveBroadcastContent, &enrichment.ScheduledStartTime, &enrichment.ActualStartTime,
			&enrichment.ActualEndTime, &enrichment.ConcurrentViewers,
			// Location
			&enrichment.LocationDescription, &enrichment.LocationLatitude, &enrichment.LocationLongitude,
			// Content rating and channel
			&contentRatingJSON, &enrichment.ChannelTitle,
			// API metadata
			&enrichment.EnrichedAt, &enrichment.APIResponseEtag, &enrichment.QuotaCost,
			&apiPartsJSON, &rawAPIResponseJSON,
			// Timestamps
			&enrichment.CreatedAt, &enrichment.UpdatedAt,
		)

		if err != nil {
			return nil, db.WrapError(err, "scan batch enrichment")
		}

		// Unmarshal JSON fields
		json.Unmarshal(tagsJSON, &enrichment.Tags)
		json.Unmarshal(topicCategoriesJSON, &enrichment.TopicCategories)
		json.Unmarshal(apiPartsJSON, &enrichment.APIPartsRequested)
		json.Unmarshal(contentRatingJSON, &enrichment.ContentRating)
		json.Unmarshal(rawAPIResponseJSON, &enrichment.RawAPIResponse)

		enrichments[enrichment.VideoID] = enrichment
	}

	return enrichments, nil
}

// ChannelEnrichmentRepository defines operations for managing channel enrichments
type ChannelEnrichmentRepository interface {
	// Create stores a new channel enrichment
	Create(ctx context.Context, enrichment *model.ChannelEnrichment) error

	// GetLatest retrieves the most recent enrichment for a channel
	GetLatest(ctx context.Context, channelID string) (*model.ChannelEnrichment, error)

	// GetHistory retrieves all enrichments for a channel
	GetHistory(ctx context.Context, channelID string, limit int) ([]*model.ChannelEnrichment, error)

	// GetBatchLatest retrieves the most recent enrichment for multiple channels
	GetBatchLatest(ctx context.Context, channelIDs []string) (map[string]*model.ChannelEnrichment, error)
}

type channelEnrichmentRepository struct {
	pool *pgxpool.Pool
}

// NewChannelEnrichmentRepository creates a new ChannelEnrichmentRepository
func NewChannelEnrichmentRepository(pool *pgxpool.Pool) ChannelEnrichmentRepository {
	return &channelEnrichmentRepository{pool: pool}
}

func (r *channelEnrichmentRepository) Create(ctx context.Context, enrichment *model.ChannelEnrichment) error {
	query := `
		INSERT INTO channel_api_enrichments (
			channel_id, description, custom_url, country, published_at,
			thumbnail_default_url, thumbnail_medium_url, thumbnail_high_url,
			view_count, subscriber_count, video_count, hidden_subscriber_count,
			banner_image_url, keywords,
			related_playlists_likes, related_playlists_uploads, related_playlists_favorites,
			topic_categories,
			privacy_status, is_linked, long_uploads_status, made_for_kids,
			enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11, $12,
			$13, $14,
			$15, $16, $17,
			$18,
			$19, $20, $21, $22,
			$23, $24, $25, $26, $27,
			$28, $29
		)
		RETURNING id, enriched_at, created_at, updated_at
	`

	// Convert arrays and maps to JSON
	topicCategoriesJSON, _ := json.Marshal(enrichment.TopicCategories)
	apiPartsJSON, _ := json.Marshal(enrichment.APIPartsRequested)
	rawAPIResponseJSON, _ := json.Marshal(enrichment.RawAPIResponse)

	now := time.Now()
	enrichedAt := now
	if !enrichment.EnrichedAt.IsZero() {
		enrichedAt = enrichment.EnrichedAt
	}

	err := r.pool.QueryRow(ctx, query,
		enrichment.ChannelID,
		enrichment.Description,
		enrichment.CustomURL,
		enrichment.Country,
		enrichment.PublishedAt,
		// Thumbnails
		enrichment.ThumbnailDefaultURL,
		enrichment.ThumbnailMediumURL,
		enrichment.ThumbnailHighURL,
		// Statistics
		enrichment.ViewCount,
		enrichment.SubscriberCount,
		enrichment.VideoCount,
		enrichment.HiddenSubscriberCount,
		// Branding
		enrichment.BannerImageURL,
		enrichment.Keywords,
		// Playlists
		enrichment.RelatedPlaylistsLikes,
		enrichment.RelatedPlaylistsUploads,
		enrichment.RelatedPlaylistsFavorites,
		// Topics
		topicCategoriesJSON,
		// Status
		enrichment.PrivacyStatus,
		enrichment.IsLinked,
		enrichment.LongUploadsStatus,
		enrichment.MadeForKids,
		// API metadata
		enrichedAt,
		enrichment.APIResponseEtag,
		enrichment.QuotaCost,
		apiPartsJSON,
		rawAPIResponseJSON,
		// Timestamps
		now, now,
	).Scan(
		&enrichment.ID,
		&enrichment.EnrichedAt,
		&enrichment.CreatedAt,
		&enrichment.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create channel enrichment")
	}

	return nil
}

func (r *channelEnrichmentRepository) GetLatest(ctx context.Context, channelID string) (*model.ChannelEnrichment, error) {
	query := `
		SELECT
			id, channel_id, description, custom_url, country, published_at,
			thumbnail_default_url, thumbnail_medium_url, thumbnail_high_url,
			view_count, subscriber_count, video_count, hidden_subscriber_count,
			banner_image_url, keywords,
			related_playlists_likes, related_playlists_uploads, related_playlists_favorites,
			topic_categories,
			privacy_status, is_linked, long_uploads_status, made_for_kids,
			enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
			created_at, updated_at
		FROM channel_api_enrichments
		WHERE channel_id = $1
		ORDER BY enriched_at DESC
		LIMIT 1
	`

	enrichment := &model.ChannelEnrichment{}
	var topicCategoriesJSON, apiPartsJSON, rawAPIResponseJSON []byte

	err := r.pool.QueryRow(ctx, query, channelID).Scan(
		&enrichment.ID,
		&enrichment.ChannelID,
		&enrichment.Description,
		&enrichment.CustomURL,
		&enrichment.Country,
		&enrichment.PublishedAt,
		// Thumbnails
		&enrichment.ThumbnailDefaultURL,
		&enrichment.ThumbnailMediumURL,
		&enrichment.ThumbnailHighURL,
		// Statistics
		&enrichment.ViewCount,
		&enrichment.SubscriberCount,
		&enrichment.VideoCount,
		&enrichment.HiddenSubscriberCount,
		// Branding
		&enrichment.BannerImageURL,
		&enrichment.Keywords,
		// Playlists
		&enrichment.RelatedPlaylistsLikes,
		&enrichment.RelatedPlaylistsUploads,
		&enrichment.RelatedPlaylistsFavorites,
		// Topics
		&topicCategoriesJSON,
		// Status
		&enrichment.PrivacyStatus,
		&enrichment.IsLinked,
		&enrichment.LongUploadsStatus,
		&enrichment.MadeForKids,
		// API metadata
		&enrichment.EnrichedAt,
		&enrichment.APIResponseEtag,
		&enrichment.QuotaCost,
		&apiPartsJSON,
		&rawAPIResponseJSON,
		// Timestamps
		&enrichment.CreatedAt,
		&enrichment.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, db.WrapError(err, "get latest channel enrichment")
	}

	// Unmarshal JSON fields
	json.Unmarshal(topicCategoriesJSON, &enrichment.TopicCategories)
	json.Unmarshal(apiPartsJSON, &enrichment.APIPartsRequested)
	json.Unmarshal(rawAPIResponseJSON, &enrichment.RawAPIResponse)

	return enrichment, nil
}

func (r *channelEnrichmentRepository) GetHistory(ctx context.Context, channelID string, limit int) ([]*model.ChannelEnrichment, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, channel_id, enriched_at, quota_cost,
			subscriber_count, video_count, view_count,
			created_at
		FROM channel_api_enrichments
		WHERE channel_id = $1
		ORDER BY enriched_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, channelID, limit)
	if err != nil {
		return nil, db.WrapError(err, "get channel enrichment history")
	}
	defer rows.Close()

	var enrichments []*model.ChannelEnrichment
	for rows.Next() {
		e := &model.ChannelEnrichment{}
		err := rows.Scan(
			&e.ID,
			&e.ChannelID,
			&e.EnrichedAt,
			&e.QuotaCost,
			&e.SubscriberCount,
			&e.VideoCount,
			&e.ViewCount,
			&e.CreatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan channel enrichment history")
		}
		enrichments = append(enrichments, e)
	}

	return enrichments, nil
}

func (r *channelEnrichmentRepository) GetBatchLatest(ctx context.Context, channelIDs []string) (map[string]*model.ChannelEnrichment, error) {
	if len(channelIDs) == 0 {
		return make(map[string]*model.ChannelEnrichment), nil
	}

	query := `
		WITH latest_enrichments AS (
			SELECT DISTINCT ON (channel_id)
				id, channel_id, description, custom_url, country, published_at,
				thumbnail_default_url, thumbnail_medium_url, thumbnail_high_url,
				view_count, subscriber_count, video_count, hidden_subscriber_count,
				banner_image_url, keywords,
				related_playlists_likes, related_playlists_uploads, related_playlists_favorites,
				topic_categories,
				privacy_status, is_linked, long_uploads_status, made_for_kids,
				enriched_at, api_response_etag, quota_cost, api_parts_requested, raw_api_response,
				created_at, updated_at
			FROM channel_api_enrichments
			WHERE channel_id = ANY($1)
			ORDER BY channel_id, enriched_at DESC
		)
		SELECT * FROM latest_enrichments
	`

	rows, err := r.pool.Query(ctx, query, channelIDs)
	if err != nil {
		return nil, db.WrapError(err, "get batch latest channel enrichments")
	}
	defer rows.Close()

	enrichments := make(map[string]*model.ChannelEnrichment)
	for rows.Next() {
		enrichment := &model.ChannelEnrichment{}
		var topicCategoriesJSON, apiPartsJSON, rawAPIResponseJSON []byte

		err := rows.Scan(
			&enrichment.ID,
			&enrichment.ChannelID,
			&enrichment.Description,
			&enrichment.CustomURL,
			&enrichment.Country,
			&enrichment.PublishedAt,
			// Thumbnails
			&enrichment.ThumbnailDefaultURL,
			&enrichment.ThumbnailMediumURL,
			&enrichment.ThumbnailHighURL,
			// Statistics
			&enrichment.ViewCount,
			&enrichment.SubscriberCount,
			&enrichment.VideoCount,
			&enrichment.HiddenSubscriberCount,
			// Branding
			&enrichment.BannerImageURL,
			&enrichment.Keywords,
			// Playlists
			&enrichment.RelatedPlaylistsLikes,
			&enrichment.RelatedPlaylistsUploads,
			&enrichment.RelatedPlaylistsFavorites,
			// Topics
			&topicCategoriesJSON,
			// Status
			&enrichment.PrivacyStatus,
			&enrichment.IsLinked,
			&enrichment.LongUploadsStatus,
			&enrichment.MadeForKids,
			// API metadata
			&enrichment.EnrichedAt,
			&enrichment.APIResponseEtag,
			&enrichment.QuotaCost,
			&apiPartsJSON,
			&rawAPIResponseJSON,
			// Timestamps
			&enrichment.CreatedAt,
			&enrichment.UpdatedAt,
		)

		if err != nil {
			return nil, db.WrapError(err, "scan batch channel enrichment")
		}

		// Unmarshal JSON fields
		json.Unmarshal(topicCategoriesJSON, &enrichment.TopicCategories)
		json.Unmarshal(apiPartsJSON, &enrichment.APIPartsRequested)
		json.Unmarshal(rawAPIResponseJSON, &enrichment.RawAPIResponse)

		enrichments[enrichment.ChannelID] = enrichment
	}

	return enrichments, nil
}
