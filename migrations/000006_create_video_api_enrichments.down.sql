-- Drop trigger
DROP TRIGGER IF EXISTS trigger_update_video_api_enrichments_updated_at ON video_api_enrichments;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_video_api_enrichments_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_video_api_enrichments_video_latest;
DROP INDEX IF EXISTS idx_video_api_enrichments_tags;
DROP INDEX IF EXISTS idx_video_api_enrichments_privacy_status;
DROP INDEX IF EXISTS idx_video_api_enrichments_category_id;
DROP INDEX IF EXISTS idx_video_api_enrichments_enriched_at;
DROP INDEX IF EXISTS idx_video_api_enrichments_video_id;

-- Drop table
DROP TABLE IF EXISTS video_api_enrichments;
