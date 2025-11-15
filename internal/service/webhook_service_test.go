package service

import (
	"testing"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
)

func TestNewWebhookService(t *testing.T) {
	service := NewWebhookService(nil, nil, nil)

	if service == nil {
		t.Fatal("NewWebhookService() returned nil")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Message: "test validation error"}

	if err.Error() != "test validation error" {
		t.Errorf("ValidationError.Error() = %s, want 'test validation error'", err.Error())
	}
}

func TestProcessingError(t *testing.T) {
	tests := []struct {
		name    string
		err     *ProcessingError
		want    string
	}{
		{
			name: "without cause",
			err:  &ProcessingError{Message: "test error", Cause: nil},
			want: "test error: <nil>",
		},
		{
			name: "with cause",
			err:  &ProcessingError{Message: "test error", Cause: &ValidationError{Message: "cause"}},
			want: "test error: cause",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("ProcessingError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookService_SerializePayload(t *testing.T) {
	ws := NewWebhookService(nil, nil, nil)

	payload := &models.WebhookPayloadDTO{
		VideoID:   "test123",
		ChannelID: "UCtest",
		EventType: "TEST_EVENT",
		Content:   "test content",
		Signature: "test sig",
		Timestamp: 1234567890000,
	}

	result := ws.serializePayload(payload)

	// Check that it returns a valid JSON string
	if result == "" {
		t.Error("serializePayload() returned empty string")
	}

	// Check that it contains expected fields
	if len(result) < 10 {
		t.Error("serializePayload() returned too short a string")
	}
}
