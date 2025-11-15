package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock HTTP client
type mockHTTPClient struct {
	mock.Mock
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestPubSubHubService_Subscribe_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{
			name:       "202 Accepted",
			statusCode: http.StatusAccepted,
		},
		{
			name:       "204 No Content",
			statusCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := new(mockHTTPClient)
			service := NewPubSubHubService(client, nil)

			secret := "test-secret"
			req := &SubscribeRequest{
				HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
				TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
				CallbackURL:  "https://example.com/webhook",
				LeaseSeconds: 432000,
				Secret:       &secret,
			}

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString("OK")),
			}
			client.On("Do", mock.MatchedBy(func(r *http.Request) bool {
				return r.Method == http.MethodPost &&
					r.URL.String() == req.HubURL &&
					r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
			})).Return(resp, nil)

			result, err := service.Subscribe(context.Background(), req)

			require.NoError(t, err)
			assert.True(t, result.Accepted)
			assert.Equal(t, tt.statusCode, result.StatusCode)
			assert.Equal(t, "OK", result.ResponseBody)
			assert.Equal(t, 432000, result.LeaseSeconds)
			client.AssertExpectations(t)
		})
	}
}

func TestPubSubHubService_Subscribe_BadRequest(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString("Invalid callback URL")),
	}
	client.On("Do", mock.Anything).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSubscriptionFailed)
	assert.Contains(t, err.Error(), "bad request")
	assert.False(t, result.Accepted)
	assert.Equal(t, http.StatusBadRequest, result.StatusCode)
	client.AssertExpectations(t)
}

func TestPubSubHubService_Subscribe_NotFound(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}

	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString("Topic not found")),
	}
	client.On("Do", mock.Anything).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSubscriptionFailed)
	assert.Contains(t, err.Error(), "not found")
	assert.False(t, result.Accepted)
	assert.Equal(t, http.StatusNotFound, result.StatusCode)
	client.AssertExpectations(t)
}

func TestPubSubHubService_Subscribe_UnexpectedStatus(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewBufferString("Internal server error")),
	}
	client.On("Do", mock.Anything).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidHubResponse)
	assert.Contains(t, err.Error(), "unexpected status code")
	assert.False(t, result.Accepted)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	client.AssertExpectations(t)
}

func TestPubSubHubService_Subscribe_NetworkError(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}

	networkErr := errors.New("network timeout")
	client.On("Do", mock.Anything).Return(nil, networkErr)

	result, err := service.Subscribe(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "send request")
	client.AssertExpectations(t)
}

func TestPubSubHubService_Subscribe_WithSecret(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	secret := "my-secret-key"
	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
		Secret:       &secret,
	}

	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}

	client.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		// Verify the request contains the secret
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		return strings.Contains(bodyStr, "hub.secret=my-secret-key")
	})).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Accepted)
	client.AssertExpectations(t)
}

func TestPubSubHubService_Subscribe_WithoutSecret(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
		Secret:       nil,
	}

	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}

	client.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		// Verify the request does NOT contain the secret
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		return !strings.Contains(bodyStr, "hub.secret")
	})).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Accepted)
	client.AssertExpectations(t)
}

func TestPubSubHubService_Unsubscribe_Success(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	req := &SubscribeRequest{
		HubURL:      "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:    "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest",
		CallbackURL: "https://example.com/webhook",
	}

	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}

	client.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		// Verify the mode is "unsubscribe"
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		return strings.Contains(bodyStr, "hub.mode=unsubscribe")
	})).Return(resp, nil)

	result, err := service.Unsubscribe(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, http.StatusAccepted, result.StatusCode)
	client.AssertExpectations(t)
}

func TestPubSubHubService_ValidateRequest(t *testing.T) {
	t.Parallel()

	service := NewPubSubHubService(nil, nil)

	tests := []struct {
		name    string
		req     *SubscribeRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
			errMsg:  "request is nil",
		},
		{
			name: "missing hub URL",
			req: &SubscribeRequest{
				TopicURL:     "https://example.com/topic",
				CallbackURL:  "https://example.com/callback",
				LeaseSeconds: 432000,
			},
			wantErr: true,
			errMsg:  "hub URL is required",
		},
		{
			name: "missing topic URL",
			req: &SubscribeRequest{
				HubURL:       "https://example.com/hub",
				CallbackURL:  "https://example.com/callback",
				LeaseSeconds: 432000,
			},
			wantErr: true,
			errMsg:  "topic URL is required",
		},
		{
			name: "missing callback URL",
			req: &SubscribeRequest{
				HubURL:       "https://example.com/hub",
				TopicURL:     "https://example.com/topic",
				LeaseSeconds: 432000,
			},
			wantErr: true,
			errMsg:  "callback URL is required",
		},
		{
			name: "negative lease seconds",
			req: &SubscribeRequest{
				HubURL:       "https://example.com/hub",
				TopicURL:     "https://example.com/topic",
				CallbackURL:  "https://example.com/callback",
				LeaseSeconds: -1,
			},
			wantErr: true,
			errMsg:  "lease seconds must be non-negative",
		},
		{
			name: "invalid hub URL",
			req: &SubscribeRequest{
				HubURL:       "://invalid",
				TopicURL:     "https://example.com/topic",
				CallbackURL:  "https://example.com/callback",
				LeaseSeconds: 432000,
			},
			wantErr: true,
			errMsg:  "invalid hub URL",
		},
		{
			name: "valid request",
			req: &SubscribeRequest{
				HubURL:       "https://example.com/hub",
				TopicURL:     "https://example.com/topic",
				CallbackURL:  "https://example.com/callback",
				LeaseSeconds: 432000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := service.validateRequest(tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPubSubHubService_Subscribe_FormData(t *testing.T) {
	t.Parallel()

	client := new(mockHTTPClient)
	service := NewPubSubHubService(client, nil)

	secret := "test-secret"
	req := &SubscribeRequest{
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest123",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
		Secret:       &secret,
	}

	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString("OK")),
	}

	client.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		// Read and verify form data
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		return strings.Contains(bodyStr, "hub.mode=subscribe") &&
			strings.Contains(bodyStr, "hub.topic=https%3A%2F%2Fwww.youtube.com%2Fxml%2Ffeeds%2Fvideos.xml%3Fchannel_id%3DUCtest123") &&
			strings.Contains(bodyStr, "hub.callback=https%3A%2F%2Fexample.com%2Fwebhook") &&
			strings.Contains(bodyStr, "hub.lease_seconds=432000") &&
			strings.Contains(bodyStr, "hub.secret=test-secret")
	})).Return(resp, nil)

	result, err := service.Subscribe(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Accepted)
	client.AssertExpectations(t)
}
