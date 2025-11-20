package models

import (
	"fmt"
	"time"
)

// Subscription status constants
const (
	StatusPending = "pending"
	StatusActive  = "active"
	StatusExpired = "expired"
	StatusFailed  = "failed"
)

// Subscription represents a PubSubHubbub subscription for a YouTube channel.
type Subscription struct {
	ID             int64      `db:"id" json:"id"`
	ChannelID      string     `db:"channel_id" json:"channel_id"`
	TopicURL       string     `db:"topic_url" json:"topic_url"`
	HubURL         string     `db:"hub_url" json:"hub_url"`
	LeaseSeconds   int        `db:"lease_seconds" json:"lease_seconds"`
	ExpiresAt      time.Time  `db:"expires_at" json:"expires_at"`
	Status         string     `db:"status" json:"status"`
	LastVerifiedAt *time.Time `db:"last_verified_at" json:"last_verified_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// NewSubscription creates a new Subscription with the given parameters.
// It automatically constructs the topic URL from the channel ID.
func NewSubscription(channelID string, leaseSeconds int) *Subscription {
	now := time.Now()
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	return &Subscription{
		ChannelID:    channelID,
		TopicURL:     topicURL,
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		LeaseSeconds: leaseSeconds,
		ExpiresAt:    now.Add(time.Duration(leaseSeconds) * time.Second),
		Status:       StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// MarkActive marks the subscription as active and updates the verification timestamp.
func (s *Subscription) MarkActive() {
	s.Status = StatusActive
	now := time.Now()
	s.LastVerifiedAt = &now
	s.UpdatedAt = now
}

// MarkFailed marks the subscription as failed.
func (s *Subscription) MarkFailed() {
	s.Status = StatusFailed
	s.UpdatedAt = time.Now()
}

// MarkExpired marks the subscription as expired.
func (s *Subscription) MarkExpired() {
	s.Status = StatusExpired
	s.UpdatedAt = time.Now()
}

// IsExpired returns true if the subscription has expired.
func (s *Subscription) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// UpdateExpiry updates the expiry time based on the lease seconds.
func (s *Subscription) UpdateExpiry(leaseSeconds int) {
	s.LeaseSeconds = leaseSeconds
	s.ExpiresAt = time.Now().Add(time.Duration(leaseSeconds) * time.Second)
	s.UpdatedAt = time.Now()
}
