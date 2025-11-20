-- Remove the unique constraint that includes callback_url
ALTER TABLE pubsub_subscriptions DROP CONSTRAINT IF EXISTS uq_channel_callback;

-- Drop the callback_url column
ALTER TABLE pubsub_subscriptions DROP COLUMN IF EXISTS callback_url;

-- Add a new unique constraint on channel_id only
-- This ensures one subscription per channel
ALTER TABLE pubsub_subscriptions ADD CONSTRAINT uq_channel UNIQUE (channel_id);
