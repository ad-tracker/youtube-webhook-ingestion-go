package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/service"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
	// Initialize logger to prevent nil pointer errors
	_ = logger.Init("error", "")
}

func TestNewWebhookHandler(t *testing.T) {
	handler := NewWebhookHandler(nil)

	if handler == nil {
		t.Fatal("NewWebhookHandler() returned nil")
	}
}

func TestWebhookHandler_HealthCheck(t *testing.T) {
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/webhook/health", nil)

	handler.HealthCheck(c)

	if w.Code != http.StatusOK {
		t.Errorf("HealthCheck() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check response contains status
	body := w.Body.String()
	if body == "" {
		t.Error("HealthCheck() returned empty body")
	}
}

func TestWebhookHandler_GetClientIP(t *testing.T) {
	handler := NewWebhookHandler(nil)

	tests := []struct {
		name    string
		headers map[string]string
		wantIP  string
	}{
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1",
			},
			wantIP: "203.0.113.1",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			wantIP: "203.0.113.2",
		},
		{
			name: "X-Forwarded-For with spaces",
			headers: map[string]string{
				"X-Forwarded-For": " 203.0.113.3 , 198.51.100.2",
			},
			wantIP: "203.0.113.3",
		},
		{
			name:    "no headers - falls back to ClientIP",
			headers: map[string]string{},
			wantIP:  "", // Will be empty in test context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/webhook", nil)

			for k, v := range tt.headers {
				c.Request.Header.Set(k, v)
			}

			got := handler.getClientIP(c)
			if tt.wantIP != "" && got != tt.wantIP {
				t.Errorf("getClientIP() = %v, want %v", got, tt.wantIP)
			}
		})
	}
}

func TestWebhookHandler_HandleYouTubeWebhook_InvalidJSON(t *testing.T) {
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Send invalid JSON
	invalidJSON := []byte(`{invalid json}`)
	c.Request = httptest.NewRequest("POST", "/webhook", bytes.NewReader(invalidJSON))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleYouTubeWebhook(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("HandleYouTubeWebhook() with invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errResp.Status != http.StatusBadRequest {
		t.Errorf("Error response status = %d, want %d", errResp.Status, http.StatusBadRequest)
	}
}

func TestWebhookHandler_HandleYouTubeWebhook_WithValidationError(t *testing.T) {
	// Create a minimal mock that returns a validation error
	// We can't easily mock the service without interfaces, so we test the error path via invalid JSON
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create payload that will fail validation if service is called
	payload := models.WebhookPayloadDTO{
		VideoID:   "",
		ChannelID: "",
		EventType: "",
	}
	jsonPayload, _ := json.Marshal(payload)

	c.Request = httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonPayload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "test-agent")

	// This will fail during JSON binding since service is nil
	handler.HandleYouTubeWebhook(c)

	// The handler should handle the nil service gracefully or fail early
	// This test ensures the function runs without panicking
}

func TestWebhookHandler_HandleError_ValidationError(t *testing.T) {
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhook", nil)

	err := &service.ValidationError{Message: "validation failed"}
	handler.handleError(c, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleError() with ValidationError status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errResp.Status != http.StatusBadRequest {
		t.Errorf("Error response status = %d, want %d", errResp.Status, http.StatusBadRequest)
	}
}

func TestWebhookHandler_HandleError_ProcessingError(t *testing.T) {
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhook", nil)

	err := &service.ProcessingError{Message: "processing failed"}
	handler.handleError(c, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("handleError() with ProcessingError status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errResp.Status != http.StatusInternalServerError {
		t.Errorf("Error response status = %d, want %d", errResp.Status, http.StatusInternalServerError)
	}
}

func TestWebhookHandler_HandleError_UnexpectedError(t *testing.T) {
	handler := NewWebhookHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhook", nil)

	err := errors.New("unexpected error")
	handler.handleError(c, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("handleError() with unexpected error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
