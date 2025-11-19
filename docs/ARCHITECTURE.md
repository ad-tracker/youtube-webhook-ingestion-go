# YouTube Webhook Ingestion - Architecture

## Project Overview

This is a production-ready YouTube PubSubHubbub webhook ingestion service written in Go. It receives, parses, and stores YouTube video notifications with a focus on:
- Event sourcing (immutable webhook events)
- Full audit trail (video update history)
- Type-safe repository pattern
- Comprehensive error handling
- PostgreSQL-first design

**Tech Stack:**
- Go 1.25.3
- PostgreSQL with pgx/v5
- HTTP server with net/http
- Structured logging (slog)
- Database migrations (golang-migrate)
- Asynq for job queuing (Redis-backed)
- Google YouTube Data API v3

## Project Structure

```
youtube-webhook-ingestion-go/
├── cmd/                              # Entry points
│   ├── server/main.go                # HTTP webhook server
│   ├── enricher/main.go              # Video enrichment worker
│   ├── renewer/main.go               # Subscription renewal service
│   └── migrate/main.go               # Database migration CLI
├── internal/                         # Private packages
│   ├── db/                           # Database layer
│   │   ├── models/                   # Data structures
│   │   │   ├── webhook_event.go     # Raw webhook events (immutable)
│   │   │   ├── video.go             # Video metadata
│   │   │   ├── channel.go           # Channel metadata
│   │   │   └── video_update.go      # Update audit trail
│   │   ├── repository/              # Data access layer
│   │   │   ├── webhook_event.go     # Webhook event CRUD
│   │   │   ├── video.go             # Video CRUD
│   │   │   ├── channel.go           # Channel CRUD
│   │   │   └── video_update.go      # Update history CRUD
│   │   ├── testutil/                # Test utilities
│   │   ├── db.go                    # Connection pool management
│   │   ├── errors.go                # Custom error types
│   │   └── hash.go                  # SHA-256 hashing
│   ├── parser/                      # XML parsing
│   │   └── atom.go                  # YouTube Atom feed parser
│   ├── service/                     # Business logic
│   │   ├── processor.go             # Event processing service
│   │   ├── pubsubhub.go             # PubSubHub subscription management
│   │   ├── channel_resolver.go      # Channel ID resolution
│   │   └── youtube/                 # YouTube API client
│   ├── handler/                     # HTTP handlers
│   │   ├── webhook.go               # PubSubHubbub endpoint
│   │   ├── subscription.go          # Subscription management
│   │   └── crud.go                  # Database CRUD endpoints
│   ├── middleware/                  # HTTP middleware
│   │   └── auth.go                  # API key authentication
│   └── queue/                       # Job queue
│       ├── tasks.go                 # Task definitions
│       ├── client.go                # Queue client
│       └── handler.go               # Task handlers
├── migrations/                      # Database schemas
│   ├── 000001_create_webhook_events.{up,down}.sql
│   ├── 000002_create_channels.{up,down}.sql
│   ├── 000003_create_videos.{up,down}.sql
│   ├── 000004_create_video_updates.{up,down}.sql
│   ├── 000005_create_pubsub_subscriptions.{up,down}.sql
│   ├── 000006_create_video_api_enrichments.{up,down}.sql
│   ├── 000007_create_api_quota_usage.{up,down}.sql
│   ├── 000008_create_enrichment_jobs.{up,down}.sql
│   └── 000009_create_channel_api_enrichments.{up,down}.sql
├── docs/                            # Documentation
│   ├── ARCHITECTURE.md              # This file
│   ├── API.md                       # API documentation
│   ├── AUTHENTICATION.md            # Authentication guide
│   └── database-schema.md           # Database schema details
├── go.mod                           # Module definition
└── README.md                        # Main documentation
```

## Architecture Layers

```
HTTP Request
    ↓
WebhookHandler (handler/)
    ├─ Verifies signature
    ├─ Logs request
    └─ Routes to processor
        ↓
    EventProcessor (service/)
        ├─ Parses XML
        ├─ Checks for duplicates
        └─ Updates database
            ↓
        Repositories (repository/)
            ├─ WebhookEventRepository
            ├─ VideoRepository
            ├─ ChannelRepository
            └─ VideoUpdateRepository
                ↓
            PostgreSQL Database
```

## Request Processing Flow

