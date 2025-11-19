package model

import "time"

// VideoEnrichment represents comprehensive YouTube API v3 data for a video
type VideoEnrichment struct {
	ID      int64  `json:"id"`
	VideoID string `json:"video_id"`

	// Basic metadata
	Description             *string `json:"description"`
	Duration                *string `json:"duration"`                 // ISO 8601 format (e.g., "PT4M13S")
	Dimension               *string `json:"dimension"`                // "2d" or "3d"
	Definition              *string `json:"definition"`               // "hd" or "sd"
	Caption                 *string `json:"caption"`                  // "true" or "false"
	LicensedContent         *bool   `json:"licensed_content"`
	Projection              *string `json:"projection"`               // "rectangular" or "360"

	// Thumbnails
	ThumbnailDefaultURL     *string `json:"thumbnail_default_url"`
	ThumbnailDefaultWidth   *int    `json:"thumbnail_default_width"`
	ThumbnailDefaultHeight  *int    `json:"thumbnail_default_height"`
	ThumbnailMediumURL      *string `json:"thumbnail_medium_url"`
	ThumbnailMediumWidth    *int    `json:"thumbnail_medium_width"`
	ThumbnailMediumHeight   *int    `json:"thumbnail_medium_height"`
	ThumbnailHighURL        *string `json:"thumbnail_high_url"`
	ThumbnailHighWidth      *int    `json:"thumbnail_high_width"`
	ThumbnailHighHeight     *int    `json:"thumbnail_high_height"`
	ThumbnailStandardURL    *string `json:"thumbnail_standard_url"`
	ThumbnailStandardWidth  *int    `json:"thumbnail_standard_width"`
	ThumbnailStandardHeight *int    `json:"thumbnail_standard_height"`
	ThumbnailMaxresURL      *string `json:"thumbnail_maxres_url"`
	ThumbnailMaxresWidth    *int    `json:"thumbnail_maxres_width"`
	ThumbnailMaxresHeight   *int    `json:"thumbnail_maxres_height"`

	// Engagement metrics
	ViewCount     *int64 `json:"view_count"`
	LikeCount     *int64 `json:"like_count"`
	DislikeCount  *int64 `json:"dislike_count"`
	FavoriteCount *int64 `json:"favorite_count"`
	CommentCount  *int64 `json:"comment_count"`

	// Categorization
	CategoryID           *string  `json:"category_id"`
	Tags                 []string `json:"tags"`
	DefaultLanguage      *string  `json:"default_language"`       // BCP-47 language code
	DefaultAudioLanguage *string  `json:"default_audio_language"` // BCP-47 language code
	TopicCategories      []string `json:"topic_categories"`       // Wikipedia URLs

	// Content classification
	PrivacyStatus            *string `json:"privacy_status"`             // "public", "unlisted", "private"
	License                  *string `json:"license"`                    // "youtube" or "creativeCommon"
	Embeddable               *bool   `json:"embeddable"`
	PublicStatsViewable      *bool   `json:"public_stats_viewable"`
	MadeForKids              *bool   `json:"made_for_kids"`
	SelfDeclaredMadeForKids  *bool   `json:"self_declared_made_for_kids"`

	// Upload details
	UploadStatus   *string `json:"upload_status"`   // "uploaded", "processed", "failed", "rejected", "deleted"
	FailureReason  *string `json:"failure_reason"`  // If upload_status is "failed" or "rejected"
	RejectionReason *string `json:"rejection_reason"` // If upload_status is "rejected"

	// Live streaming details
	LiveBroadcastContent *string    `json:"live_broadcast_content"` // "none", "upcoming", "live", "completed"
	ScheduledStartTime   *time.Time `json:"scheduled_start_time"`
	ActualStartTime      *time.Time `json:"actual_start_time"`
	ActualEndTime        *time.Time `json:"actual_end_time"`
	ConcurrentViewers    *int64     `json:"concurrent_viewers"`

	// Location data
	LocationDescription *string  `json:"location_description"`
	LocationLatitude    *float64 `json:"location_latitude"`
	LocationLongitude   *float64 `json:"location_longitude"`

	// Content rating (stored as JSON)
	ContentRating map[string]interface{} `json:"content_rating"`

	// Channel info (at enrichment time)
	ChannelTitle *string `json:"channel_title"`

	// API metadata
	EnrichedAt        time.Time `json:"enriched_at"`
	APIResponseEtag   *string   `json:"api_response_etag"`
	QuotaCost         int       `json:"quota_cost"`
	APIPartsRequested []string  `json:"api_parts_requested"`

	// Raw API response (for debugging and future schema evolution)
	RawAPIResponse map[string]interface{} `json:"raw_api_response"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EnrichmentJob represents a job to enrich a video with YouTube API data
type EnrichmentJob struct {
	ID           int64                  `json:"id"`
	AsynqTaskID  *string                `json:"asynq_task_id"`
	JobType      string                 `json:"job_type"`
	VideoID      string                 `json:"video_id"`
	Status       string                 `json:"status"` // pending, processing, completed, failed, cancelled
	Priority     int                    `json:"priority"`
	ScheduledAt  time.Time              `json:"scheduled_at"`
	StartedAt    *time.Time             `json:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at"`
	Attempts     int                    `json:"attempts"`
	MaxAttempts  int                    `json:"max_attempts"`
	NextRetryAt  *time.Time             `json:"next_retry_at"`
	ErrorMessage *string                `json:"error_message"`
	ErrorStackTrace *string             `json:"error_stack_trace"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// APIQuotaUsage tracks daily YouTube API quota consumption
type APIQuotaUsage struct {
	ID               int64     `json:"id"`
	Date             time.Time `json:"date"`
	QuotaUsed        int       `json:"quota_used"`
	QuotaLimit       int       `json:"quota_limit"`
	OperationsCount  int       `json:"operations_count"`
	VideosListCalls  int       `json:"videos_list_calls"`
	ChannelsListCalls int      `json:"channels_list_calls"`
	OtherCalls       int       `json:"other_calls"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// QuotaInfo provides current quota status
type QuotaInfo struct {
	QuotaUsed      int `json:"quota_used"`
	QuotaLimit     int `json:"quota_limit"`
	QuotaRemaining int `json:"quota_remaining"`
	OperationsCount int `json:"operations_count"`
}
