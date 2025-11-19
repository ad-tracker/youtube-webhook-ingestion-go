-- Create enrichment_jobs table to track job metadata
-- Note: asynq uses Redis as the primary job queue, but we track our own state in PostgreSQL
-- for analytics, debugging, and persistence beyond Redis TTL
CREATE TABLE enrichment_jobs (
    id BIGSERIAL PRIMARY KEY,

    -- Job identification
    asynq_task_id VARCHAR(100) UNIQUE,       -- ID from asynq for correlation
    job_type VARCHAR(50) NOT NULL,           -- 'youtube_api_enrichment', future: 'ad_detection', 'sponsorblock', etc.

    -- Target resource
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,

    -- Job status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- 'pending', 'processing', 'completed', 'failed', 'cancelled'

    -- Priority and scheduling
    priority INTEGER NOT NULL DEFAULT 0,      -- Higher = more urgent (currently not used, FIFO)
    scheduled_at TIMESTAMPTZ NOT NULL,        -- When job should be processed
    started_at TIMESTAMPTZ,                   -- When processing actually began
    completed_at TIMESTAMPTZ,                 -- When processing finished (success or failure)

    -- Retry logic
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,               -- When to retry if failed

    -- Error tracking
    error_message TEXT,
    error_stack_trace TEXT,

    -- Metadata
    metadata JSONB,                          -- Additional context (e.g., {"source": "webhook", "channel_id": "..."})

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT chk_status CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    CONSTRAINT chk_attempts CHECK (attempts >= 0 AND attempts <= max_attempts)
);

-- Indexes for efficient job processing
CREATE INDEX idx_enrichment_jobs_status ON enrichment_jobs(status)
    WHERE status IN ('pending', 'processing');  -- Partial index for active jobs

CREATE INDEX idx_enrichment_jobs_priority ON enrichment_jobs(priority DESC, scheduled_at ASC)
    WHERE status = 'pending';                   -- For priority-based dequeuing

CREATE INDEX idx_enrichment_jobs_video_id ON enrichment_jobs(video_id);
CREATE INDEX idx_enrichment_jobs_job_type ON enrichment_jobs(job_type);
CREATE INDEX idx_enrichment_jobs_scheduled_at ON enrichment_jobs(scheduled_at);
CREATE INDEX idx_enrichment_jobs_created_at ON enrichment_jobs(created_at DESC);

-- Composite index for finding failed jobs ready for retry
CREATE INDEX idx_enrichment_jobs_retry ON enrichment_jobs(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_enrichment_jobs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_enrichment_jobs_updated_at
    BEFORE UPDATE ON enrichment_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_enrichment_jobs_updated_at();

-- Comment on table
COMMENT ON TABLE enrichment_jobs IS 'Tracks enrichment job metadata for analytics and debugging. Primary job queue is Redis (asynq), this is for persistence and observability.';

-- Helper view for job statistics
CREATE OR REPLACE VIEW enrichment_job_stats AS
SELECT
    job_type,
    status,
    COUNT(*) AS count,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) AS avg_duration_seconds,
    MIN(created_at) AS oldest_job,
    MAX(created_at) AS newest_job
FROM enrichment_jobs
WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'  -- Last 7 days
GROUP BY job_type, status;

COMMENT ON VIEW enrichment_job_stats IS 'Provides job statistics for the last 7 days';
