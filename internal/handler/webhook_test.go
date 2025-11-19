package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ad-tracker/youtube-webhook-ingestion/internal/queue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock processor
type mockProcessor struct {
	mock.Mock
}

func (m *mockProcessor) ProcessEvent(ctx context.Context, rawXML string) error {
	args := m.Called(ctx, rawXML)
	return args.Error(0)
}

func (m *mockProcessor) SetQueueClient(client *queue.Client) {
	// No-op for tests - queue client is optional
}

func TestWebhookHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	handler := NewWebhookHandler(processor, "", nil)

	req := httptest.NewRequest(http.MethodPut, "/webhook", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestWebhookHandler_HandleVerification_Success(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	handler := NewWebhookHandler(processor, "", nil)

	challenge := "test-challenge-12345"
	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.challenge="+challenge, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, challenge, rec.Body.String())
}

func TestWebhookHandler_HandleVerification_MissingChallenge(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	handler := NewWebhookHandler(processor, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Missing hub.challenge parameter")
}

func TestWebhookHandler_HandleNotification_Success(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	handler := NewWebhookHandler(processor, "", nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`

	processor.On("ProcessEvent", mock.Anything, atomXML).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	processor.AssertExpectations(t)
}

func TestWebhookHandler_HandleNotification_ProcessingError(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	handler := NewWebhookHandler(processor, "", nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
  </entry>
</feed>`

	expectedErr := errors.New("processing failed")
	processor.On("ProcessEvent", mock.Anything, atomXML).Return(expectedErr)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Failed to process event")
	processor.AssertExpectations(t)
}

func TestWebhookHandler_HandleNotification_WithValidSignature(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	secret := "test-secret"
	handler := NewWebhookHandler(processor, secret, nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
  </entry>
</feed>`

	// Compute valid signature
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(atomXML))
	signature := "sha1=" + hex.EncodeToString(mac.Sum(nil))

	processor.On("ProcessEvent", mock.Anything, atomXML).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	req.Header.Set("X-Hub-Signature", signature)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	processor.AssertExpectations(t)
}

func TestWebhookHandler_HandleNotification_WithInvalidSignature(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	secret := "test-secret"
	handler := NewWebhookHandler(processor, secret, nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
  </entry>
</feed>`

	// Use an invalid signature
	invalidSignature := "sha1=invalid1234567890abcdef"

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	req.Header.Set("X-Hub-Signature", invalidSignature)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Signature verification failed")
	// Processor should not be called
	processor.AssertNotCalled(t, "ProcessEvent")
}

func TestWebhookHandler_HandleNotification_MissingSignature(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	secret := "test-secret"
	handler := NewWebhookHandler(processor, secret, nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
  </entry>
</feed>`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	// No X-Hub-Signature header set
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// Processor should not be called
	processor.AssertNotCalled(t, "ProcessEvent")
}

func TestWebhookHandler_HandleNotification_WrongSignatureFormat(t *testing.T) {
	t.Parallel()

	processor := new(mockProcessor)
	secret := "test-secret"
	handler := NewWebhookHandler(processor, secret, nil)

	atomXML := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
  </entry>
</feed>`

	// Wrong format - should start with "sha1="
	wrongFormatSignature := "md5=1234567890abcdef"

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(atomXML))
	req.Header.Set("X-Hub-Signature", wrongFormatSignature)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// Processor should not be called
	processor.AssertNotCalled(t, "ProcessEvent")
}

func TestWebhookHandler_VerifySignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secret    string
		body      string
		signature string
		wantErr   bool
	}{
		{
			name:   "valid signature",
			secret: "my-secret",
			body:   "test body content",
			signature: func() string {
				mac := hmac.New(sha1.New, []byte("my-secret"))
				mac.Write([]byte("test body content"))
				return "sha1=" + hex.EncodeToString(mac.Sum(nil))
			}(),
			wantErr: false,
		},
		{
			name:      "invalid signature",
			secret:    "my-secret",
			body:      "test body content",
			signature: "sha1=invalid",
			wantErr:   true,
		},
		{
			name:      "missing sha1 prefix",
			secret:    "my-secret",
			body:      "test body content",
			signature: "1234567890abcdef",
			wantErr:   true,
		},
		{
			name:      "empty signature",
			secret:    "my-secret",
			body:      "test body content",
			signature: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &WebhookHandler{secret: tt.secret}

			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(tt.body))
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature", tt.signature)
			}

			err := handler.verifySignature(req, []byte(tt.body))

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
