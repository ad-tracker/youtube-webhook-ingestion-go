package models

import "time"

// Channel represents a YouTube channel that we're tracking.
type Channel struct {
	ChannelID     string    `db:"channel_id" json:"channel_id"`
	Title         string    `db:"title" json:"title"`
	ChannelURL    string    `db:"channel_url" json:"channel_url"`
	FirstSeenAt   time.Time `db:"first_seen_at" json:"first_seen_at"`
	LastUpdatedAt time.Time `db:"last_updated_at" json:"last_updated_at"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

// NewChannel creates a new Channel with the given information.
func NewChannel(channelID, title, channelURL string) *Channel {
	now := time.Now()
	return &Channel{
		ChannelID:     channelID,
		Title:         title,
		ChannelURL:    channelURL,
		FirstSeenAt:   now,
		LastUpdatedAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Update updates the channel information and timestamps.
func (c *Channel) Update(title, channelURL string) {
	c.Title = title
	c.ChannelURL = channelURL
	c.LastUpdatedAt = time.Now()
	c.UpdatedAt = time.Now()
}
