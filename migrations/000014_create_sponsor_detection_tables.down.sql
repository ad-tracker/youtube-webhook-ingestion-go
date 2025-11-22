-- Migration rollback: 000013_create_sponsor_detection_tables
-- Description: Drops all sponsor detection tables in reverse order (respecting foreign key dependencies)

-- Drop tables in reverse order of creation to respect foreign key constraints
DROP TABLE IF EXISTS video_sponsors;
DROP TABLE IF EXISTS sponsor_detection_jobs;
DROP TABLE IF EXISTS sponsors;
DROP TABLE IF EXISTS sponsor_detection_prompts;
