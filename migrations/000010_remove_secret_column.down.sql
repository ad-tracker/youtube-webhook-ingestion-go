-- Rollback: Add the secret column back to pubsub_subscriptions table
ALTER TABLE pubsub_subscriptions ADD COLUMN secret VARCHAR(255);
