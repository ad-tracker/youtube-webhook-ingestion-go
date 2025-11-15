# PubSubHubbub Subscription API

This document describes the API endpoint for managing YouTube PubSubHubbub subscriptions.

## Overview

The subscription API allows you to create and manage subscriptions to YouTube channels via Google's PubSubHubbub (PubSub) infrastructure. When you subscribe to a channel, YouTube will send real-time notifications to your callback URL whenever new videos are published or existing videos are updated.

## Authentication

All subscription endpoints require API key authentication. The API key must be provided in one of the following ways:

1. **X-API-Key header** (recommended):
   ```
   X-API-Key: your-api-key-here
   ```

2. **Authorization Bearer header**:
   ```
   Authorization: Bearer your-api-key-here
   ```

The API key is configured via the `API_KEYS` environment variable (comma-separated for multiple keys).

### Unauthorized Response

If the API key is missing or invalid, the endpoint returns:

**401 Unauthorized**
```json
{
  "error": "Unauthorized"
}
```

## Endpoints

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

- `channel_id` (string, required): YouTube channel ID (must start with "UC" followed by 22 characters)
- `callback_url` (string, required): HTTPS URL where YouTube will send notifications
- `lease_seconds` (integer, optional): Subscription duration in seconds. Default: 432000 (5 days). Maximum: 864000 (10 days)
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
  "expires_at": "2025-11-20T13:00:00Z",
  "status": "active",
  "secret": "optional-secret-for-hmac-verification",
  "last_verified_at": "2025-11-15T13:00:00Z",
  "created_at": "2025-11-15T13:00:00Z",
  "updated_at": "2025-11-15T13:00:00Z"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid request parameters
  ```json
  {
    "error": "validation failed",
    "message": "invalid channel_id format (must start with 'UC' followed by 22 characters)"
  }
  ```

- `409 Conflict`: Subscription already exists for this channel and callback URL
  ```json
  {
    "error": "subscription already exists",
    "message": "a subscription for this channel and callback URL already exists"
  }
  ```

- `500 Internal Server Error`: Server error or PubSubHubbub hub error
  ```json
  {
    "error": "failed to subscribe to hub",
    "message": "hub returned error: ..."
  }
  ```

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
      "expires_at": "2025-11-20T13:00:00Z",
      "status": "active",
      "created_at": "2025-11-15T13:00:00Z",
      "updated_at": "2025-11-15T13:00:00Z"
    }
  ],
  "count": 1
}
```

**Error Responses:**

- `400 Bad Request`: Missing channel_id parameter
- `500 Internal Server Error`: Database error

## Subscription Lifecycle

### Subscription Status

Subscriptions can have the following statuses:

- `pending`: Subscription request sent to hub, waiting for verification
- `active`: Subscription verified and active
- `expired`: Subscription lease has expired
- `failed`: Subscription request failed

### Verification Flow

1. When you create a subscription, the service sends a POST request to YouTube's PubSubHubbub hub
2. YouTube responds with `202 Accepted` if the request is valid
3. YouTube then sends a GET request to your `callback_url` with a `hub.challenge` parameter
4. Your webhook handler must respond with the challenge to complete verification
5. Once verified, the subscription status is updated to `active`

### Expiration and Renewal

- Subscriptions expire after `lease_seconds` (default: 5 days)
- You should renew subscriptions before they expire
- You can query subscriptions that are expiring soon and renew them

## Examples

### Create a Subscription (curl)

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

### Get Subscriptions (curl)

```bash
curl -X GET "http://localhost:8080/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx" \
  -H "X-API-Key: your-api-key-here"
```

### Create a Subscription (Go)

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type SubscriptionRequest struct {
    ChannelID    string  `json:"channel_id"`
    CallbackURL  string  `json:"callback_url"`
    LeaseSeconds int     `json:"lease_seconds,omitempty"`
    Secret       *string `json:"secret,omitempty"`
}

func main() {
    secret := "my-webhook-secret"
    req := SubscriptionRequest{
        ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
        CallbackURL:  "https://yourdomain.com/webhook",
        LeaseSeconds: 432000,
        Secret:       &secret,
    }

    body, _ := json.Marshal(req)

    httpReq, _ := http.NewRequest(
        "POST",
        "http://localhost:8080/api/v1/subscriptions",
        bytes.NewReader(body),
    )
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-API-Key", "your-api-key-here")

    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        fmt.Printf("Failed to create subscription: %d\n", resp.StatusCode)
        return
    }

    fmt.Println("Subscription created successfully")
}
```

## Database Schema

The subscriptions are stored in the `pubsub_subscriptions` table:

```sql
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

    CONSTRAINT uq_channel_callback UNIQUE (channel_id, callback_url),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'active', 'expired', 'failed'))
);
```

## Environment Configuration

The server requires the following environment variables:

- `DATABASE_URL` (required): PostgreSQL connection string
- `API_KEYS` (required for subscription endpoints): Comma-separated list of valid API keys
  - Example: `API_KEYS="key1,key2,key3"`
  - If not set, all subscription endpoint requests will be rejected with 401 Unauthorized
- `PORT` (optional): Server port (default: 8080)
- `WEBHOOK_PATH` (optional): Path for webhook endpoint (default: /webhook)
- `WEBHOOK_SECRET` (optional): Secret for webhook HMAC verification

Example:
```bash
export DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"
export API_KEYS="sk_live_abc123,sk_test_def456"
export PORT="8080"
./server
```

## Best Practices

1. **Secure your API keys**: Store API keys securely and never commit them to version control
2. **Use HTTPS for callback URLs**: YouTube requires HTTPS for production webhooks
3. **Implement webhook verification**: Use the `secret` parameter to verify incoming webhooks
4. **Monitor subscription status**: Regularly check for expired subscriptions
5. **Renew before expiration**: Renew subscriptions before they expire to avoid gaps
6. **Handle duplicate subscriptions**: The API prevents duplicate subscriptions automatically
7. **Implement proper error handling**: Handle various error scenarios gracefully
8. **Rotate API keys regularly**: For security, rotate your API keys periodically

## Notes

- YouTube's PubSubHubbub hub URL is: `https://pubsubhubbub.appspot.com/subscribe`
- The topic URL format is: `https://www.youtube.com/xml/feeds/videos.xml?channel_id={channel_id}`
- Subscriptions are verified asynchronously by YouTube
- The callback URL must be publicly accessible for YouTube to verify it
- HMAC signature verification is optional but recommended for security
