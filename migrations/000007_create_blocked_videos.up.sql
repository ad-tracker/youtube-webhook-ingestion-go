-- Create blocked_videos table
-- This table stores video IDs that should be ignored by the webhook processor
CREATE TABLE blocked_videos (
    id BIGSERIAL PRIMARY KEY,
    video_id VARCHAR(20) NOT NULL UNIQUE,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255)
);

-- Create index for efficient lookups by video_id
CREATE INDEX idx_blocked_videos_video_id ON blocked_videos(video_id);

-- Create index for sorting by creation date
CREATE INDEX idx_blocked_videos_created_at ON blocked_videos(created_at DESC);
