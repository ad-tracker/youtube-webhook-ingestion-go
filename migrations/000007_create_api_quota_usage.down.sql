-- Drop helper functions
DROP FUNCTION IF EXISTS increment_quota_usage(INTEGER, VARCHAR);
DROP FUNCTION IF EXISTS get_todays_quota_usage();

-- Drop trigger
DROP TRIGGER IF EXISTS trigger_update_api_quota_usage_updated_at ON api_quota_usage;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_api_quota_usage_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_api_quota_usage_date;

-- Drop table
DROP TABLE IF EXISTS api_quota_usage;
