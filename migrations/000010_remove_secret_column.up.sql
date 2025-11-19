-- Remove the secret column from pubsub_subscriptions table
-- Secrets should be sourced from environment variables, not stored in database
ALTER TABLE pubsub_subscriptions DROP COLUMN IF EXISTS secret;
