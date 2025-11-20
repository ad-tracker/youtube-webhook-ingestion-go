package queue

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
)

// ParseRedisURL parses a Redis URL and returns asynq.RedisClientOpt
// Supports formats:
//   - redis://[:password@]host:port[/db]
//   - rediss://[:password@]host:port[/db] (TLS)
//   - host:port (legacy format, no password)
func ParseRedisURL(redisURL string) (asynq.RedisClientOpt, error) {
	// Default options
	opt := asynq.RedisClientOpt{
		DB: 0,
	}

	// Handle legacy format (simple host:port)
	if !strings.Contains(redisURL, "://") {
		opt.Addr = redisURL
		return opt, nil
	}

	// Parse as URL
	u, err := url.Parse(redisURL)
	if err != nil {
		return opt, fmt.Errorf("invalid redis URL: %w", err)
	}

	// Validate scheme
	switch u.Scheme {
	case "redis":
		// Standard Redis connection
	case "rediss":
		// Redis with TLS
		opt.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	default:
		return opt, fmt.Errorf("unsupported redis URL scheme: %s (expected 'redis' or 'rediss')", u.Scheme)
	}

	// Extract host and port
	if u.Host == "" {
		return opt, fmt.Errorf("redis URL missing host")
	}
	opt.Addr = u.Host

	// Extract password
	if u.User != nil {
		if password, hasPassword := u.User.Password(); hasPassword {
			opt.Password = password
		}
	}

	// Extract database number
	if u.Path != "" && u.Path != "/" {
		dbStr := strings.TrimPrefix(u.Path, "/")
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return opt, fmt.Errorf("invalid database number in redis URL: %s", dbStr)
		}
		opt.DB = db
	}

	return opt, nil
}