```
YouTube Server
    │
    ├─ GET /webhook?hub.challenge=...     [Subscription Verification]
    │   │
    │   └─→ WebhookHandler.handleVerification()
    │       ├─ Extract hub.challenge parameter
    │       ├─ Log verification request
    │       └─→ HTTP 200 + challenge value
    │
    └─ POST /webhook + Atom XML           [Notification]
        │
        └─→ WebhookHandler.handleNotification()
            │
            ├─ Read request body
            ├─ Verify X-Hub-Signature (HMAC-SHA1)
            │   └─ Constant-time comparison
            │
            └─→ EventProcessor.ProcessEvent()
                │
                ├─ AtomParser.ParseAtomFeed()
                │   ├─ Unmarshal XML
                │   ├─ Validate required fields
                │   └─ Return VideoData
                │
                ├─ Check for deleted videos
                │   └─ If deleted: Create webhook_event + return
                │
                ├─ WebhookEventRepository.CreateWebhookEvent()
                │   ├─ Generate SHA-256 content hash
                │   ├─ Check unique constraint
                │   └─→ SQL INSERT
                │
                ├─ IF duplicate: Return nil (idempotent)
                │
                ├─ Transaction.Begin()
                │   │
                │   ├─ VideoRepository.GetVideoByID()
                │   │   └─ Determine update type
                │   │
                │   ├─ ChannelRepository.UpsertChannel()
                │   │   └─→ INSERT ... ON CONFLICT DO UPDATE
                │   │
                │   ├─ VideoRepository.UpsertVideo()
                │   │   └─→ INSERT ... ON CONFLICT DO UPDATE
                │   │
                │   ├─ VideoUpdateRepository.CreateVideoUpdate()
                │   │   └─→ INSERT (immutable audit record)
                │   │
                │   └─ Transaction.Commit()
                │
                ├─ WebhookEventRepository.MarkEventProcessed()
                │   └─→ UPDATE processed, processed_at, processing_error
                │
                └─→ HTTP 200 OK
```

## Database Schema

### Overview (9 Tables)

#### 1. webhook_events (Immutable Event Store)
```sql
CREATE TABLE webhook_events (
    id BIGSERIAL PRIMARY KEY,
    raw_xml TEXT,
    content_hash VARCHAR(64) UNIQUE,    -- SHA-256 for deduplication
    received_at TIMESTAMPTZ,
    processed BOOLEAN,
    processed_at TIMESTAMPTZ,          -- Only mutable fields
    processing_error TEXT,             -- Only mutable fields
    video_id VARCHAR(20),
    channel_id VARCHAR(30),
    created_at TIMESTAMPTZ
);
```

**Key Features:**
- Immutable by design (trigger prevents deletes and updates to core fields)
- Only `processed`, `processed_at`, `processing_error` can be updated
- Content hash deduplication (unique constraint)
- Indexed on: received_at, processed status, video_id, channel_id

