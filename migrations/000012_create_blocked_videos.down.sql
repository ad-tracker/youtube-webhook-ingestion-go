-- Drop blocked_videos table and its indexes
DROP INDEX IF EXISTS idx_blocked_videos_created_at;
DROP INDEX IF EXISTS idx_blocked_videos_video_id;
DROP TABLE IF EXISTS blocked_videos;
