-- Create video_api_enrichments table to store comprehensive YouTube API v3 data
CREATE TABLE video_api_enrichments (
    id BIGSERIAL PRIMARY KEY,
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,

    -- Basic metadata
    description TEXT,
    duration VARCHAR(20),                    -- ISO 8601 duration (e.g., "PT4M13S")
    dimension VARCHAR(10),                   -- "2d" or "3d"
    definition VARCHAR(10),                  -- "hd" or "sd"
    caption VARCHAR(10),                     -- "true" or "false"
    licensed_content BOOLEAN,
    projection VARCHAR(20),                  -- "rectangular" or "360"

    -- Thumbnails (all available resolutions)
    thumbnail_default_url TEXT,
    thumbnail_default_width INTEGER,
    thumbnail_default_height INTEGER,
    thumbnail_medium_url TEXT,
    thumbnail_medium_width INTEGER,
    thumbnail_medium_height INTEGER,
    thumbnail_high_url TEXT,
    thumbnail_high_width INTEGER,
    thumbnail_high_height INTEGER,
    thumbnail_standard_url TEXT,
    thumbnail_standard_width INTEGER,
    thumbnail_standard_height INTEGER,
    thumbnail_maxres_url TEXT,
    thumbnail_maxres_width INTEGER,
    thumbnail_maxres_height INTEGER,

    -- Engagement metrics (change over time)
    view_count BIGINT,
    like_count BIGINT,
    dislike_count BIGINT,                    -- Usually hidden now, but still in API
    favorite_count BIGINT,
    comment_count BIGINT,

    -- Categorization
    category_id VARCHAR(50),
    tags TEXT[],                             -- Array of tags
    default_language VARCHAR(10),            -- BCP-47 language code
    default_audio_language VARCHAR(10),      -- BCP-47 language code
    topic_categories TEXT[],                 -- Array of Wikipedia URLs

    -- Content classification
    privacy_status VARCHAR(50),              -- "public", "unlisted", "private"
    license VARCHAR(50),                     -- "youtube" or "creativeCommon"
    embeddable BOOLEAN,
    public_stats_viewable BOOLEAN,
    made_for_kids BOOLEAN,
    self_declared_made_for_kids BOOLEAN,

    -- Upload details
    upload_status VARCHAR(50),               -- "uploaded", "processed", "failed", "rejected", "deleted"
    failure_reason VARCHAR(100),             -- If upload_status is "failed" or "rejected"
    rejection_reason VARCHAR(100),           -- If upload_status is "rejected"

    -- Live streaming details (if applicable)
    live_broadcast_content VARCHAR(50),      -- "none", "upcoming", "live", "completed"
    scheduled_start_time TIMESTAMPTZ,
    actual_start_time TIMESTAMPTZ,
    actual_end_time TIMESTAMPTZ,
    concurrent_viewers BIGINT,

    -- Location data (if available)
    location_description TEXT,
    location_latitude DOUBLE PRECISION,
    location_longitude DOUBLE PRECISION,

    -- Content rating
    content_rating JSONB,                    -- Store complex rating object as JSON

    -- Channel info (redundant with channels table, but captures at enrichment time)
    channel_title VARCHAR(500),

    -- API metadata
    enriched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    api_response_etag VARCHAR(100),          -- For conditional requests (If-None-Match)
    quota_cost INTEGER NOT NULL DEFAULT 1,   -- Track API quota cost for this enrichment
    api_parts_requested TEXT[],              -- Which API parts were requested (e.g., ["snippet", "contentDetails", "statistics"])

    -- Raw response storage (for debugging and future schema evolution)
    raw_api_response JSONB,                  -- Full API response as JSON

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_video_api_enrichments_video_id ON video_api_enrichments(video_id);
CREATE INDEX idx_video_api_enrichments_enriched_at ON video_api_enrichments(enriched_at DESC);
CREATE INDEX idx_video_api_enrichments_category_id ON video_api_enrichments(category_id);
CREATE INDEX idx_video_api_enrichments_privacy_status ON video_api_enrichments(privacy_status);

-- Index for finding videos by tags (GIN index for array contains operations)
CREATE INDEX idx_video_api_enrichments_tags ON video_api_enrichments USING GIN(tags);

-- Composite index for finding latest enrichment per video
CREATE UNIQUE INDEX idx_video_api_enrichments_video_latest ON video_api_enrichments(video_id, enriched_at DESC);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_video_api_enrichments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_video_api_enrichments_updated_at
    BEFORE UPDATE ON video_api_enrichments
    FOR EACH ROW
    EXECUTE FUNCTION update_video_api_enrichments_updated_at();

-- Comment on table
COMMENT ON TABLE video_api_enrichments IS 'Stores comprehensive YouTube API v3 data for videos. Supports multiple enrichments per video for historical tracking.';
