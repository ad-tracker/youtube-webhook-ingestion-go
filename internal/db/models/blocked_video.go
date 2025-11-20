package models

import (
	"database/sql"
	"time"
)

// BlockedVideo represents a video that should be ignored by the webhook processor.
// Videos in this table will have their webhook events rejected before any database writes.
type BlockedVideo struct {
	ID        int64        `json:"id"`
	VideoID   string       `json:"video_id"`
	Reason    string       `json:"reason"`
	CreatedAt time.Time    `json:"created_at"`
	CreatedBy sql.NullString `json:"created_by"`
}
