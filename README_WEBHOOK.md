# YouTube Webhook Ingestion Server

This is a YouTube PubSubHubbub webhook endpoint implementation that receives, processes, and stores YouTube video notifications.

## Components

### 1. Atom Feed Parser (`internal/parser/atom.go`)
Parses YouTube Atom feed XML format with support for:
- Video metadata extraction (ID, channel ID, title, URL, timestamps)
- YouTube-specific XML namespaces (xmlns:yt)
- Deleted video notifications
- Comprehensive error handling for malformed feeds

### 2. Event Processor (`internal/service/processor.go`)
Processes webhook events with transactional guarantees:
- Parses incoming Atom feed XML
- Creates immutable webhook event records
- Updates projection tables (videos, channels, video_updates)
- Determines update type (new_video, title_update, unknown)
- Handles errors gracefully with rollback support

### 3. HTTP Webhook Handler (`internal/handler/webhook.go`)
Handles PubSubHubbub protocol:
- **GET**: Subscription verification (returns hub.challenge)
- **POST**: Notification processing (receives Atom feed)
- Optional HMAC-SHA1 signature verification
- Structured logging for all requests

### 4. HTTP Server (`cmd/server/main.go`)
Production-ready server with:
- Graceful shutdown on SIGINT/SIGTERM
- Health check endpoint
- Request logging middleware
- Database connection pooling
- Environment-based configuration

## Running the Server

### Prerequisites
- Go 1.25.3 or later
- PostgreSQL database with migrations applied
- Environment variables configured

### Configuration

Set the following environment variables:

```bash
# Required
export DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"

# Optional
export PORT=8080                    # Default: 8080
export WEBHOOK_PATH=/webhook        # Default: /webhook
export WEBHOOK_SECRET=your-secret   # Optional: Enable HMAC verification
```

### Build and Run

```bash
# Build the server
go build -o webhook-server ./cmd/server/

# Run the server
./webhook-server
```

Or run directly:

```bash
go run ./cmd/server/main.go
```

### Server Endpoints

- `POST /webhook` - Webhook notification endpoint
- `GET /webhook?hub.challenge=...` - Subscription verification
- `GET /health` - Health check (returns JSON status)

## Testing

### Run All Tests

```bash
# Run all tests
go test ./internal/... -v

# Run specific package tests
go test ./internal/parser -v
go test ./internal/service -v
go test ./internal/handler -v
```

### Test Coverage

The implementation includes comprehensive tests:
- **Parser tests**: 11 test cases covering valid feeds, edge cases, and error conditions
- **Service tests**: 5 test cases with mocked repositories
- **Handler tests**: 10 test cases covering GET/POST, signature verification, and error handling
- **Repository integration tests**: Full database integration tests (existing)

## Example Usage

### Subscription Verification (GET)

```bash
curl "http://localhost:8080/webhook?hub.challenge=test123&hub.mode=subscribe&hub.topic=..."
# Returns: test123
```

### Webhook Notification (POST)

```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/atom+xml" \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>dQw4w9WgXcQ</yt:videoId>
    <yt:channelId>UCuAXFkgsw1L7xaCfnd5JJOw</yt:channelId>
    <title>Test Video</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ"/>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>'
```

### With HMAC Signature

If `WEBHOOK_SECRET` is configured, include the signature header:

```bash
# Calculate signature
echo -n 'XML_CONTENT' | openssl dgst -sha1 -hmac 'your-secret' | awk '{print $2}'

# Send request with signature
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/atom+xml" \
  -H "X-Hub-Signature: sha1=CALCULATED_SIGNATURE" \
  -d 'XML_CONTENT'
```

### Health Check

```bash
curl http://localhost:8080/health
# Returns: {"status":"healthy","database":"connected"}
```

## Architecture

### Data Flow

1. YouTube sends POST request with Atom feed XML
2. Handler verifies HMAC signature (if configured)
3. Processor parses Atom feed
4. Processor creates webhook_events record
5. Processor starts transaction:
   - Checks for existing video
   - Determines update type
   - Upserts channel
   - Upserts video
   - Creates video_update record
6. Processor commits transaction
7. Processor marks webhook event as processed
8. Handler returns 200 OK

### Error Handling

- **Parse errors**: Event rejected with 500 error
- **Duplicate events**: Silently ignored (content hash check)
- **Projection errors**: Event marked as processed with error message
- **Database errors**: Rolled back with detailed error logging

## Production Considerations

1. **HMAC Verification**: Always enable `WEBHOOK_SECRET` in production
2. **Database Pool**: Configured for 25 max connections, 5 min connections
3. **Timeouts**: 15s read/write, 60s idle timeout
4. **Graceful Shutdown**: 30s shutdown timeout for draining connections
5. **Logging**: Structured JSON logging to stdout
6. **Monitoring**: Health endpoint for load balancer checks

## File Structure

```
cmd/server/main.go                      # HTTP server entrypoint
internal/
├── handler/
│   ├── webhook.go                      # HTTP webhook handler
│   └── webhook_test.go                 # Handler tests
├── service/
│   ├── processor.go                    # Event processor
│   └── processor_test.go               # Processor tests
└── parser/
    ├── atom.go                         # Atom feed parser
    └── atom_test.go                    # Parser tests
```

## Protocol References

- [PubSubHubbub Core 0.4](https://pubsubhubbub.github.io/PubSubHubbub/pubsubhubbub-core-0.4.html)
- [YouTube Push Notifications](https://developers.google.com/youtube/v3/guides/push_notifications)
