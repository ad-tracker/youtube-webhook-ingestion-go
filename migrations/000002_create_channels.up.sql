-- Create channels table
CREATE TABLE channels (
    channel_id VARCHAR(30) PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    channel_url VARCHAR(500) NOT NULL,

    -- Metadata
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for title searches
CREATE INDEX idx_channels_title ON channels(title);
CREATE INDEX idx_channels_last_updated_at ON channels(last_updated_at DESC);
