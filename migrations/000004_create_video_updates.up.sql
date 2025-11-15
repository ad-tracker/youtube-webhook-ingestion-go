-- Create video_updates table (immutable audit trail)
CREATE TABLE video_updates (
    id BIGSERIAL PRIMARY KEY,
    webhook_event_id BIGINT NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,
    channel_id VARCHAR(30) NOT NULL REFERENCES channels(channel_id) ON DELETE CASCADE,

    -- Snapshot of data at this point in time
    title VARCHAR(500) NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    feed_updated_at TIMESTAMPTZ NOT NULL,

    -- Update type detection
    update_type VARCHAR(50) NOT NULL, -- 'new_video', 'title_update', 'description_update', 'unknown'

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_video_updates_webhook_event_id ON video_updates(webhook_event_id);
CREATE INDEX idx_video_updates_video_id ON video_updates(video_id, created_at DESC);
CREATE INDEX idx_video_updates_channel_id ON video_updates(channel_id, created_at DESC);
CREATE INDEX idx_video_updates_update_type ON video_updates(update_type);
CREATE INDEX idx_video_updates_created_at ON video_updates(created_at DESC);
