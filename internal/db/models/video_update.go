package models

import "time"

// UpdateType represents the type of update detected for a video.
type UpdateType string

const (
	UpdateTypeNewVideo          UpdateType = "new_video"
	UpdateTypeTitleUpdate       UpdateType = "title_update"
	UpdateTypeDescriptionUpdate UpdateType = "description_update"
	UpdateTypeUnknown           UpdateType = "unknown"
)

// VideoUpdate represents a historical record of a video update.
// This table is immutable - records are only created, never updated or deleted.
type VideoUpdate struct {
	ID             int64      `db:"id" json:"id"`
	WebhookEventID int64      `db:"webhook_event_id" json:"webhook_event_id"`
	VideoID        string     `db:"video_id" json:"video_id"`
	ChannelID      string     `db:"channel_id" json:"channel_id"`
	Title          string     `db:"title" json:"title"`
	PublishedAt    time.Time  `db:"published_at" json:"published_at"`
	FeedUpdatedAt  time.Time  `db:"feed_updated_at" json:"feed_updated_at"`
	UpdateType     UpdateType `db:"update_type" json:"update_type"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}

// NewVideoUpdate creates a new VideoUpdate with the given information.
func NewVideoUpdate(
	webhookEventID int64,
	videoID, channelID, title string,
	publishedAt, feedUpdatedAt time.Time,
	updateType UpdateType,
) *VideoUpdate {
	return &VideoUpdate{
		WebhookEventID: webhookEventID,
		VideoID:        videoID,
		ChannelID:      channelID,
		Title:          title,
		PublishedAt:    publishedAt,
		FeedUpdatedAt:  feedUpdatedAt,
		UpdateType:     updateType,
		CreatedAt:      time.Now(),
	}
}
