-- Create api_quota_usage table to track daily YouTube API quota consumption
CREATE TABLE api_quota_usage (
    id BIGSERIAL PRIMARY KEY,

    -- Date tracking (one row per day)
    date DATE NOT NULL,

    -- Quota tracking
    quota_used INTEGER NOT NULL DEFAULT 0,
    quota_limit INTEGER NOT NULL,              -- Daily limit (default 10,000 for YouTube API v3)

    -- Operation tracking
    operations_count INTEGER NOT NULL DEFAULT 0,  -- Number of API calls made

    -- Breakdown by operation type (for analysis)
    videos_list_calls INTEGER NOT NULL DEFAULT 0,
    channels_list_calls INTEGER NOT NULL DEFAULT 0,
    other_calls INTEGER NOT NULL DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure only one row per date
    CONSTRAINT uq_quota_date UNIQUE (date)
);

-- Index for quick lookups by date
CREATE INDEX idx_api_quota_usage_date ON api_quota_usage(date DESC);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_api_quota_usage_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_api_quota_usage_updated_at
    BEFORE UPDATE ON api_quota_usage
    FOR EACH ROW
    EXECUTE FUNCTION update_api_quota_usage_updated_at();

-- Comment on table
COMMENT ON TABLE api_quota_usage IS 'Tracks daily YouTube API v3 quota consumption to prevent exceeding limits.';

-- Helper function to get current quota usage for today
CREATE OR REPLACE FUNCTION get_todays_quota_usage()
RETURNS TABLE (
    quota_used INTEGER,
    quota_limit INTEGER,
    quota_remaining INTEGER,
    operations_count INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        COALESCE(q.quota_used, 0) AS quota_used,
        COALESCE(q.quota_limit, 10000) AS quota_limit,
        COALESCE(q.quota_limit, 10000) - COALESCE(q.quota_used, 0) AS quota_remaining,
        COALESCE(q.operations_count, 0) AS operations_count
    FROM api_quota_usage q
    WHERE q.date = CURRENT_DATE
    LIMIT 1;

    -- If no row exists for today, return default values
    IF NOT FOUND THEN
        RETURN QUERY SELECT 0, 10000, 10000, 0;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Helper function to increment quota usage
CREATE OR REPLACE FUNCTION increment_quota_usage(
    p_quota_cost INTEGER,
    p_operation_type VARCHAR DEFAULT 'other'
)
RETURNS void AS $$
BEGIN
    INSERT INTO api_quota_usage (
        date,
        quota_used,
        quota_limit,
        operations_count,
        videos_list_calls,
        channels_list_calls,
        other_calls
    ) VALUES (
        CURRENT_DATE,
        p_quota_cost,
        10000,  -- Default YouTube API v3 quota
        1,
        CASE WHEN p_operation_type = 'videos_list' THEN 1 ELSE 0 END,
        CASE WHEN p_operation_type = 'channels_list' THEN 1 ELSE 0 END,
        CASE WHEN p_operation_type = 'other' THEN 1 ELSE 0 END
    )
    ON CONFLICT (date) DO UPDATE SET
        quota_used = api_quota_usage.quota_used + p_quota_cost,
        operations_count = api_quota_usage.operations_count + 1,
        videos_list_calls = api_quota_usage.videos_list_calls +
            CASE WHEN p_operation_type = 'videos_list' THEN 1 ELSE 0 END,
        channels_list_calls = api_quota_usage.channels_list_calls +
            CASE WHEN p_operation_type = 'channels_list' THEN 1 ELSE 0 END,
        other_calls = api_quota_usage.other_calls +
            CASE WHEN p_operation_type = 'other' THEN 1 ELSE 0 END,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
