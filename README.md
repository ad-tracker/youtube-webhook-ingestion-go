# YouTube Webhook Ingestion - Database Layer

This is the database layer implementation for a YouTube PubSubHubbub webhook ingestion service in Go.

## Features

- Production-ready PostgreSQL database layer using pgx/v5
- Immutable event sourcing for webhook events
- Full audit trail for video updates
- Comprehensive test coverage with testcontainers
- Type-safe repository pattern
- Proper error handling and context support
- Database migrations with golang-migrate

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

## Database Schema

The schema consists of 4 tables:

1. **webhook_events** (immutable) - Raw webhook notifications from YouTube
2. **channels** - Normalized channel information
3. **videos** - Normalized video information
4. **video_updates** (immutable) - Audit trail of all video updates

See [docs/database-schema.md](docs/database-schema.md) for detailed schema documentation.

## Installation

```bash
go get ad-tracker/youtube-webhook-ingestion
```

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
- **github.com/testcontainers/testcontainers-go** - Integration testing with real PostgreSQL
- **github.com/stretchr/testify** - Testing assertions

## License

See LICENSE file for details.
