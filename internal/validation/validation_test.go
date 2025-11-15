package validation

import (
	"strings"
	"testing"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		maxPayloadSize int64
		enabled        bool
	}{
		{
			name:           "enabled validator with 1MB limit",
			maxPayloadSize: 1024 * 1024,
			enabled:        true,
		},
		{
			name:           "disabled validator",
			maxPayloadSize: 1024,
			enabled:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(tt.maxPayloadSize, tt.enabled)
			if v == nil {
				t.Fatal("New() returned nil")
			}
			if v.maxPayloadSize != tt.maxPayloadSize {
				t.Errorf("maxPayloadSize = %d, want %d", v.maxPayloadSize, tt.maxPayloadSize)
			}
			if v.validationEnabled != tt.enabled {
				t.Errorf("validationEnabled = %v, want %v", v.validationEnabled, tt.enabled)
			}
		})
	}
}

func TestValidator_ValidatePayload(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name    string
		enabled bool
		payload *models.WebhookPayloadDTO
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid payload",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Content:   "test content",
				Timestamp: now * 1000, // milliseconds
			},
			wantErr: false,
		},
		{
			name:    "validation disabled - invalid payload passes",
			enabled: false,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "invalid",
				ChannelID: "invalid",
				EventType: "invalid",
				Timestamp: now * 1000,
			},
			wantErr: false,
		},
		{
			name:    "invalid video ID format",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "invalid",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Timestamp: now * 1000,
			},
			wantErr: true,
			errMsg:  "invalid video ID format",
		},
		{
			name:    "empty video ID is valid",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Timestamp: now * 1000,
			},
			wantErr: false,
		},
		{
			name:    "invalid channel ID format",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "invalid",
				EventType: "VIDEO_PUBLISHED",
				Timestamp: now * 1000,
			},
			wantErr: true,
			errMsg:  "invalid channel ID format",
		},
		{
			name:    "invalid event type format",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "invalid-event",
				Timestamp: now * 1000,
			},
			wantErr: true,
			errMsg:  "invalid event type format",
		},
		{
			name:    "timestamp in future",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Timestamp: (now + 120) * 1000, // 2 minutes in future
			},
			wantErr: true,
			errMsg:  "timestamp is in the future",
		},
		{
			name:    "timestamp too old",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Timestamp: (now - 8*24*60*60) * 1000, // 8 days old
			},
			wantErr: true,
			errMsg:  "timestamp is too old",
		},
		{
			name:    "payload content too large",
			enabled: true,
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "VIDEO_PUBLISHED",
				Content:   strings.Repeat("a", 1024*1024+1), // 1MB + 1 byte
				Timestamp: now * 1000,
			},
			wantErr: true,
			errMsg:  "payload content exceeds maximum size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New(1024*1024, tt.enabled)
			err := v.ValidatePayload(tt.payload)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidatePayload() error = nil, wantErr = true")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePayload() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePayload() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidator_IsValidVideoID(t *testing.T) {
	tests := []struct {
		name    string
		videoID string
		want    bool
	}{
		{
			name:    "valid video ID",
			videoID: "dQw4w9WgXcQ",
			want:    true,
		},
		{
			name:    "valid video ID with underscore",
			videoID: "dQw4w9Wg_cQ",
			want:    true,
		},
		{
			name:    "valid video ID with hyphen",
			videoID: "dQw4w9Wg-cQ",
			want:    true,
		},
		{
			name:    "invalid - too short",
			videoID: "short",
			want:    false,
		},
		{
			name:    "invalid - too long",
			videoID: "dQw4w9WgXcQExtra",
			want:    false,
		},
		{
			name:    "invalid - special characters",
			videoID: "dQw4w9Wg@cQ",
			want:    false,
		},
		{
			name:    "invalid - empty",
			videoID: "",
			want:    false,
		},
	}

	v := New(1024, true)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsValidVideoID(tt.videoID); got != tt.want {
				t.Errorf("IsValidVideoID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_IsValidChannelID(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		want      bool
	}{
		{
			name:      "valid channel ID",
			channelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
			want:      true,
		},
		{
			name:      "valid channel ID with underscore",
			channelID: "UC_AXFkgsw1L7xaCfnd5JJOw",
			want:      true,
		},
		{
			name:      "valid channel ID with hyphen",
			channelID: "UC-AXFkgsw1L7xaCfnd5JJOw",
			want:      true,
		},
		{
			name:      "invalid - doesn't start with UC",
			channelID: "ABuAXFkgsw1L7xaCfnd5JJOw",
			want:      false,
		},
		{
			name:      "invalid - too short",
			channelID: "UCshort",
			want:      false,
		},
		{
			name:      "invalid - too long",
			channelID: "UCuAXFkgsw1L7xaCfnd5JJOwExtra",
			want:      false,
		},
		{
			name:      "invalid - special characters",
			channelID: "UCuAXFkgsw1L7xaCfnd5JJ@w",
			want:      false,
		},
		{
			name:      "invalid - empty",
			channelID: "",
			want:      false,
		},
	}

	v := New(1024, true)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsValidChannelID(tt.channelID); got != tt.want {
				t.Errorf("IsValidChannelID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_IsValidEventType(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      bool
	}{
		{
			name:      "valid event type",
			eventType: "VIDEO_PUBLISHED",
			want:      true,
		},
		{
			name:      "valid event type with underscores",
			eventType: "VIDEO_UPDATED",
			want:      true,
		},
		{
			name:      "valid single word",
			eventType: "PUBLISHED",
			want:      true,
		},
		{
			name:      "invalid - lowercase",
			eventType: "video_published",
			want:      false,
		},
		{
			name:      "invalid - mixed case",
			eventType: "Video_Published",
			want:      false,
		},
		{
			name:      "invalid - with hyphen",
			eventType: "VIDEO-PUBLISHED",
			want:      false,
		},
		{
			name:      "invalid - with space",
			eventType: "VIDEO PUBLISHED",
			want:      false,
		},
		{
			name:      "invalid - empty",
			eventType: "",
			want:      false,
		},
	}

	v := New(1024, true)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.IsValidEventType(tt.eventType); got != tt.want {
				t.Errorf("IsValidEventType() = %v, want %v", got, tt.want)
			}
		})
	}
}
