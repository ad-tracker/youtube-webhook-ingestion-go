-- Create webhook_events table (immutable event store)
CREATE TABLE webhook_events (
    id BIGSERIAL PRIMARY KEY,
    raw_xml TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    processing_error TEXT,

    -- Extracted for indexing/filtering
    video_id VARCHAR(20),
    channel_id VARCHAR(30),

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_webhook_events_received_at ON webhook_events(received_at DESC);
CREATE INDEX idx_webhook_events_processed ON webhook_events(processed) WHERE NOT processed;
CREATE INDEX idx_webhook_events_video_id ON webhook_events(video_id);
CREATE INDEX idx_webhook_events_channel_id ON webhook_events(channel_id);
CREATE UNIQUE INDEX idx_webhook_events_content_hash ON webhook_events(content_hash);

-- Prevent updates and deletes to maintain immutability
CREATE OR REPLACE FUNCTION prevent_webhook_events_modification()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'Deleting webhook events is not allowed';
    ELSIF TG_OP = 'UPDATE' AND OLD.id IS DISTINCT FROM NEW.id THEN
        RAISE EXCEPTION 'Modifying webhook event IDs is not allowed';
    ELSIF TG_OP = 'UPDATE' AND (
        OLD.raw_xml IS DISTINCT FROM NEW.raw_xml OR
        OLD.content_hash IS DISTINCT FROM NEW.content_hash OR
        OLD.received_at IS DISTINCT FROM NEW.received_at OR
        OLD.created_at IS DISTINCT FROM NEW.created_at OR
        OLD.video_id IS DISTINCT FROM NEW.video_id OR
        OLD.channel_id IS DISTINCT FROM NEW.channel_id
    ) THEN
        RAISE EXCEPTION 'Modifying immutable fields in webhook events is not allowed';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_webhook_events_modification
    BEFORE UPDATE OR DELETE ON webhook_events
    FOR EACH ROW
    EXECUTE FUNCTION prevent_webhook_events_modification();