#### 2. channels (Normalized Channel Data)
```sql
CREATE TABLE channels (
    channel_id VARCHAR(30) PRIMARY KEY,
    title VARCHAR(500),
    channel_url VARCHAR(500),
    first_seen_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

#### 3. videos (Normalized Video Data)
```sql
CREATE TABLE videos (
    video_id VARCHAR(20) PRIMARY KEY,
    channel_id VARCHAR(30) REFERENCES channels,
    title VARCHAR(500),
    video_url VARCHAR(500),
    published_at TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

#### 4. video_updates (Immutable Audit Trail)
```sql
CREATE TABLE video_updates (
    id BIGSERIAL PRIMARY KEY,
    webhook_event_id BIGINT REFERENCES webhook_events,
    video_id VARCHAR(20) REFERENCES videos,
    channel_id VARCHAR(30) REFERENCES channels,
    title VARCHAR(500),                -- Snapshot at update time
    published_at TIMESTAMPTZ,
    feed_updated_at TIMESTAMPTZ,
    update_type VARCHAR(50),           -- new_video, title_update, unknown
    created_at TIMESTAMPTZ
);
```

#### 5. pubsub_subscriptions (Subscription Management)
```sql
CREATE TABLE pubsub_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    channel_id VARCHAR(255),
    topic_url TEXT,
    callback_url TEXT,
    hub_url TEXT,
    lease_seconds INTEGER,
    expires_at TIMESTAMPTZ,
    status VARCHAR(50),                -- pending, active, expired, failed
    secret VARCHAR(255),
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

#### 6-9. Enrichment Tables
- **video_api_enrichments**: YouTube API data for videos
- **channel_api_enrichments**: YouTube API data for channels
- **api_quota_usage**: Tracks API quota consumption
- **enrichment_jobs**: Tracks enrichment job status

### Database Relationships

```
┌──────────────────────────────┐
│     webhook_events           │ ◄──────── Event Store (Immutable)
├──────────────────────────────┤
│ content_hash (UNIQUE)        │ ◄──── SHA-256 for deduplication
│ processed (BOOLEAN)          │
│ processed_at, error          │ ◄──── Only mutable fields
└──────────────────────────────┘
        │              │
        │              ├──── has ONE ────────┐
        │              │                      │
        │              └──── has ONE ────┐   │
        ▼                                 ▼   ▼
┌──────────────────────────────┐  ┌──────────────────────────────┐
│     channels                 │  │     videos                   │
├──────────────────────────────┤  ├──────────────────────────────┤
│ channel_id (PK)              │◄─┤ channel_id (FK)              │
│ title                        │  │ video_id (PK)                │
│ channel_url                  │  │ title                        │
└──────────────────────────────┘  │ published_at                 │
        ▲                          └──────────────────────────────┘
        │                                    ▲
        │  references                        │
        │                                    │
┌───────┴──────────────────────────┴────────┐
│     video_updates                         │ ◄──── Audit Trail (Immutable)
├──────────────────────────────────────────┤
│ webhook_event_id (FK)                    │
│ video_id (FK)                            │
│ channel_id (FK)                          │
│ title (snapshot)                         │
│ update_type                              │
└──────────────────────────────────────────┘
```

## API Structure

### HTTP Endpoints

**Public Webhook Endpoints** (no authentication):
- `GET /webhook` - PubSubHubbub subscription verification
- `POST /webhook` - Notification processing (HMAC-protected)
- `GET /health` - Health check

**Protected API Endpoints** (require API key):
- `POST /api/v1/subscriptions` - Create subscription
- `GET /api/v1/subscriptions` - List subscriptions
- `GET /api/v1/webhook-events` - List webhook events
- `GET /api/v1/channels` - List channels
- `GET /api/v1/videos` - List videos
- `GET /api/v1/video-updates` - List video updates
- `POST /api/v1/channels/from-url` - Add channel by URL

### Authentication

API key authentication via:
1. `X-API-Key` header (recommended)
2. `Authorization: Bearer` header

Configured via `API_KEYS` environment variable (comma-separated for multiple keys).

## Key Design Patterns

### 1. Repository Pattern
- Interface-based repositories for testability
- Private struct implementations
- Consistent CRUD operations across all entities

### 2. Service Layer Pattern
- EventProcessor as business logic orchestrator
- Separates HTTP concerns from processing logic
- Facilitates testing with mock repositories

### 3. Dependency Injection
- Handler receives processor, secret, logger
- Processor receives repositories and connection pool
- Repositories receive connection pool

### 4. Immutability by Design
- **webhook_events**: Database-level enforcement via triggers
- **video_updates**: Application-level (no update/delete methods)
- Models: Read-only after construction

### 5. Error Handling
- Custom error types for semantic checking
- Error wrapping with operation context
- PostgreSQL error code mapping

### 6. Context Usage
- All database operations accept `context.Context`
- Supports timeout and cancellation
- Request context threaded through all layers

### 7. Idempotent Processing
- Content hash deduplication
- Duplicate events silently ignored
- Safe to replay events

## Data Transformation Pipeline

```
YouTube Atom Feed XML
    │
    ▼
┌────────────────────────────────┐
│   XML Parsing                  │
│   (encoding/xml)               │
└────────────────────────────────┘
    │
    ├─ AtomFeed
    │   ├─ Entry
    │   │   ├─ videoId
    │   │   ├─ channelId
    │   │   ├─ title
    │   │   ├─ link (href)
    │   │   ├─ published
    │   │   └─ updated
    │   └─ Deleted (if deleted)
    │
    ▼
┌────────────────────────────────┐
│   VideoData (struct)           │
├────────────────────────────────┤
│ VideoID                        │
│ ChannelID                      │
│ Title                          │
│ VideoURL                       │
│ PublishedAt                    │
│ UpdatedAt                      │
│ IsDeleted                      │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│   EventProcessor               │
├────────────────────────────────┤
│ 1. Create WebhookEvent         │
│    ├─ Hash content             │
│    └─ Check duplicates         │
│                                │
│ 2. Upsert Channel              │
│    ├─ channel_id (PK)          │
│    └─ title, url               │
│                                │
│ 3. Upsert Video                │
│    ├─ video_id (PK)            │
│    ├─ channel_id (FK)          │
│    └─ title, url, published_at │
│                                │
│ 4. Create VideoUpdate          │
│    ├─ webhook_event_id (FK)    │
│    ├─ video_id (FK)            │
│    ├─ channel_id (FK)          │
│    ├─ update_type              │
│    └─ snapshot data            │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│   Database State               │
├────────────────────────────────┤
│ webhook_events: Immutable      │
│ channels: Latest               │
│ videos: Latest                 │
│ video_updates: History         │
└────────────────────────────────┘
```

## Enrichment System

### Job Queue Architecture

```
New Video Detected
    │
    ▼
┌────────────────────────────────┐
│ EventProcessor                 │
│ ├─ Enqueues enrichment job     │
│ └─ Creates enrichment_jobs row │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│ Redis (Asynq)                  │
│ ├─ Priority queue              │
│ └─ Automatic retry             │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│ Enricher Worker                │
│ ├─ Fetches video details       │
│ ├─ Uses YouTube Data API v3    │
│ ├─ Tracks quota usage          │
│ └─ Stores enriched data        │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│ video_api_enrichments          │
│ ├─ Statistics                  │
│ ├─ Thumbnails                  │
│ ├─ Category                    │
│ └─ Full metadata               │
└────────────────────────────────┘
```

## Immutability Enforcement

### webhook_events Table (DB-level)

```
Insert: ✅ ALLOWED
  └─→ Any field can be set

Update: ⚠️ PARTIAL
  ├─ processed → ✅ ALLOWED
  ├─ processed_at → ✅ ALLOWED
  ├─ processing_error → ✅ ALLOWED
  │
  ├─ raw_xml → ❌ BLOCKED by trigger
  ├─ content_hash → ❌ BLOCKED by trigger
  ├─ received_at → ❌ BLOCKED by trigger
  ├─ created_at → ❌ BLOCKED by trigger
  ├─ video_id → ❌ BLOCKED by trigger
  └─ channel_id → ❌ BLOCKED by trigger

Delete: ❌ BLOCKED by trigger

Trigger: prevent_webhook_events_modification()
  └─ Raises Exception if violation detected
```

### video_updates Table (App-level)

```
Insert: ✅ ALLOWED
  └─→ Create audit trail entry

Update: ❌ NEVER EXPOSED
  └─ No update methods in interface

Delete: ❌ NEVER EXPOSED
  └─ No delete methods in interface

Strategy: No update/delete methods on repository
  └─→ Immutability enforced at application layer
```

## Configuration & Environment

**Environment Variables:**
- `DATABASE_URL` - PostgreSQL connection string (required)
- `REDIS_URL` - Redis connection for job queue (optional)
- `PORT` - Server port (default: 8080)
- `WEBHOOK_PATH` - Webhook endpoint path (default: /webhook)
- `WEBHOOK_SECRET` - HMAC secret for signature verification (optional)
- `API_KEYS` - Comma-separated API keys for protected endpoints
- `YOUTUBE_API_KEY` - YouTube Data API v3 key (optional)
- `DOMAIN` - Domain name for callback URLs (required for subscriptions)

**Server Configuration:**
- Read timeout: 15 seconds
- Write timeout: 15 seconds
- Idle timeout: 60 seconds
- Shutdown timeout: 30 seconds

**Connection Pool:**
- Max connections: 25
- Min connections: 5
- Max lifetime: 1 hour
- Idle timeout: 30 minutes

## Dependencies

**Direct Dependencies:**
- `github.com/jackc/pgx/v5` - PostgreSQL driver and pooling
- `github.com/golang-migrate/migrate/v4` - Database migrations
- `github.com/hibiken/asynq` - Job queue with Redis backend
- `google.golang.org/api/youtube/v3` - YouTube Data API client
- `github.com/stretchr/testify` - Testing assertions
- `github.com/testcontainers/testcontainers-go` - Integration testing

**Standard Library Usage:**
- `net/http` - HTTP server
- `log/slog` - Structured logging
- `context` - Context management
- `encoding/xml` - XML parsing
- `crypto/hmac`, `crypto/sha1` - Signature verification

## Key Features Summary

✅ Production-ready HTTP server with graceful shutdown
✅ YouTube PubSubHubbub protocol implementation
✅ Event sourcing with immutable event store
✅ Full audit trail of all changes
✅ Duplicate detection and deduplication
✅ HMAC-SHA1 signature verification
✅ Comprehensive error handling
✅ Type-safe repository pattern
✅ Database connection pooling
✅ Structured logging throughout
✅ Database migrations support
✅ Integration tests with real PostgreSQL
✅ API key authentication
✅ YouTube Data API enrichment
✅ Job queue for asynchronous processing
✅ Quota tracking for API usage
