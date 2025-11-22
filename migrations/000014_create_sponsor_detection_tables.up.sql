-- Migration: 000013_create_sponsor_detection_tables
-- Description: Creates tables for LLM-based sponsor detection feature
--
-- This migration adds four tables:
-- 1. sponsor_detection_prompts: Stores unique prompts (deduplicated by hash) to avoid storing the same prompt text multiple times
-- 2. sponsors: Master table for all sponsors/brands (normalized to prevent duplicates)
-- 3. sponsor_detection_jobs: Tracks each LLM analysis run per video
-- 4. video_sponsors: Many-to-many relationship between videos and sponsors with confidence scores
--
-- Design rationale:
-- - UUIDs for primary keys: Better for distributed systems, prevents PK collision
-- - Normalized sponsors: Single source of truth for sponsor names, enables analytics
-- - Prompt deduplication: Saves storage when same prompt used for thousands of videos
-- - Foreign key to prompts is ON DELETE SET NULL: Allows prompt cleanup without breaking job history
-- - Foreign keys to videos ON DELETE CASCADE: Clean up sponsor data when videos are deleted

-- Table 1: sponsor_detection_prompts
-- Stores unique LLM prompts to avoid duplication across thousands of jobs
CREATE TABLE sponsor_detection_prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prompt_text TEXT NOT NULL,
    prompt_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hash of prompt_text for deduplication
    version VARCHAR(50), -- e.g., 'v1.0', 'v1.1' for tracking prompt evolution
    description TEXT, -- Human-readable description of what changed in this prompt version
    usage_count INTEGER NOT NULL DEFAULT 0, -- How many detection jobs have used this prompt
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast deduplication lookups by hash
CREATE UNIQUE INDEX idx_sponsor_detection_prompts_hash ON sponsor_detection_prompts(prompt_hash);

-- Index for viewing prompt history
CREATE INDEX idx_sponsor_detection_prompts_created_at ON sponsor_detection_prompts(created_at DESC);

-- Table 2: sponsors
-- Master table for all sponsors/brands (normalized)
CREATE TABLE sponsors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE, -- Original sponsor name as detected by LLM
    normalized_name VARCHAR(255) NOT NULL, -- Lowercase version for case-insensitive matching
    category VARCHAR(100), -- e.g., 'VPN', 'Education', 'Software', 'Productivity'
    website_url TEXT, -- Optional sponsor website
    description TEXT, -- Optional description of the sponsor
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- When this sponsor was first detected
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Most recent detection
    video_count INTEGER NOT NULL DEFAULT 0, -- Denormalized count of videos sponsored (for analytics)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique index on sponsor name to prevent duplicates
CREATE UNIQUE INDEX idx_sponsors_name ON sponsors(name);

-- Index for case-insensitive lookups (prevents "NordVPN" vs "nordvpn" duplicates)
CREATE INDEX idx_sponsors_normalized_name ON sponsors(normalized_name);

-- Index for filtering sponsors by category
CREATE INDEX idx_sponsors_category ON sponsors(category) WHERE category IS NOT NULL;

-- Index for finding recently active sponsors
CREATE INDEX idx_sponsors_last_seen_at ON sponsors(last_seen_at DESC);

-- Index for analytics queries (top sponsors by video count)
CREATE INDEX idx_sponsors_video_count ON sponsors(video_count DESC);

-- Table 3: sponsor_detection_jobs
-- Tracks each LLM analysis run for a video
CREATE TABLE sponsor_detection_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,
    prompt_id UUID REFERENCES sponsor_detection_prompts(id) ON DELETE SET NULL, -- NULL if prompt was deleted
    llm_model VARCHAR(100) NOT NULL, -- e.g., 'llama3:8b', 'llama3:70b'
    llm_response_raw TEXT, -- Raw JSON response from LLM (for debugging and reprocessing)
    sponsors_detected_count INTEGER NOT NULL DEFAULT 0, -- Denormalized count for quick stats
    processing_time_ms INTEGER, -- Time taken for LLM API call (for performance monitoring)
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'completed', 'failed', 'skipped'
    error_message TEXT, -- Error details if status is 'failed'
    detected_at TIMESTAMPTZ, -- When detection completed (NULL if pending or failed)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding all detection runs for a specific video
CREATE INDEX idx_sponsor_detection_jobs_video_id ON sponsor_detection_jobs(video_id);

-- Index for analyzing which prompts are used
CREATE INDEX idx_sponsor_detection_jobs_prompt_id ON sponsor_detection_jobs(prompt_id) WHERE prompt_id IS NOT NULL;

-- Index for chronological queries
CREATE INDEX idx_sponsor_detection_jobs_detected_at ON sponsor_detection_jobs(detected_at DESC) WHERE detected_at IS NOT NULL;

-- Index for monitoring job status
CREATE INDEX idx_sponsor_detection_jobs_status ON sponsor_detection_jobs(status);

-- Index for job history
CREATE INDEX idx_sponsor_detection_jobs_created_at ON sponsor_detection_jobs(created_at DESC);

-- Index for analyzing LLM model performance
CREATE INDEX idx_sponsor_detection_jobs_llm_model ON sponsor_detection_jobs(llm_model);

-- Table 4: video_sponsors
-- Many-to-many relationship between videos and sponsors
CREATE TABLE video_sponsors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,
    sponsor_id UUID NOT NULL REFERENCES sponsors(id) ON DELETE CASCADE,
    detection_job_id UUID NOT NULL REFERENCES sponsor_detection_jobs(id) ON DELETE CASCADE,
    confidence NUMERIC(5,4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1), -- 0.0000 to 1.0000
    evidence TEXT NOT NULL, -- Quote from video title/description that indicates sponsorship
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Prevent duplicate sponsor detections in the same job run
    UNIQUE(video_id, sponsor_id, detection_job_id)
);

-- Index for finding all sponsors for a specific video
CREATE INDEX idx_video_sponsors_video_id ON video_sponsors(video_id);

-- Index for finding all videos sponsored by a specific sponsor
CREATE INDEX idx_video_sponsors_sponsor_id ON video_sponsors(sponsor_id);

-- Index for finding all sponsors detected in a specific job run
CREATE INDEX idx_video_sponsors_detection_job_id ON video_sponsors(detection_job_id);

-- Index for filtering high-confidence detections
CREATE INDEX idx_video_sponsors_confidence ON video_sponsors(confidence DESC);

-- Index for chronological queries
CREATE INDEX idx_video_sponsors_detected_at ON video_sponsors(detected_at DESC);

-- Composite index for efficient "sponsors for video" queries with JOIN
CREATE INDEX idx_video_sponsors_video_sponsor ON video_sponsors(video_id, sponsor_id);
