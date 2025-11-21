-- Revert the increment_quota_usage function to original version (without search_list)
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

-- Remove the search_list_calls column
ALTER TABLE api_quota_usage
DROP COLUMN search_list_calls;
