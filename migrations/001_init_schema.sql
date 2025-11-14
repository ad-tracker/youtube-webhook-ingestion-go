-- Initialize database schema for webhook ingestion service

-- Create schema
CREATE SCHEMA IF NOT EXISTS webhook_ingestion;

-- Set search path
SET search_path TO webhook_ingestion;

-- Create UUIDv7 function (time-ordered UUIDs)
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS UUID AS $$
DECLARE
    unix_ts_ms BIGINT;
    uuid_bytes BYTEA;
BEGIN
    unix_ts_ms := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT;
    uuid_bytes := '\x'
        || lpad(to_hex((unix_ts_ms >> 16)::BIGINT), 12, '0')
        || lpad(to_hex((unix_ts_ms & 65535)::BIGINT), 4, '0')
        || lpad(to_hex(((random() * 65535)::INT | 32768)), 4, '0')  -- version 7
        || lpad(to_hex((random() * 281474976710655)::BIGINT), 12, '0');

    RETURN uuid_bytes::UUID;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Create webhook_events table (transient processing state)
CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id VARCHAR(50) NOT NULL,
    channel_id VARCHAR(50) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload TEXT NOT NULL,
    source_ip VARCHAR(45),
    user_agent VARCHAR(500),
    processed BOOLEAN NOT NULL DEFAULT false,
    processing_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Create events table (immutable audit trail)
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    event_type VARCHAR(50) NOT NULL,
    channel_id VARCHAR(255) NOT NULL,
    video_id VARCHAR(255),
    raw_xml TEXT NOT NULL,
    event_hash VARCHAR(64) NOT NULL UNIQUE,
    received_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create subscriptions table (PubSubHubbub lifecycle)
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    channel_id VARCHAR(255) NOT NULL UNIQUE,
    topic_url VARCHAR(500) NOT NULL,
    callback_url VARCHAR(500) NOT NULL,
    subscription_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    lease_seconds INTEGER,
    lease_expires_at TIMESTAMP WITH TIME ZONE,
    next_renewal_at TIMESTAMP WITH TIME ZONE,
    last_renewed_at TIMESTAMP WITH TIME ZONE,
    renewal_attempts INTEGER NOT NULL DEFAULT 0,
    last_renewal_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for webhook_events
CREATE INDEX IF NOT EXISTS idx_webhook_events_video_id ON webhook_events(video_id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_channel_id ON webhook_events(channel_id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_processed ON webhook_events(processed);
CREATE INDEX IF NOT EXISTS idx_webhook_events_processing_status ON webhook_events(processing_status);
CREATE INDEX IF NOT EXISTS idx_webhook_events_created_at ON webhook_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_events_event_type ON webhook_events(event_type);

-- Create indexes for events
CREATE INDEX IF NOT EXISTS idx_events_channel_id ON events(channel_id);
CREATE INDEX IF NOT EXISTS idx_events_video_id ON events(video_id);
CREATE INDEX IF NOT EXISTS idx_events_received_at ON events(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);

-- Create indexes for subscriptions
CREATE INDEX IF NOT EXISTS idx_subscriptions_channel_id ON subscriptions(channel_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(subscription_status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_next_renewal ON subscriptions(next_renewal_at);
CREATE INDEX IF NOT EXISTS idx_subscriptions_lease_expires ON subscriptions(lease_expires_at);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_webhook_events_updated_at
    BEFORE UPDATE ON webhook_events
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Grant permissions (adjust as needed for production)
GRANT ALL PRIVILEGES ON SCHEMA webhook_ingestion TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA webhook_ingestion TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA webhook_ingestion TO postgres;

-- Success message
SELECT 'Database schema initialized successfully' AS status;
