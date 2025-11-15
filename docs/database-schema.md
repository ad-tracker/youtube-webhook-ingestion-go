# Database Schema Design for YouTube PubSubHubbub Webhook Events

## Overview

This document describes the PostgreSQL 17 database schema designed to store and process YouTube PubSubHubbub notification events. The schema is optimized for event sourcing, data integrity, and efficient querying.

## Design Principles

1. **Event Immutability**: Raw webhook events are never modified or deleted
2. **Data Normalization**: Channels and videos are normalized to avoid duplication
3. **Audit Trail**: Full history of all video updates is maintained
4. **Query Optimization**: Indexes are strategically placed for common query patterns
5. **Data Integrity**: Foreign key constraints ensure referential integrity

## Schema Diagram

```
webhook_events (immutable)
    ↓
video_updates
    ↓ ↓
videos ← channels
```

## Table Definitions

### 1. webhook_events (Immutable Event Store)

Stores the complete raw XML payload from YouTube PubSubHubbub notifications along with metadata about when the event was received and processed.

```sql
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
        OLD.created_at IS DISTINCT FROM NEW.created_at
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
```

**Key Features:**
- `content_hash`: SHA-256 hash of raw_xml for deduplication
- `processed`: Flag to track processing status (only field allowed to update)
- `processing_error`: Stores any errors that occurred during processing
- Trigger prevents deletion and modification of immutable fields

### 2. channels

Stores normalized channel information extracted from webhook notifications.

```sql
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
```

**Key Features:**
- `channel_id`: YouTube channel ID (e.g., "UCxxxxxxxxxxxxxx")
- `first_seen_at`: When we first encountered this channel
- `last_updated_at`: When we last received an event from this channel

### 3. videos

Stores normalized video information. This table is updated when we receive newer information about a video.

```sql
CREATE TABLE videos (
    video_id VARCHAR(20) PRIMARY KEY,
    channel_id VARCHAR(30) NOT NULL REFERENCES channels(channel_id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    video_url VARCHAR(500) NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,

    -- Metadata
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_videos_channel_id ON videos(channel_id);
CREATE INDEX idx_videos_published_at ON videos(published_at DESC);
CREATE INDEX idx_videos_last_updated_at ON videos(last_updated_at DESC);
CREATE INDEX idx_videos_title ON videos(title);
```

**Key Features:**
- `video_id`: YouTube video ID (e.g., "dQw4w9WgXcQ")
- `published_at`: Original publication date from YouTube
- `first_seen_at`: When we first encountered this video
- `last_updated_at`: When we last received an event for this video

### 4. video_updates

Tracks the complete history of all video updates detected through webhook notifications. This provides an audit trail of all changes.

```sql
CREATE TABLE video_updates (
    id BIGSERIAL PRIMARY KEY,
    webhook_event_id BIGINT NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    video_id VARCHAR(20) NOT NULL REFERENCES videos(video_id) ON DELETE CASCADE,
    channel_id VARCHAR(30) NOT NULL REFERENCES channels(channel_id) ON DELETE CASCADE,

    -- Snapshot of data at this point in time
    title VARCHAR(500) NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    feed_updated_at TIMESTAMPTZ NOT NULL,

    -- Update type detection
    update_type VARCHAR(50) NOT NULL, -- 'new_video', 'title_update', 'description_update', 'unknown'

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_video_updates_webhook_event_id ON video_updates(webhook_event_id);
CREATE INDEX idx_video_updates_video_id ON video_updates(video_id, created_at DESC);
CREATE INDEX idx_video_updates_channel_id ON video_updates(channel_id, created_at DESC);
CREATE INDEX idx_video_updates_update_type ON video_updates(update_type);
CREATE INDEX idx_video_updates_created_at ON video_updates(created_at DESC);
```

**Key Features:**
- Links back to the original `webhook_event_id` for full traceability
- `feed_updated_at`: The `<updated>` timestamp from the Atom feed
- `update_type`: Categorizes the type of update (new upload, title change, etc.)
- Maintains chronological history for each video

## Event Processing Flow

1. **Receive Webhook**: Raw XML is stored in `webhook_events` with `processed=false`
2. **Parse Event**: XML is parsed to extract video and channel information
3. **Upsert Channel**: Channel info is created or updated in `channels` table
4. **Upsert Video**: Video info is created or updated in `videos` table
5. **Record Update**: A new record is inserted into `video_updates`
6. **Mark Processed**: `webhook_events.processed` is set to `true`

