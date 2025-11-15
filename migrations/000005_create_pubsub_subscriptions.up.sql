-- Create pubsub_subscriptions table
CREATE TABLE pubsub_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    channel_id VARCHAR(255) NOT NULL,
    topic_url TEXT NOT NULL,
    callback_url TEXT NOT NULL,
    hub_url TEXT NOT NULL DEFAULT 'https://pubsubhubbub.appspot.com/subscribe',
    lease_seconds INTEGER NOT NULL DEFAULT 432000,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    secret VARCHAR(255),
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT uq_channel_callback UNIQUE (channel_id, callback_url),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'active', 'expired', 'failed'))
);

-- Indexes for common queries
CREATE INDEX idx_pubsub_subscriptions_expires_at ON pubsub_subscriptions(expires_at);
CREATE INDEX idx_pubsub_subscriptions_status ON pubsub_subscriptions(status);
CREATE INDEX idx_pubsub_subscriptions_channel_id ON pubsub_subscriptions(channel_id);

-- Trigger to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_pubsub_subscriptions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_pubsub_subscriptions_updated_at
    BEFORE UPDATE ON pubsub_subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_pubsub_subscriptions_updated_at();
