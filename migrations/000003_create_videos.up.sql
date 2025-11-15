-- Create videos table
CREATE TABLE videos (
    video_id VARCHAR(20) PRIMARY KEY,
    channel_id VARCHAR(30) NOT NULL REFERENCES channels(channel_id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    video_url VARCHAR(500) NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,

    -- Metadata
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_videos_channel_id ON videos(channel_id);
CREATE INDEX idx_videos_published_at ON videos(published_at DESC);
CREATE INDEX idx_videos_last_updated_at ON videos(last_updated_at DESC);
CREATE INDEX idx_videos_title ON videos(title);
