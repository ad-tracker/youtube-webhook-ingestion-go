-- Create channel_api_enrichments table to store comprehensive YouTube API v3 channel data
CREATE TABLE channel_api_enrichments (
    id BIGSERIAL PRIMARY KEY,
    channel_id VARCHAR(30) NOT NULL REFERENCES channels(channel_id) ON DELETE CASCADE,

    -- Basic metadata
    description TEXT,
    custom_url VARCHAR(500),                 -- Channel's custom URL (e.g., youtube.com/c/CustomName)
    country VARCHAR(2),                      -- ISO 3166-1 alpha-2 country code
    published_at TIMESTAMPTZ,                -- When the channel was created

    -- Thumbnails
    thumbnail_default_url TEXT,
    thumbnail_medium_url TEXT,
    thumbnail_high_url TEXT,

    -- Statistics (change over time)
    view_count BIGINT,
    subscriber_count BIGINT,
    video_count BIGINT,
    hidden_subscriber_count BOOLEAN,         -- Whether subscriber count is hidden

    -- Branding
    banner_image_url TEXT,                   -- Channel banner/header image
    keywords TEXT,                           -- Channel keywords (comma-separated)

    -- Content details
    related_playlists_likes VARCHAR(50),     -- Playlist ID for liked videos
    related_playlists_uploads VARCHAR(50),   -- Playlist ID for uploads
    related_playlists_favorites VARCHAR(50), -- Playlist ID for favorites

    -- Topic details
    topic_categories TEXT[],                 -- Array of Wikipedia URLs

    -- Status
    privacy_status VARCHAR(50),              -- "public", "unlisted", "private"
    is_linked BOOLEAN,                       -- Whether the channel is linked to a Google+ page
    long_uploads_status VARCHAR(50),         -- Status of long uploads feature
    made_for_kids BOOLEAN,                   -- Whether channel is made for kids

    -- API metadata
    enriched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    api_response_etag VARCHAR(100),          -- For conditional requests (If-None-Match)
    quota_cost INTEGER NOT NULL DEFAULT 6,   -- Track API quota cost for this enrichment
    api_parts_requested TEXT[],              -- Which API parts were requested

    -- Raw response storage (for debugging and future schema evolution)
    raw_api_response JSONB,                  -- Full API response as JSON

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_channel_api_enrichments_channel_id ON channel_api_enrichments(channel_id);
CREATE INDEX idx_channel_api_enrichments_enriched_at ON channel_api_enrichments(enriched_at DESC);
CREATE INDEX idx_channel_api_enrichments_country ON channel_api_enrichments(country);
CREATE INDEX idx_channel_api_enrichments_subscriber_count ON channel_api_enrichments(subscriber_count DESC);

-- Index for finding channels by topic categories
CREATE INDEX idx_channel_api_enrichments_topics ON channel_api_enrichments USING GIN(topic_categories);

-- Composite index for finding latest enrichment per channel
CREATE UNIQUE INDEX idx_channel_api_enrichments_channel_latest ON channel_api_enrichments(channel_id, enriched_at DESC);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_channel_api_enrichments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_channel_api_enrichments_updated_at
    BEFORE UPDATE ON channel_api_enrichments
    FOR EACH ROW
    EXECUTE FUNCTION update_channel_api_enrichments_updated_at();

-- Comment on table
COMMENT ON TABLE channel_api_enrichments IS 'Stores comprehensive YouTube API v3 data for channels. Supports multiple enrichments per channel for historical tracking.';
