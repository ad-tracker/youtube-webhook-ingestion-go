-- Add search_list_calls column to api_quota_usage table
-- Search API calls are the most expensive (100 units each), so tracking them separately is important

ALTER TABLE api_quota_usage
ADD COLUMN search_list_calls INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN api_quota_usage.search_list_calls IS 'Count of search.list API calls (100 units each)';

-- Update the increment_quota_usage function to handle search_list operation type
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
        search_list_calls,
        other_calls
    ) VALUES (
        CURRENT_DATE,
        p_quota_cost,
        10000,  -- Default YouTube API v3 quota
        1,
        CASE WHEN p_operation_type = 'videos_list' THEN 1 ELSE 0 END,
        CASE WHEN p_operation_type = 'channels_list' THEN 1 ELSE 0 END,
        CASE WHEN p_operation_type = 'search_list' THEN 1 ELSE 0 END,
        CASE WHEN p_operation_type = 'other' THEN 1 ELSE 0 END
    )
    ON CONFLICT (date) DO UPDATE SET
        quota_used = api_quota_usage.quota_used + p_quota_cost,
        operations_count = api_quota_usage.operations_count + 1,
        videos_list_calls = api_quota_usage.videos_list_calls +
            CASE WHEN p_operation_type = 'videos_list' THEN 1 ELSE 0 END,
        channels_list_calls = api_quota_usage.channels_list_calls +
            CASE WHEN p_operation_type = 'channels_list' THEN 1 ELSE 0 END,
        search_list_calls = api_quota_usage.search_list_calls +
            CASE WHEN p_operation_type = 'search_list' THEN 1 ELSE 0 END,
        other_calls = api_quota_usage.other_calls +
            CASE WHEN p_operation_type = 'other' THEN 1 ELSE 0 END,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
