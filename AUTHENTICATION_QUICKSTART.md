# API Key Authentication - Quick Start Guide

## Setup (5 minutes)

### 1. Generate an API Key

```bash
# Option 1: Using openssl
API_KEY=$(openssl rand -base64 32)
echo "Your API Key: $API_KEY"

# Option 2: Using Go
go run -exec sh -c 'package main; import ("crypto/rand"; "encoding/base64"; "fmt"); func main() { b := make([]byte, 32); rand.Read(b); fmt.Println("sk_live_" + base64.URLEncoding.EncodeToString(b)[:40]) }'

# Option 3: Using Python
python3 -c "import secrets; print('sk_live_' + secrets.token_urlsafe(32)[:40])"
```

### 2. Configure the Server

```bash
# Set the API key
export API_KEYS="your-generated-api-key-here"

# Set other required variables
export DATABASE_URL="postgres://user:password@localhost:5432/youtube_webhooks?sslmode=disable"

# Start the server
./server
```

### 3. Test It

```bash
# Test without API key (should fail with 401)
curl -X GET "http://localhost:8080/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx"

# Test with API key (should succeed)
curl -X GET "http://localhost:8080/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx" \
  -H "X-API-Key: your-generated-api-key-here"
```

## Usage

### Create Subscription

```bash
curl -X POST http://localhost:8080/api/v1/subscriptions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "channel_id": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "callback_url": "https://yourdomain.com/webhook"
  }'
```

### List Subscriptions

```bash
curl -X GET "http://localhost:8080/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx" \
  -H "X-API-Key: your-api-key"
```

## Common Issues

### Issue: 401 Unauthorized

**Problem:** All requests return `{"error":"Unauthorized"}`

**Solutions:**
1. Check API key is set: `echo $API_KEYS`
2. Verify you're sending the header: `-H "X-API-Key: your-key"`
3. Check for typos in the key (case-sensitive)
4. Remove any whitespace from the key

### Issue: Server logs "no API keys configured"

**Problem:** Server warns about missing API keys

**Solution:**
```bash
export API_KEYS="your-api-key-here"
# Then restart the server
```

## Multiple API Keys

```bash
# For key rotation or multiple clients
export API_KEYS="key1,key2,key3"
```

## Environment Variables Reference

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `API_KEYS` | Yes* | Comma-separated API keys | `key1,key2,key3` |
| `DATABASE_URL` | Yes | PostgreSQL connection string | `postgres://...` |
| `PORT` | No | Server port (default: 8080) | `8080` |
| `WEBHOOK_PATH` | No | Webhook path (default: /webhook) | `/webhook` |
| `WEBHOOK_SECRET` | No | HMAC secret for webhooks | `secret123` |

*Required for subscription endpoints to work

## Production Checklist

- [ ] Generate strong API keys (32+ characters)
- [ ] Use HTTPS for all requests
- [ ] Store API keys in environment variables or secret manager
- [ ] Set up monitoring for 401 errors
- [ ] Document which keys are used where
- [ ] Plan key rotation schedule
- [ ] Test all clients before deploying

## Documentation

- Full Authentication Guide: `docs/AUTHENTICATION.md`
- API Reference: `docs/SUBSCRIPTION_API.md`
- Implementation Details: `docs/API_KEY_IMPLEMENTATION.md`

## Quick Commands

```bash
# Generate a key
openssl rand -base64 32

# Set environment and start server
export API_KEYS="$(openssl rand -base64 32)"
export DATABASE_URL="postgres://localhost/youtube_webhooks"
./server

# Test endpoint
curl -H "X-API-Key: $API_KEYS" http://localhost:8080/api/v1/subscriptions?channel_id=test
```
