-- Drop view
DROP VIEW IF EXISTS enrichment_job_stats;

-- Drop trigger
DROP TRIGGER IF EXISTS trigger_update_enrichment_jobs_updated_at ON enrichment_jobs;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_enrichment_jobs_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_enrichment_jobs_retry;
DROP INDEX IF EXISTS idx_enrichment_jobs_created_at;
DROP INDEX IF EXISTS idx_enrichment_jobs_scheduled_at;
DROP INDEX IF EXISTS idx_enrichment_jobs_job_type;
DROP INDEX IF EXISTS idx_enrichment_jobs_video_id;
DROP INDEX IF EXISTS idx_enrichment_jobs_priority;
DROP INDEX IF EXISTS idx_enrichment_jobs_status;

-- Drop table
DROP TABLE IF EXISTS enrichment_jobs;
