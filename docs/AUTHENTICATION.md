# API Authentication

## Overview

The YouTube Webhook Ingestion service uses API key authentication to protect subscription management endpoints. This ensures that only authorized clients can create and manage YouTube channel subscriptions.

## Protected Endpoints

The following endpoints require authentication:

- `POST /api/v1/subscriptions` - Create a new subscription
- `GET /api/v1/subscriptions` - List subscriptions

## Unprotected Endpoints

These endpoints do NOT require authentication:

- `GET /webhook` - YouTube subscription verification endpoint
- `POST /webhook` - YouTube notification receiver (protected by HMAC signature)
- `GET /health` - Health check endpoint

## Authentication Methods

The API supports two authentication methods using API keys:

### 1. X-API-Key Header (Recommended)

Send the API key in a custom header:

```http
GET /api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx HTTP/1.1
Host: localhost:8080
X-API-Key: your-api-key-here
```

### 2. Authorization Bearer Header

Send the API key as a Bearer token:

```http
POST /api/v1/subscriptions HTTP/1.1
Host: localhost:8080
Authorization: Bearer your-api-key-here
Content-Type: application/json
```

## Configuration

### Setting API Keys

API keys are configured via the `API_KEYS` environment variable. Multiple keys can be provided as a comma-separated list:

```bash
# Single API key
export API_KEYS="sk_live_abc123def456"

# Multiple API keys (for key rotation or multiple clients)
export API_KEYS="sk_live_abc123,sk_test_def456,sk_prod_ghi789"

# Start the server
./server
```

### Key Format

API keys can be any string, but we recommend:

- Minimum length: 32 characters
- Use cryptographically secure random strings
- Use a prefix to identify the environment (e.g., `sk_live_`, `sk_test_`)

Example key generation in Go:

```go
package main

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
)

func generateAPIKey() string {
    b := make([]byte, 32)
    rand.Read(b)
    return "sk_live_" + base64.URLEncoding.EncodeToString(b)[:40]
}

func main() {
    key := generateAPIKey()
    fmt.Println("Generated API Key:", key)
}
```

## Error Responses

### Missing or Invalid API Key

If the API key is missing or invalid, the server returns:

**Status:** `401 Unauthorized`

```json
{
  "error": "Unauthorized"
}
```

This response is returned for:
- No API key provided
- Empty API key
- API key that doesn't match any configured keys
- Malformed Authorization header (e.g., missing "Bearer " prefix)

## Security Features

### Constant-Time Comparison

The authentication middleware uses constant-time comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing attacks when validating API keys.

### Multiple API Keys Support

The service supports multiple API keys simultaneously, which enables:

1. **Key Rotation**: Add a new key, update clients, then remove the old key
2. **Multiple Clients**: Different API keys for different services or environments
3. **Emergency Revocation**: Quickly revoke a compromised key while keeping others active

### Logging

Failed authentication attempts are logged with the following information:
- Request path
- HTTP method
- Remote IP address
- Timestamp

Example log entry:

```json
{
  "time": "2025-11-15T14:03:53Z",
  "level": "WARN",
  "msg": "unauthorized request - invalid or missing API key",
  "path": "/api/v1/subscriptions",
  "method": "POST",
  "remote_addr": "192.168.1.100:54321"
}
```

## Best Practices

### 1. Keep API Keys Secret

- Never commit API keys to version control
- Use environment variables or secret management systems
- Don't include API keys in client-side code
- Rotate keys if they are exposed

### 2. Use HTTPS in Production

Always use HTTPS when transmitting API keys to prevent interception:

```bash
# Good - HTTPS
curl -H "X-API-Key: sk_live_abc123" https://api.yourdomain.com/api/v1/subscriptions

# Bad - HTTP (API key visible in clear text)
curl -H "X-API-Key: sk_live_abc123" http://api.yourdomain.com/api/v1/subscriptions
```

### 3. Implement Key Rotation

