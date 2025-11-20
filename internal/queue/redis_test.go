package queue

import (
	"crypto/tls"
	"testing"

	"github.com/hibiken/asynq"
)

func TestParseRedisURL(t *testing.T) {
	tests := []struct {
		name      string
		redisURL  string
		want      asynq.RedisClientOpt
		wantError bool
	}{
		{
			name:     "simple host:port format (legacy)",
			redisURL: "localhost:6379",
			want: asynq.RedisClientOpt{
				Addr: "localhost:6379",
				DB:   0,
			},
			wantError: false,
		},
		{
			name:     "redis URL without password",
			redisURL: "redis://localhost:6379",
			want: asynq.RedisClientOpt{
				Addr: "localhost:6379",
				DB:   0,
			},
			wantError: false,
		},
		{
			name:     "redis URL with password",
			redisURL: "redis://:mypassword@localhost:6379",
			want: asynq.RedisClientOpt{
				Addr:     "localhost:6379",
				Password: "mypassword",
				DB:       0,
			},
			wantError: false,
		},
		{
			name:     "redis URL with password and database number",
			redisURL: "redis://:secretpass@redis.example.com:6379/1",
			want: asynq.RedisClientOpt{
				Addr:     "redis.example.com:6379",
				Password: "secretpass",
				DB:       1,
			},
			wantError: false,
		},
		{
			name:     "redis URL with URL-encoded password",
			redisURL: "redis://:p%40ssw0rd%21%40%23%24@localhost:6379/0",
			want: asynq.RedisClientOpt{
				Addr:     "localhost:6379",
				Password: "p@ssw0rd!@#$",
				DB:       0,
			},
			wantError: false,
		},
		{
			name:     "rediss URL with TLS",
			redisURL: "rediss://:password@secure-redis.example.com:6380/0",
			want: asynq.RedisClientOpt{
				Addr:      "secure-redis.example.com:6380",
				Password:  "password",
				DB:        0,
				TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
			wantError: false,
		},
		{
			name:     "redis URL with database number 5",
			redisURL: "redis://localhost:6379/5",
			want: asynq.RedisClientOpt{
				Addr: "localhost:6379",
				DB:   5,
			},
			wantError: false,
		},
		{
			name:     "redis URL with valkey host",
			redisURL: "redis://valkey:6379/0",
			want: asynq.RedisClientOpt{
				Addr: "valkey:6379",
				DB:   0,
			},
			wantError: false,
		},
		{
			name:      "invalid scheme",
			redisURL:  "http://localhost:6379",
			want:      asynq.RedisClientOpt{DB: 0},
			wantError: true,
		},
		{
			name:      "invalid database number",
			redisURL:  "redis://localhost:6379/abc",
			want:      asynq.RedisClientOpt{DB: 0},
			wantError: true,
		},
		{
			name:      "redis URL missing host",
			redisURL:  "redis://:password@/0",
			want:      asynq.RedisClientOpt{DB: 0},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRedisURL(tt.redisURL)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseRedisURL() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err != nil {
				// If we expect an error, don't check the result
				return
			}

			// Check basic fields
			if got.Addr != tt.want.Addr {
				t.Errorf("ParseRedisURL() Addr = %v, want %v", got.Addr, tt.want.Addr)
			}
			if got.Password != tt.want.Password {
				t.Errorf("ParseRedisURL() Password = %v, want %v", got.Password, tt.want.Password)
			}
			if got.DB != tt.want.DB {
				t.Errorf("ParseRedisURL() DB = %v, want %v", got.DB, tt.want.DB)
			}

			// Check TLS config
			if (got.TLSConfig != nil) != (tt.want.TLSConfig != nil) {
				t.Errorf("ParseRedisURL() TLSConfig presence = %v, want %v",
					got.TLSConfig != nil, tt.want.TLSConfig != nil)
			}

			if got.TLSConfig != nil && tt.want.TLSConfig != nil {
				if got.TLSConfig.MinVersion != tt.want.TLSConfig.MinVersion {
					t.Errorf("ParseRedisURL() TLSConfig.MinVersion = %v, want %v",
						got.TLSConfig.MinVersion, tt.want.TLSConfig.MinVersion)
				}
			}
		})
	}
}

func TestParseRedisURL_BackwardCompatibility(t *testing.T) {
	// Test that old simple host:port format still works
	legacyFormats := []string{
		"localhost:6379",
		"redis.local:6379",
		"127.0.0.1:6379",
		"valkey:6379",
	}

	for _, format := range legacyFormats {
		t.Run(format, func(t *testing.T) {
			got, err := ParseRedisURL(format)
			if err != nil {
				t.Errorf("ParseRedisURL() legacy format failed: %v", err)
				return
			}
			if got.Addr != format {
				t.Errorf("ParseRedisURL() Addr = %v, want %v", got.Addr, format)
			}
			if got.DB != 0 {
				t.Errorf("ParseRedisURL() DB = %v, want 0", got.DB)
			}
			if got.Password != "" {
				t.Errorf("ParseRedisURL() Password should be empty for legacy format")
			}
		})
	}
}
