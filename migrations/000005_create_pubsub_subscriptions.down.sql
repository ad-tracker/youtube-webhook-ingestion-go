-- Drop trigger and function
DROP TRIGGER IF EXISTS trg_update_pubsub_subscriptions_updated_at ON pubsub_subscriptions;
DROP FUNCTION IF EXISTS update_pubsub_subscriptions_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_pubsub_subscriptions_channel_id;
DROP INDEX IF EXISTS idx_pubsub_subscriptions_status;
DROP INDEX IF EXISTS idx_pubsub_subscriptions_expires_at;

-- Drop table
DROP TABLE IF EXISTS pubsub_subscriptions;
