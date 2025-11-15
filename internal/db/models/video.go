package models

import "time"

// Video represents a YouTube video that we're tracking.
type Video struct {
	VideoID       string    `db:"video_id"`
	ChannelID     string    `db:"channel_id"`
	Title         string    `db:"title"`
	VideoURL      string    `db:"video_url"`
	PublishedAt   time.Time `db:"published_at"`
	FirstSeenAt   time.Time `db:"first_seen_at"`
	LastUpdatedAt time.Time `db:"last_updated_at"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// NewVideo creates a new Video with the given information.
func NewVideo(videoID, channelID, title, videoURL string, publishedAt time.Time) *Video {
	now := time.Now()
	return &Video{
		VideoID:       videoID,
		ChannelID:     channelID,
		Title:         title,
		VideoURL:      videoURL,
		PublishedAt:   publishedAt,
		FirstSeenAt:   now,
		LastUpdatedAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Update updates the video information and timestamps.
func (v *Video) Update(title, videoURL string, publishedAt time.Time) {
	v.Title = title
	v.VideoURL = videoURL
	v.PublishedAt = publishedAt
	v.LastUpdatedAt = time.Now()
	v.UpdatedAt = time.Now()
}
