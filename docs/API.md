# API Reference

Complete API documentation for the YouTube Webhook Ingestion service.

## Table of Contents

- [Authentication](#authentication)
- [Common Responses](#common-responses)
- [Subscription Management API](#subscription-management-api)
- [Webhook Events API](#webhook-events-api)
- [Channels API](#channels-api)
- [Videos API](#videos-api)
- [Video Updates API](#video-updates-api)
- [Sponsors and Sponsor Detection API](#sponsors-and-sponsor-detection-api)
- [Channel from URL API](#channel-from-url-api)
- [Error Handling](#error-handling)
- [Examples](#examples)

## Authentication

### Protected Endpoints

All CRUD and subscription management endpoints require API key authentication. The webhook and health endpoints do NOT require authentication.

**Protected:**
- `/api/v1/subscriptions` - Subscription management
- `/api/v1/webhook-events` - Webhook event queries
- `/api/v1/channels` - Channel management
- `/api/v1/videos` - Video management
- `/api/v1/video-updates` - Video update queries
- `/api/v1/sponsors` - Sponsor management and queries
- `/api/v1/sponsor-detection-jobs` - Detection job history
- `/api/v1/channels/from-url` - Add channel by URL

**Public (no authentication):**
- `/webhook` - PubSubHubbub endpoint (HMAC-protected)
- `/health` - Health check

### Authentication Methods

Provide your API key using one of these methods:

#### 1. X-API-Key Header (Recommended)
```http
X-API-Key: your-api-key-here
```

####  2. Authorization Bearer Header
```http
Authorization: Bearer your-api-key-here
```

### Configuration

API keys are configured via the `API_KEYS` environment variable (comma-separated for multiple keys):

```bash
# Single key
export API_KEYS="sk_live_abc123def456"

# Multiple keys (for rotation or multiple clients)
export API_KEYS="sk_live_abc123,sk_test_def456,sk_prod_ghi789"
```

### Unauthorized Response

If the API key is missing or invalid:

**401 Unauthorized**
```json
{
  "error": "Unauthorized"
}
```

## Common Responses

### Success Responses
- `200 OK` - Successful GET/PUT/PATCH request
- `201 Created` - Successful POST request
- `204 No Content` - Successful DELETE request

### Error Responses
- `400 Bad Request` - Invalid request parameters or validation error
- `401 Unauthorized` - Missing or invalid API key
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource already exists or constraint violation
- `500 Internal Server Error` - Server or database error

---

## Subscription Management API

Manage YouTube PubSubHubbub subscriptions for real-time video notifications.

### Create Subscription

**POST** `/api/v1/subscriptions`

Creates a new PubSubHubbub subscription for a YouTube channel.

**Authentication:** Required

#### Request Body

```json
{
  "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
  "callback_url": "https://yourdomain.com/webhook",
  "lease_seconds": 432000,
  "secret": "optional-secret-for-hmac-verification"
}
```

**Fields:**
- `channel_id` (string, required): YouTube channel ID (must start with "UC" + 22 characters)
- `callback_url` (string, required): HTTPS URL where YouTube will send notifications
- `lease_seconds` (integer, optional): Subscription duration in seconds. Default: 432000 (5 days). Max: 864000 (10 days)
- `secret` (string, optional): Secret key for HMAC signature verification of incoming webhooks

#### Response

**201 Created**

```json
{
  "id": 1,
  "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
  "topic_url": "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx",
  "callback_url": "https://yourdomain.com/webhook",
  "hub_url": "https://pubsubhubbub.appspot.com/subscribe",
  "lease_seconds": 432000,
  "expires_at": "2025-11-23T10:30:00Z",
  "status": "active",
  "secret": "optional-secret-for-hmac-verification",
  "last_verified_at": "2025-11-18T10:30:00Z",
  "created_at": "2025-11-18T10:30:00Z",
  "updated_at": "2025-11-18T10:30:00Z"
}
```

**Subscription Status Values:**
- `pending` - Subscription request sent, waiting for verification
- `active` - Subscription verified and active
- `expired` - Subscription lease has expired
- `failed` - Subscription request failed

### Get Subscriptions

**GET** `/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx`

Retrieves all subscriptions for a specific channel.

**Authentication:** Required

#### Query Parameters
- `channel_id` (string, required): YouTube channel ID

#### Response

**200 OK**

```json
{
  "subscriptions": [
    {
      "id": 1,
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "topic_url": "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx",
      "callback_url": "https://yourdomain.com/webhook",
      "hub_url": "https://pubsubhubbub.appspot.com/subscribe",
      "lease_seconds": 432000,
      "expires_at": "2025-11-23T10:30:00Z",
      "status": "active",
      "created_at": "2025-11-18T10:30:00Z",
      "updated_at": "2025-11-18T10:30:00Z"
    }
  ],
  "count": 1
}
```

---

## Webhook Events API

Query webhook events received from YouTube.

### List Webhook Events

**GET** `/api/v1/webhook-events`

Retrieves a paginated list of webhook events with optional filters.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `processed` (boolean, optional): Filter by processing status
- `video_id` (string, optional): Filter by video ID
- `channel_id` (string, optional): Filter by channel ID
- `order_by` (string, optional): Sort field (default: `received_at`)
- `order` (string, optional): Sort direction - `asc` or `desc` (default: `desc`)

#### Response

**200 OK**

```json
{
  "webhook_events": [
    {
      "id": 12345,
      "raw_xml": "<feed>...</feed>",
      "content_hash": "abc123...",
      "received_at": "2025-11-18T10:30:00Z",
      "processed": true,
      "processed_at": "2025-11-18T10:30:05Z",
      "processing_error": null,
      "video_id": "dQw4w9WgXcQ",
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "created_at": "2025-11-18T10:30:00Z"
    }
  ],
  "count": 1,
  "total": 1234,
  "limit": 50,
  "offset": 0
}
```

### Get Webhook Event

**GET** `/api/v1/webhook-events/{id}`

Retrieves a specific webhook event by ID.

**Authentication:** Required

---

## Channels API

Manage channel information.

### List Channels

**GET** `/api/v1/channels`

Retrieves a paginated list of channels.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `title` (string, optional): Filter by title (case-insensitive partial match)
- `order_by` (string, optional): Sort field - `channel_id`, `title`, `last_updated_at` (default: `last_updated_at`)
- `order` (string, optional): Sort direction - `asc` or `desc` (default: `desc`)

#### Response

**200 OK**

```json
{
  "channels": [
    {
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "title": "Example Channel",
      "channel_url": "https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx",
      "first_seen_at": "2025-11-18T10:30:00Z",
      "last_updated_at": "2025-11-18T10:35:00Z",
      "created_at": "2025-11-18T10:30:00Z",
      "updated_at": "2025-11-18T10:35:00Z"
    }
  ],
  "count": 1,
  "total": 250,
  "limit": 50,
  "offset": 0
}
```

### Get Channel

**GET** `/api/v1/channels/{channel_id}`

Retrieves a specific channel.

**Authentication:** Required

---

## Videos API

Manage video information.

### List Videos

**GET** `/api/v1/videos`

Retrieves a paginated list of videos.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `channel_id` (string, optional): Filter by channel ID
- `title` (string, optional): Filter by title (case-insensitive partial match)
- `published_after` (timestamp, optional): Filter videos published after this date
- `published_before` (timestamp, optional): Filter videos published before this date
- `order_by` (string, optional): Sort field (default: `published_at`)
- `order` (string, optional): Sort direction - `asc` or `desc` (default: `desc`)

#### Response

**200 OK**

```json
{
  "videos": [
    {
      "video_id": "dQw4w9WgXcQ",
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "title": "Example Video Title",
      "video_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
      "published_at": "2025-11-15T14:00:00Z",
      "first_seen_at": "2025-11-18T10:30:00Z",
      "last_updated_at": "2025-11-18T10:35:00Z",
      "created_at": "2025-11-18T10:30:00Z",
      "updated_at": "2025-11-18T10:35:00Z"
    }
  ],
  "count": 1,
  "total": 5000,
  "limit": 50,
  "offset": 0
}
```

### Get Video

**GET** `/api/v1/videos/{video_id}`

Retrieves a specific video.

**Authentication:** Required

---

## Video Updates API

Query the immutable audit trail of video changes.

### List Video Updates

**GET** `/api/v1/video-updates`

Retrieves a paginated list of video updates.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `video_id` (string, optional): Filter by video ID
- `channel_id` (string, optional): Filter by channel ID
- `webhook_event_id` (integer, optional): Filter by webhook event ID
- `update_type` (string, optional): Filter by update type - `new_video`, `title_update`, `unknown`
- `order_by` (string, optional): Sort field (default: `created_at`)
- `order` (string, optional): Sort direction - `asc` or `desc` (default: `desc`)

#### Response

**200 OK**

```json
{
  "video_updates": [
    {
      "id": 67890,
      "webhook_event_id": 12345,
      "video_id": "dQw4w9WgXcQ",
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "title": "Video Title at This Point",
      "published_at": "2025-11-15T14:00:00Z",
      "feed_updated_at": "2025-11-18T10:30:00Z",
      "update_type": "title_update",
      "created_at": "2025-11-18T10:30:00Z"
    }
  ],
  "count": 1,
  "total": 15000,
  "limit": 50,
  "offset": 0
}
```

**Note:** Video updates are immutable and cannot be modified or deleted. They serve as an audit trail.

---

## Sponsors and Sponsor Detection API

Manage sponsors detected in video content and track detection job history.

### List Sponsors

**GET** `/api/v1/sponsors`

Retrieves a paginated list of sponsors with sorting and filtering options.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `sort_by` (string, optional): Sort field - `video_count`, `name`, `last_seen`, `created` (default: `video_count`)
- `order` (string, optional): Sort direction - `asc` or `desc` (default: `desc` for video_count/last_seen/created, `asc` for name)
- `category` (string, optional): Filter by sponsor category (case-insensitive)

#### Response

**200 OK**

```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "NordVPN",
      "normalized_name": "nordvpn",
      "category": "VPN",
      "website_url": "https://nordvpn.com",
      "description": "Virtual Private Network service",
      "first_seen_at": "2025-10-01T10:00:00Z",
      "last_seen_at": "2025-11-20T15:30:00Z",
      "video_count": 42,
      "created_at": "2025-10-01T10:00:00Z",
      "updated_at": "2025-11-20T15:30:00Z"
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "Squarespace",
      "normalized_name": "squarespace",
      "category": "Web Hosting",
      "website_url": "https://squarespace.com",
      "description": "Website builder platform",
      "first_seen_at": "2025-09-15T08:00:00Z",
      "last_seen_at": "2025-11-18T12:00:00Z",
      "video_count": 28,
      "created_at": "2025-09-15T08:00:00Z",
      "updated_at": "2025-11-18T12:00:00Z"
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0
}
```

**Fields:**
- `id`: Unique sponsor identifier (UUID)
- `name`: Display name of the sponsor
- `normalized_name`: Lowercase normalized name for deduplication
- `category`: Product/service category (nullable)
- `website_url`: Sponsor's website (nullable)
- `description`: Brief description (nullable)
- `first_seen_at`: First time this sponsor was detected
- `last_seen_at`: Most recent detection
- `video_count`: Total number of videos featuring this sponsor

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sponsors?sort_by=video_count&order=desc&limit=10" \
  -H "X-API-Key: your-api-key-here"
```

### Get Sponsor Details

**GET** `/api/v1/sponsors/{id}`

Retrieves detailed information about a specific sponsor.

**Authentication:** Required

#### Response

**200 OK**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "NordVPN",
  "normalized_name": "nordvpn",
  "category": "VPN",
  "website_url": "https://nordvpn.com",
  "description": "Virtual Private Network service",
  "first_seen_at": "2025-10-01T10:00:00Z",
  "last_seen_at": "2025-11-20T15:30:00Z",
  "video_count": 42,
  "created_at": "2025-10-01T10:00:00Z",
  "updated_at": "2025-11-20T15:30:00Z"
}
```

**404 Not Found**

```json
{
  "error": "not found",
  "message": "sponsor with id '550e8400-e29b-41d4-a716-446655440000' not found"
}
```

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sponsors/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: your-api-key-here"
```

### Get Videos for Sponsor

**GET** `/api/v1/sponsors/{id}/videos`

Retrieves all videos featuring a specific sponsor with enriched details.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)

#### Response

**200 OK**

```json
{
  "items": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "video_id": "dQw4w9WgXcQ",
      "video_title": "Ultimate Tech Review 2025",
      "video_url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
      "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
      "published_at": "2025-11-15T14:00:00Z",
      "sponsor_id": "550e8400-e29b-41d4-a716-446655440000",
      "sponsor_name": "NordVPN",
      "sponsor_category": "VPN",
      "confidence": 0.95,
      "evidence": "Mentioned 'protect your online privacy with NordVPN' at 2:30 and showed promo code",
      "detected_at": "2025-11-16T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

**Fields:**
- `id`: Video-sponsor relationship ID (UUID)
- `video_id`: YouTube video identifier
- `video_title`: Title of the video
- `video_url`: Full YouTube URL
- `channel_id`: YouTube channel identifier
- `published_at`: Video publication timestamp
- `sponsor_id`: Sponsor UUID
- `sponsor_name`: Display name of sponsor
- `sponsor_category`: Product/service category
- `confidence`: LLM confidence score (0.0-1.0)
- `evidence`: Text snippet explaining detection
- `detected_at`: When the sponsor was detected

**404 Not Found**

```json
{
  "error": "not found",
  "message": "sponsor with id '550e8400-e29b-41d4-a716-446655440000' not found"
}
```

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sponsors/550e8400-e29b-41d4-a716-446655440000/videos?limit=20" \
  -H "X-API-Key: your-api-key-here"
```

### Get Sponsors for Video

**GET** `/api/v1/videos/{id}/sponsors`

Retrieves all sponsors detected in a specific video.

**Authentication:** Required

#### Response

**200 OK**

```json
{
  "items": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "video_id": "dQw4w9WgXcQ",
      "sponsor_id": "550e8400-e29b-41d4-a716-446655440000",
      "detection_job_id": "880e8400-e29b-41d4-a716-446655440003",
      "confidence": 0.95,
      "evidence": "Mentioned 'protect your online privacy with NordVPN' at 2:30 and showed promo code",
      "detected_at": "2025-11-16T10:00:00Z",
      "created_at": "2025-11-16T10:00:00Z",
      "updated_at": "2025-11-16T10:00:00Z",
      "sponsor_name": "NordVPN",
      "sponsor_category": "VPN"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440004",
      "video_id": "dQw4w9WgXcQ",
      "sponsor_id": "660e8400-e29b-41d4-a716-446655440001",
      "detection_job_id": "880e8400-e29b-41d4-a716-446655440003",
      "confidence": 0.88,
      "evidence": "Brief mention of Squarespace for website building at 5:45",
      "detected_at": "2025-11-16T10:00:00Z",
      "created_at": "2025-11-16T10:00:00Z",
      "updated_at": "2025-11-16T10:00:00Z",
      "sponsor_name": "Squarespace",
      "sponsor_category": "Web Hosting"
    }
  ],
  "total": 2
}
```

**Fields:**
- `id`: Video-sponsor relationship ID (UUID)
- `video_id`: YouTube video identifier
- `sponsor_id`: Sponsor UUID
- `detection_job_id`: ID of the detection job that found this sponsor
- `confidence`: LLM confidence score (0.0-1.0)
- `evidence`: Text snippet explaining detection
- `detected_at`: When the sponsor was detected
- `sponsor_name`: Display name of sponsor (from JOIN)
- `sponsor_category`: Product/service category (from JOIN, nullable)

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/videos/dQw4w9WgXcQ/sponsors" \
  -H "X-API-Key: your-api-key-here"
```

### Get Sponsors for Channel

**GET** `/api/v1/channels/{id}/sponsors`

Retrieves all sponsors detected across videos from a specific channel.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)

#### Response

**200 OK**

```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "NordVPN",
      "normalized_name": "nordvpn",
      "category": "VPN",
      "website_url": "https://nordvpn.com",
      "description": "Virtual Private Network service",
      "first_seen_at": "2025-10-01T10:00:00Z",
      "last_seen_at": "2025-11-20T15:30:00Z",
      "video_count": 8,
      "created_at": "2025-10-01T10:00:00Z",
      "updated_at": "2025-11-20T15:30:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

**Note:** The `video_count` field in this response represents the number of videos from THIS channel featuring the sponsor, not the global video count.

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/channels/UCxxxxxxxxxxxxxxxxxxxxxx/sponsors?limit=20" \
  -H "X-API-Key: your-api-key-here"
```

### List Sponsor Detection Jobs

**GET** `/api/v1/sponsor-detection-jobs`

Retrieves sponsor detection job history with optional filters.

**Authentication:** Required

#### Query Parameters
- `limit` (integer, optional): Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional): Number of results to skip (default: 0)
- `video_id` (string, optional): Filter by video ID (currently required - listing all jobs not implemented)
- `status` (string, optional): Filter by job status - `pending`, `completed`, `failed`, `skipped`

#### Response

**200 OK**

```json
{
  "items": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "video_id": "dQw4w9WgXcQ",
      "prompt_id": "990e8400-e29b-41d4-a716-446655440005",
      "llm_model": "llama3.2:3b",
      "llm_response_raw": "{\"sponsors\": [{\"name\": \"NordVPN\", \"confidence\": 0.95, \"evidence\": \"Mentioned at 2:30\"}]}",
      "sponsors_detected_count": 2,
      "processing_time_ms": 1250,
      "status": "completed",
      "error_message": null,
      "detected_at": "2025-11-16T10:00:00Z",
      "created_at": "2025-11-16T09:59:30Z",
      "updated_at": "2025-11-16T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

**Fields:**
- `id`: Unique job identifier (UUID)
- `video_id`: YouTube video identifier
- `prompt_id`: ID of the LLM prompt used (UUID, nullable)
- `llm_model`: Name of the LLM model (e.g., "llama3.2:3b", "gpt-4")
- `llm_response_raw`: Raw JSON response from LLM (nullable)
- `sponsors_detected_count`: Number of sponsors found in this job
- `processing_time_ms`: Job duration in milliseconds (nullable)
- `status`: Job status - `pending`, `completed`, `failed`, `skipped`
- `error_message`: Error details if status is `failed` (nullable)
- `detected_at`: When detection completed (nullable for non-completed jobs)

**Status Values:**
- `pending`: Job queued but not started
- `completed`: Successfully analyzed video and extracted sponsors
- `failed`: Job encountered an error (see error_message)
- `skipped`: Video skipped (e.g., no transcript available)

**400 Bad Request** (invalid status filter)

```json
{
  "error": "validation failed",
  "message": "invalid status value (valid: pending, completed, failed, skipped)"
}
```

#### Example Request

```bash
# Get all detection jobs for a specific video
curl -X GET "http://localhost:8080/api/v1/sponsor-detection-jobs?video_id=dQw4w9WgXcQ" \
  -H "X-API-Key: your-api-key-here"

# Get only completed jobs for a video
curl -X GET "http://localhost:8080/api/v1/sponsor-detection-jobs?video_id=dQw4w9WgXcQ&status=completed" \
  -H "X-API-Key: your-api-key-here"
```

### Get Detection Job Details

**GET** `/api/v1/sponsor-detection-jobs/{id}`

Retrieves detailed information about a specific sponsor detection job.

**Authentication:** Required

#### Response

**200 OK**

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "video_id": "dQw4w9WgXcQ",
  "prompt_id": "990e8400-e29b-41d4-a716-446655440005",
  "llm_model": "llama3.2:3b",
  "llm_response_raw": "{\"sponsors\": [{\"name\": \"NordVPN\", \"confidence\": 0.95, \"evidence\": \"Mentioned 'protect your online privacy with NordVPN' at 2:30 and showed promo code\"}, {\"name\": \"Squarespace\", \"confidence\": 0.88, \"evidence\": \"Brief mention of Squarespace for website building at 5:45\"}]}",
  "sponsors_detected_count": 2,
  "processing_time_ms": 1250,
  "status": "completed",
  "error_message": null,
  "detected_at": "2025-11-16T10:00:00Z",
  "created_at": "2025-11-16T09:59:30Z",
  "updated_at": "2025-11-16T10:00:00Z"
}
```

**400 Bad Request** (invalid UUID)

```json
{
  "error": "invalid job ID",
  "message": "job ID must be a valid UUID"
}
```

**404 Not Found**

```json
{
  "error": "not found",
  "message": "detection job with id '880e8400-e29b-41d4-a716-446655440003' not found"
}
```

#### Example Request

```bash
curl -X GET "http://localhost:8080/api/v1/sponsor-detection-jobs/880e8400-e29b-41d4-a716-446655440003" \
  -H "X-API-Key: your-api-key-here"
```

---

## Channel from URL API

Add a channel subscription by providing a YouTube channel or video URL. Requires YouTube Data API configuration (`YOUTUBE_API_KEY`).

### Add Channel from URL

**POST** `/api/v1/channels/from-url`

Resolves a YouTube URL to a channel ID and creates a subscription.

**Authentication:** Required

#### Request Body

```json
{
  "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  "callback_url": "https://yourdomain.com/webhook"
}
```

**Fields:**
- `url` (string, required): YouTube channel URL or video URL
- `callback_url` (string, required): HTTPS URL for webhook notifications

**Supported URL formats:**
- `https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx`
- `https://www.youtube.com/@username`
- `https://www.youtube.com/watch?v=VIDEO_ID`
- `https://youtu.be/VIDEO_ID`

#### Response

**201 Created**

```json
{
  "channel": {
    "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "title": "Channel Name",
    "channel_url": "https://www.youtube.com/channel/UCxxxxxxxxxxxxxxxxxxxxxx"
  },
  "subscription": {
    "id": 1,
    "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "callback_url": "https://yourdomain.com/webhook",
    "status": "active"
  }
}
```

---

## Error Handling

### Standard Error Response Format

All error responses follow this structure:

```json
{
  "error": "Error Type",
  "message": "Detailed error message",
  "details": {
    "field": "additional context if applicable"
  }
}
```

### Common Error Scenarios

#### Validation Errors (400 Bad Request)
```json
{
  "error": "validation failed",
  "message": "invalid channel_id format (must start with 'UC' followed by 22 characters)",
  "details": {
    "field": "channel_id",
    "value": "invalid-id"
  }
}
```

#### Not Found (404 Not Found)
```json
{
  "error": "not found",
  "message": "video with id 'dQw4w9WgXcQ' not found"
}
```

#### Conflict (409 Conflict)
```json
{
  "error": "conflict",
  "message": "subscription already exists for this channel and callback URL"
}
```

---

## Examples

### cURL Examples

#### Create a Subscription
```bash
curl -X POST http://localhost:8080/api/v1/subscriptions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "callback_url": "https://yourdomain.com/webhook",
    "lease_seconds": 432000,
    "secret": "my-webhook-secret"
  }'
```

#### List Videos for a Channel
```bash
curl -X GET "http://localhost:8080/api/v1/videos?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx&limit=100" \
  -H "X-API-Key: your-api-key-here"
```

#### Get Unprocessed Webhook Events
```bash
curl -X GET "http://localhost:8080/api/v1/webhook-events?processed=false&limit=50" \
  -H "X-API-Key: your-api-key-here"
```

#### Add Channel by URL
```bash
curl -X POST http://localhost:8080/api/v1/channels/from-url \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "url": "https://www.youtube.com/@channelname",
    "callback_url": "https://yourdomain.com/webhook"
  }'
```

### Go Client Example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const (
    baseURL = "http://localhost:8080"
    apiKey  = "your-api-key-here"
)

type SubscriptionRequest struct {
    ChannelID    string  `json:"channel_id"`
    CallbackURL  string  `json:"callback_url"`
    LeaseSeconds int     `json:"lease_seconds,omitempty"`
    Secret       *string `json:"secret,omitempty"`
}

func createSubscription(channelID, callbackURL string) error {
    secret := "my-webhook-secret"
    reqBody := SubscriptionRequest{
        ChannelID:    channelID,
        CallbackURL:  callbackURL,
        LeaseSeconds: 432000,
        Secret:       &secret,
    }

    body, _ := json.Marshal(reqBody)

    req, err := http.NewRequest(
        "POST",
        baseURL+"/api/v1/subscriptions",
        bytes.NewReader(body),
    )
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-API-Key", apiKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    return nil
}
```

### Python Client Example

```python
import requests
from typing import Optional, Dict

BASE_URL = "http://localhost:8080"
API_KEY = "your-api-key-here"

class YouTubeWebhookClient:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url
        self.headers = {
            "Content-Type": "application/json",
            "X-API-Key": api_key
        }

    def create_subscription(self, channel_id: str, callback_url: str,
                           lease_seconds: int = 432000,
                           secret: Optional[str] = None) -> Dict:
        """Create a new subscription"""
        payload = {
            "channel_id": channel_id,
            "callback_url": callback_url,
            "lease_seconds": lease_seconds
        }
        if secret:
            payload["secret"] = secret

        response = requests.post(
            f"{self.base_url}/api/v1/subscriptions",
            json=payload,
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def list_videos(self, channel_id: Optional[str] = None,
                   limit: int = 50, offset: int = 0) -> Dict:
        """List videos with optional filters"""
        params = {"limit": limit, "offset": offset}
        if channel_id:
            params["channel_id"] = channel_id

        response = requests.get(
            f"{self.base_url}/api/v1/videos",
            params=params,
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

# Usage
client = YouTubeWebhookClient(BASE_URL, API_KEY)
subscription = client.create_subscription(
    channel_id="UCxxxxxxxxxxxxxxxxxxxxxx",
    callback_url="https://yourdomain.com/webhook",
    secret="my-webhook-secret"
)
print(f"Created subscription: {subscription['id']}")
```

## Best Practices

### 1. API Key Security
- Store API keys in environment variables
- Use HTTPS in production
- Rotate keys regularly
- Use different keys for different environments

### 2. Pagination
- Always use pagination for list endpoints
- Default limit is 50, maximum is 1000
- Use filtering to reduce result sets

### 3. Filtering and Sorting
- Leverage database indexes by filtering on indexed fields
- Use specific filters to reduce load

### 4. Error Handling
- Always check HTTP status codes
- Parse error responses for details
- Implement retry logic for 500/503 errors
- Don't retry 400/401/404/409 errors

### 5. Monitoring
- Log all API requests and responses
- Track error rates by endpoint
- Monitor subscription expiration
- Alert on quota limits (if using YouTube API)

## Environment Configuration

Required environment variables:

```bash
# Required
DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"
API_KEYS="sk_live_abc123,sk_test_def456"

# Optional
PORT="8080"
WEBHOOK_PATH="/webhook"
WEBHOOK_SECRET="your-webhook-secret"
YOUTUBE_API_KEY="your-youtube-api-key"  # Required for /channels/from-url endpoint
REDIS_URL="redis://localhost:6379"      # Required for enrichment jobs
DOMAIN="yourdomain.com"                 # Required for subscriptions
```

## Rate Limiting

Currently, there is no built-in rate limiting. Implement client-side rate limiting to avoid overwhelming the server.

## References

- Architecture details: See [ARCHITECTURE.md](ARCHITECTURE.md)
- Authentication guide: See [AUTHENTICATION.md](AUTHENTICATION.md)
- Database schema: See [database-schema.md](database-schema.md)
- Main README: See [../README.md](../README.md)
