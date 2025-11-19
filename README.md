# YouTube Webhook Ingestion Service

A production-ready Go service that receives, processes, and stores YouTube video notifications via PubSubHubbub webhooks.

##  What This Does

This service:
1. Receives real-time YouTube webhook notifications when videos are published or updated
2. Parses and validates video metadata from Atom feeds
3. Deduplicates events using content hashing
4. Stores immutable audit trails of all video changes
5. Enriches video data using YouTube Data API v3 (optional)
6. Provides REST API for managing subscriptions and querying data

## Features

- **Event Sourcing**: Immutable webhook event store with SHA-256 deduplication
- **Audit Trail**: Complete history of all video changes
- **YouTube API Integration**: Optional enrichment with full video metadata
- **Job Queue**: Async processing with Redis-backed Asynq
- **REST API**: Full CRUD operations with API key authentication
- **PubSubHubbub Protocol**: Complete implementation with signature verification
- **Production-Ready**: Connection pooling, graceful shutdown, structured logging
- **Type-Safe**: Repository pattern with comprehensive error handling
- **Well-Tested**: Integration tests with real PostgreSQL via testcontainers
- **Database Migrations**: Version-controlled schema with golang-migrate

## Project Structure

```
.
├── cmd/
│   └── migrate/              # Migration CLI tool
│       └── main.go
├── internal/
│   └── db/
│       ├── models/           # Database models (structs)
│       │   ├── channel.go
│       │   ├── video.go
│       │   ├── video_update.go
│       │   └── webhook_event.go
│       ├── repository/       # Repository interfaces and implementations
│       │   ├── channel.go
│       │   ├── channel_test.go
│       │   ├── video.go
│       │   ├── video_test.go
│       │   ├── video_update.go
│       │   ├── video_update_test.go
│       │   ├── webhook_event.go
│       │   └── webhook_event_test.go
│       ├── testutil/         # Test utilities
│       │   └── testutil.go
│       ├── db.go             # Connection pool management
│       ├── errors.go         # Error handling
│       └── hash.go           # Hash generation utilities
├── migrations/               # SQL migration files
│   ├── 000001_create_webhook_events.up.sql
│   ├── 000001_create_webhook_events.down.sql
│   ├── 000002_create_channels.up.sql
│   ├── 000002_create_channels.down.sql
│   ├── 000003_create_videos.up.sql
│   ├── 000003_create_videos.down.sql
│   ├── 000004_create_video_updates.up.sql
│   └── 000004_create_video_updates.down.sql
├── docs/
│   └── database-schema.md   # Database schema documentation
├── go.mod
└── README.md
```

## Quick Start

### Prerequisites
- Go 1.25.3 or later
- PostgreSQL 14+
- (Optional) Redis for job queue
- (Optional) YouTube Data API v3 key

### Run the Server

```bash
# Set environment variables
export DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"
export API_KEYS="your-api-key-here"
export DOMAIN="yourdomain.com"

# Run migrations
go run ./cmd/migrate -direction up

# Start the server
go run ./cmd/server
```

The server will be available at `http://localhost:8080`.

### Test the Webhook

```bash
# Health check
curl http://localhost:8080/health

# Subscription verification
curl "http://localhost:8080/webhook?hub.challenge=test123"
```

## Database Schema

The schema consists of 9 tables:

1. **webhook_events** (immutable) - Raw webhook notifications from YouTube
2. **channels** - Normalized channel information
3. **videos** - Normalized video information
4. **video_updates** (immutable) - Audit trail of all video updates
5. **pubsub_subscriptions** - YouTube PubSubHubbub subscriptions
6. **video_api_enrichments** - Enriched video metadata from YouTube API
7. **channel_api_enrichments** - Enriched channel metadata
8. **api_quota_usage** - YouTube API quota tracking
9. **enrichment_jobs** - Job queue metadata

See [docs/database-schema.md](docs/database-schema.md) for detailed schema documentation.

## Running Migrations

### Using the migration CLI tool

```bash
# Build the migration tool
go build -o migrate ./cmd/migrate

# Run migrations up
./migrate -db "postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable" -direction up

# Run migrations down
./migrate -db "postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable" -direction down

# Or use environment variable
export DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"
./migrate -direction up
```

