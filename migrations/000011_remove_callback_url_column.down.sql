-- Drop the unique constraint on channel_id only
ALTER TABLE pubsub_subscriptions DROP CONSTRAINT IF EXISTS uq_channel;

-- Re-add the callback_url column
ALTER TABLE pubsub_subscriptions ADD COLUMN callback_url TEXT;

-- Re-add the unique constraint on both channel_id and callback_url
ALTER TABLE pubsub_subscriptions ADD CONSTRAINT uq_channel_callback UNIQUE (channel_id, callback_url);