Regularly rotate API keys following this process:

1. Generate a new API key
2. Add it to the `API_KEYS` environment variable (keep old key)
3. Restart the server
4. Update all clients to use the new key
5. Remove the old key from `API_KEYS`
6. Restart the server again

### 4. Use Different Keys per Environment

Use different API keys for different environments:

```bash
# Development
export API_KEYS="sk_dev_abc123"

# Staging
export API_KEYS="sk_staging_def456"

# Production
export API_KEYS="sk_prod_ghi789"
```

### 5. Monitor Failed Authentication Attempts

Set up monitoring and alerting for failed authentication attempts, which may indicate:
- Misconfigured clients
- Unauthorized access attempts
- Credential leakage

## Example Usage

### cURL with X-API-Key

```bash
curl -X POST http://localhost:8080/api/v1/subscriptions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: sk_live_abc123def456" \
  -d '{
    "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "callback_url": "https://yourdomain.com/webhook"
  }'
```

### cURL with Authorization Bearer

```bash
curl -X GET "http://localhost:8080/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx" \
  -H "Authorization: Bearer sk_live_abc123def456"
```

### Go Client

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
)

func createSubscription(apiKey, channelID, callbackURL string) error {
    reqBody := map[string]interface{}{
        "channel_id":   channelID,
        "callback_url": callbackURL,
    }

    body, _ := json.Marshal(reqBody)

    req, err := http.NewRequest(
        "POST",
        "http://localhost:8080/api/v1/subscriptions",
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

### Python Client

```python
import requests

def create_subscription(api_key, channel_id, callback_url):
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": api_key
    }

    payload = {
        "channel_id": channel_id,
        "callback_url": callback_url
    }

    response = requests.post(
        "http://localhost:8080/api/v1/subscriptions",
        json=payload,
        headers=headers
    )

    response.raise_for_status()
    return response.json()
```

## Testing

### Testing Authentication Middleware

The authentication middleware includes comprehensive unit tests. Run them with:

```bash
go test ./internal/middleware/... -v
```

### Testing Authenticated Endpoints

When writing integration tests, include the API key in your test requests:

```go
func TestCreateSubscription(t *testing.T) {
    req := httptest.NewRequest("POST", "/api/v1/subscriptions", body)
    req.Header.Set("X-API-Key", "test-key")
    req.Header.Set("Content-Type", "application/json")

    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusCreated, rec.Code)
}
```

## Troubleshooting

### Problem: All requests return 401 Unauthorized

**Possible causes:**
1. `API_KEYS` environment variable is not set
2. API key doesn't match any configured keys
3. API key has leading/trailing whitespace
4. Wrong header name or format

**Solution:**
1. Check that `API_KEYS` is set: `echo $API_KEYS`
2. Verify the API key matches exactly (case-sensitive)
3. Remove any whitespace from the key
4. Ensure header is `X-API-Key: key` or `Authorization: Bearer key`

### Problem: Server logs warning "no API keys configured"

**Cause:** The `API_KEYS` environment variable is empty or not set.

**Solution:**
```bash
export API_KEYS="your-api-key-here"
./server
```

### Problem: One client works but another doesn't

**Possible causes:**
1. Different API keys being used
2. Whitespace in one of the keys
3. Header format differences

**Solution:**
1. Enable debug logging to see exactly what's being sent
2. Verify both clients use the same API key
3. Check for whitespace or encoding issues

## Migration Guide

If you're adding authentication to an existing deployment:

1. **Add API keys to environment without enforcing** (optional grace period)
2. **Update all clients** to include API keys in requests
3. **Deploy the authentication changes**
4. **Monitor logs** for unauthorized requests
5. **Fix any client misconfigurations**

## Reference

- Middleware implementation: `internal/middleware/auth.go`
- Middleware tests: `internal/middleware/auth_test.go`
- Server configuration: `cmd/server/main.go`
- API documentation: `docs/SUBSCRIPTION_API.md`
