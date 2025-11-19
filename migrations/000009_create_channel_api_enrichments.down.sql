-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_update_channel_api_enrichments_updated_at ON channel_api_enrichments;
DROP FUNCTION IF EXISTS update_channel_api_enrichments_updated_at();

-- Drop table
DROP TABLE IF EXISTS channel_api_enrichments;