If any error occurs during processing, it's logged in `webhook_events.processing_error` and the event remains unprocessed for retry.

## Common Queries

### Get all unprocessed events
```sql
SELECT * FROM webhook_events
WHERE NOT processed
ORDER BY received_at ASC;
```

### Get update history for a video
```sql
SELECT vu.*, we.raw_xml
FROM video_updates vu
JOIN webhook_events we ON vu.webhook_event_id = we.id
WHERE vu.video_id = 'VIDEO_ID'
ORDER BY vu.created_at DESC;
```

### Get recent videos from a channel
```sql
SELECT * FROM videos
WHERE channel_id = 'CHANNEL_ID'
ORDER BY published_at DESC
LIMIT 50;
```

### Get channels with most recent activity
```sql
SELECT c.*, COUNT(v.video_id) as video_count
FROM channels c
LEFT JOIN videos v ON c.channel_id = v.channel_id
GROUP BY c.channel_id
ORDER BY c.last_updated_at DESC;
```

### Detect title changes for a video
```sql
SELECT
    created_at,
    title,
    LAG(title) OVER (ORDER BY created_at) as previous_title
FROM video_updates
WHERE video_id = 'VIDEO_ID'
ORDER BY created_at;
```

## Data Retention Considerations

- **webhook_events**: Retain indefinitely for audit purposes (consider archiving old processed events)
- **video_updates**: Retain indefinitely for historical analysis
- **videos**: Retain as long as the channel is being monitored
- **channels**: Retain as long as being monitored

Consider implementing a separate archival strategy for webhook_events older than a certain threshold (e.g., 1 year) by moving them to a separate archive table or cold storage.

## Migration Strategy

Tables should be created in the following order to respect foreign key dependencies:

1. `webhook_events`
2. `channels`
3. `videos`
4. `video_updates`

## Security Considerations

- Database user for the application should have:
  - `SELECT, INSERT` on `webhook_events`
  - `UPDATE` on `webhook_events` (only for `processed`, `processed_at`, `processing_error` columns)
  - `SELECT, INSERT, UPDATE` on `channels`, `videos`
  - `SELECT, INSERT` on `video_updates`
  - No `DELETE` permissions on any table (except for testing/development)

## Performance Considerations

- All timestamps use `TIMESTAMPTZ` for timezone awareness
- Indexes are created on frequently queried columns
- `content_hash` prevents duplicate event storage
- Consider partitioning `webhook_events` and `video_updates` by date for very large deployments

## YouTube PubSubHubbub XML Format Reference

The webhook notifications arrive in this Atom feed format:

```xml
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015"
      xmlns="http://www.w3.org/2005/Atom">
  <link rel="hub" href="https://pubsubhubbub.appspot.com"/>
  <link rel="self" href="https://www.youtube.com/xml/feeds/videos.xml?channel_id=CHANNEL_ID"/>
  <title>YouTube video feed</title>
  <updated>2015-04-01T19:05:24.552394234+00:00</updated>
  <entry>
    <id>yt:video:VIDEO_ID</id>
    <yt:videoId>VIDEO_ID</yt:videoId>
    <yt:channelId>CHANNEL_ID</yt:channelId>
    <title>Video title</title>
    <link rel="alternate" href="http://www.youtube.com/watch?v=VIDEO_ID"/>
    <author>
     <name>Channel title</name>
     <uri>http://www.youtube.com/channel/CHANNEL_ID</uri>
    </author>
    <published>2015-03-06T21:40:57+00:00</published>
    <updated>2015-03-09T19:05:24.552394234+00:00</updated>
  </entry>
</feed>
```

## Field Mapping

| XML Path | Database Table | Column |
|----------|---------------|---------|
| `//yt:videoId` | videos | video_id |
| `//yt:channelId` | videos, channels | channel_id |
| `//entry/title` | videos, video_updates | title |
| `//entry/link[@rel='alternate']/@href` | videos | video_url |
| `//entry/published` | videos, video_updates | published_at |
| `//entry/updated` | video_updates | feed_updated_at |
| `//author/name` | channels | title |
| `//author/uri` | channels | channel_url |
| Entire feed | webhook_events | raw_xml |
