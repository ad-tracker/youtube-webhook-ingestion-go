//go:build integration
// +build integration

package service

import (
	"testing"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/repository"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/validation"
)

// Integration test for ProcessWebhook that provides better code coverage
// This runs only when the integration build tag is set
func TestWebhookService_ProcessWebhook_Integration(t *testing.T) {
	// This test can be expanded when integration test infrastructure is available
	t.Skip("Integration test infrastructure not yet implemented")
}

// This helper allows us to test the serialization logic more thoroughly
func TestWebhookService_SerializePayload_CompleteValidation(t *testing.T) {
	ws := NewWebhookService(nil, nil, nil)

	tests := []struct {
		name    string
		payload *models.WebhookPayloadDTO
		wantNonEmpty bool
	}{
		{
			name: "valid complete payload",
			payload: &models.WebhookPayloadDTO{
				VideoID:   "dQw4w9WgXcQ",
				ChannelID: "UCuAXFkgsw1L7xaCfnd5JJOw",
				EventType: "published",
				Content:   "<xml>test content</xml>",
				Signature: "test-signature",
				Timestamp: time.Now().Unix(),
			},
			wantNonEmpty: true,
		},
		{
			name: "minimal payload",
			payload: &models.WebhookPayloadDTO{
				VideoID:   "abc",
				ChannelID: "UCdef",
				EventType: "updated",
			},
			wantNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ws.serializePayload(tt.payload)

			if tt.wantNonEmpty && len(result) == 0 {
				t.Errorf("serializePayload() returned empty string for %s", tt.name)
			}

			// Basic validation that it's JSON-like
			if tt.wantNonEmpty && (result[0] != '{' && result[0] != '[') {
				t.Errorf("serializePayload() doesn't appear to return valid JSON: %s", result)
			}
		})
	}
}

// Test the creation of the service
func TestNewWebhookService_WithNilParams(t *testing.T) {
	ws := NewWebhookService(nil, nil, nil)
	if ws == nil {
		t.Fatal("NewWebhookService() returned nil")
	}

	if ws.repo != nil {
		t.Error("NewWebhookService() with nil repo should have nil repo")
	}
	if ws.publisher != nil {
		t.Error("NewWebhookService() with nil publisher should have nil publisher")
	}
	if ws.validator != nil {
		t.Error("NewWebhookService() with nil validator should have nil validator")
	}
}

// Test with actual validator
func TestNewWebhookService_WithValidator(t *testing.T) {
	validator := validation.New(1*1024*1024, true)
	ws := NewWebhookService(nil, nil, validator)

	if ws == nil {
		t.Fatal("NewWebhookService() returned nil")
	}

	if ws.validator == nil {
		t.Error("NewWebhookService() should preserve validator reference")
	}
}

// Test error types
func TestValidationError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"simple message", "validation failed"},
		{"empty message", ""},
		{"long message", "this is a very long validation error message that contains lots of details about what went wrong"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{Message: tt.message}
			if err.Error() != tt.message {
				t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

func TestProcessingError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name    string
		message string
		cause   error
		want    string
	}{
		{
			name:    "with cause",
			message: "failed to process",
			cause:   &ValidationError{Message: "invalid data"},
			want:    "failed to process: invalid data",
		},
		{
			name:    "without cause",
			message: "processing error",
			cause:   nil,
			want:    "processing error: <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ProcessingError{Message: tt.message, Cause: tt.cause}
			if err.Error() != tt.want {
				t.Errorf("ProcessingError.Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

// Test repository hash computation (indirect coverage)
func TestRepositoryComputeEventHash(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int // expected hash length
	}{
		{"simple content", "<xml>test</xml>", 64}, // SHA-256 produces 64 hex chars
		{"empty content", "", 64},
		{"long content", string(make([]byte, 1000)), 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := repository.ComputeEventHash(tt.content)
			if len(hash) != tt.want {
				t.Errorf("ComputeEventHash() hash length = %d, want %d", len(hash), tt.want)
			}

			// Hash should be deterministic
			hash2 := repository.ComputeEventHash(tt.content)
			if hash != hash2 {
				t.Error("ComputeEventHash() should be deterministic")
			}
		})
	}
}