### Using golang-migrate CLI directly

```bash
# Install migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path ./migrations -database "postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable" up

# Rollback
migrate -path ./migrations -database "postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable" down
```

## Running Tests

The tests use testcontainers to spin up a real PostgreSQL instance, ensuring high confidence in the implementation.

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/db/repository

# Run a specific test
go test -v ./internal/db/repository -run TestWebhookEventRepository_CreateWebhookEvent
```

**Requirements:**
- Docker must be installed and running (for testcontainers)
- Tests will automatically create and destroy PostgreSQL containers

## Usage Examples

### Creating a Connection Pool

```go
package main

import (
    "context"
    "log"

    "ad-tracker/youtube-webhook-ingestion/internal/db"
)

func main() {
    ctx := context.Background()

    cfg := &db.Config{
        Host:     "localhost",
        Port:     5432,
        User:     "postgres",
        Password: "postgres",
        Database: "youtube_webhooks",
        SSLMode:  "disable",
        MaxConns: 25,
        MinConns: 5,
    }

    pool, err := db.NewPool(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to create connection pool: %v", err)
    }
    defer db.Close(pool)

    // Use pool with repositories...
}
```

### Using Webhook Event Repository

```go
package main

import (
    "context"
    "log"

    "ad-tracker/youtube-webhook-ingestion/internal/db"
    "ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

func main() {
    ctx := context.Background()
    pool, _ := db.NewPool(ctx, db.DefaultConfig())
    defer db.Close(pool)

    // Create repository
    webhookRepo := repository.NewWebhookEventRepository(pool)

    // Create a new webhook event
    rawXML := `<feed xmlns="http://www.w3.org/2005/Atom">...</feed>`
    event, err := webhookRepo.CreateWebhookEvent(ctx, rawXML, "videoID123", "channelID456")
    if err != nil {
        log.Fatalf("Failed to create webhook event: %v", err)
    }
    log.Printf("Created webhook event with ID: %d", event.ID)

    // Get unprocessed events
    unprocessed, err := webhookRepo.GetUnprocessedEvents(ctx, 100)
    if err != nil {
        log.Fatalf("Failed to get unprocessed events: %v", err)
    }

    for _, evt := range unprocessed {
        // Process event...

        // Mark as processed
        err := webhookRepo.MarkEventProcessed(ctx, evt.ID, "")
        if err != nil {
            log.Printf("Failed to mark event as processed: %v", err)
        }
    }
}
```

### Using Channel and Video Repositories

```go
package main

import (
    "context"
    "log"
    "time"

    "ad-tracker/youtube-webhook-ingestion/internal/db"
    "ad-tracker/youtube-webhook-ingestion/internal/db/models"
    "ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

func main() {
    ctx := context.Background()
    pool, _ := db.NewPool(ctx, db.DefaultConfig())
    defer db.Close(pool)

    channelRepo := repository.NewChannelRepository(pool)
    videoRepo := repository.NewVideoRepository(pool)

    // Upsert channel
    channel := models.NewChannel(
        "UC1234567890",
        "Example Channel",
        "https://youtube.com/channel/UC1234567890",
    )
    err := channelRepo.UpsertChannel(ctx, channel)
    if err != nil {
        log.Fatalf("Failed to upsert channel: %v", err)
    }

    // Upsert video
    publishedAt := time.Now().Add(-24 * time.Hour)
    video := models.NewVideo(
        "dQw4w9WgXcQ",
        "UC1234567890",
        "Example Video",
        "https://youtube.com/watch?v=dQw4w9WgXcQ",
        publishedAt,
    )
    err = videoRepo.UpsertVideo(ctx, video)
    if err != nil {
        log.Fatalf("Failed to upsert video: %v", err)
    }

    // Get videos for channel
    videos, err := videoRepo.GetVideosByChannelID(ctx, "UC1234567890", 50)
    if err != nil {
        log.Fatalf("Failed to get videos: %v", err)
    }

    for _, v := range videos {
        log.Printf("Video: %s - %s", v.VideoID, v.Title)
    }
}
```

### Using Video Update Repository

```go
package main

import (
    "context"
    "log"
    "time"

    "ad-tracker/youtube-webhook-ingestion/internal/db"
    "ad-tracker/youtube-webhook-ingestion/internal/db/models"
    "ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

func main() {
    ctx := context.Background()
    pool, _ := db.NewPool(ctx, db.DefaultConfig())
    defer db.Close(pool)

    updateRepo := repository.NewVideoUpdateRepository(pool)

    // Create video update
    publishedAt := time.Now().Add(-24 * time.Hour)
    feedUpdatedAt := time.Now()

    update := models.NewVideoUpdate(
        1, // webhook_event_id
        "videoID123",
        "channelID456",
        "Video Title",
        publishedAt,
        feedUpdatedAt,
        models.UpdateTypeNewVideo,
    )

    err := updateRepo.CreateVideoUpdate(ctx, update)
    if err != nil {
        log.Fatalf("Failed to create video update: %v", err)
    }

    // Get update history for a video
    history, err := updateRepo.GetUpdatesByVideoID(ctx, "videoID123", 100)
    if err != nil {
        log.Fatalf("Failed to get update history: %v", err)
    }

    for _, h := range history {
        log.Printf("Update: %s - %s (type: %s)", h.CreatedAt, h.Title, h.UpdateType)
    }
}
```

## Error Handling

The package provides custom error types for common database errors:

```go
import (
    "ad-tracker/youtube-webhook-ingestion/internal/db"
)

event, err := webhookRepo.CreateWebhookEvent(ctx, rawXML, videoID, channelID)
if err != nil {
    if db.IsDuplicateKey(err) {
        // Handle duplicate content hash
        log.Println("Event already exists")
    } else if db.IsForeignKeyViolation(err) {
        // Handle foreign key violation
        log.Println("Referenced record doesn't exist")
    } else if db.IsNotFound(err) {
        // Handle not found
        log.Println("Record not found")
    } else if db.IsImmutableRecord(err) {
        // Handle attempt to modify immutable record
        log.Println("Cannot modify immutable record")
    } else {
        // Handle other errors
        log.Printf("Database error: %v", err)
    }
}
```

## Important Design Decisions

### Immutability

1. **webhook_events**: Enforced at database level with triggers. Only `processed`, `processed_at`, and `processing_error` fields can be updated. No deletes allowed.

2. **video_updates**: Immutability enforced at application level through repository interface (no update/delete methods exposed).

### Content Hash

SHA-256 hashes are used for webhook event deduplication to prevent processing the same event multiple times.

### Thread Safety

All repository implementations are thread-safe and can be used concurrently. The pgx connection pool handles concurrent requests efficiently.

### Context Handling

All database operations accept `context.Context` for proper cancellation and timeout handling.

## Dependencies

- **github.com/jackc/pgx/v5** - PostgreSQL driver and connection pooling
- **github.com/golang-migrate/migrate/v4** - Database migrations
- **github.com/hibiken/asynq** - Job queue with Redis backend
- **google.golang.org/api/youtube/v3** - YouTube Data API client
- **github.com/testcontainers/testcontainers-go** - Integration testing
- **github.com/stretchr/testify** - Testing assertions

## Documentation

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Complete architecture overview, design patterns, and data flows
- **[docs/API.md](docs/API.md)** - REST API reference with authentication and examples
- **[docs/AUTHENTICATION.md](docs/AUTHENTICATION.md)** - API key authentication guide
- **[docs/database-schema.md](docs/database-schema.md)** - Detailed database schema documentation

## Components

### HTTP Server (`cmd/server`)
- PubSubHubbub webhook endpoint
- REST API for subscriptions and data queries
- Health check endpoint
- API key authentication

### Enricher Worker (`cmd/enricher`)
- Processes enrichment jobs from Redis queue
- Fetches video metadata from YouTube Data API v3
- Tracks API quota usage

### Renewal Service (`cmd/renewer`)
- Automatically renews expiring PubSubHubbub subscriptions
- Configurable renewal threshold

### Migration Tool (`cmd/migrate`)
- Applies database schema migrations
- Supports up/down migrations

## License

See LICENSE file for details.
