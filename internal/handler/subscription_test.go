package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repository
type mockSubscriptionRepository struct {
	mock.Mock
}

func (m *mockSubscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	args := m.Called(ctx, sub)
	if args.Error(0) == nil && sub.ID == 0 {
		sub.ID = 1 // Set a default ID for successful creation
	}
	return args.Error(0)
}

func (m *mockSubscriptionRepository) GetByID(ctx context.Context, id int64) (*models.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepository) GetByChannelID(ctx context.Context, channelID string) ([]*models.Subscription, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepository) Update(ctx context.Context, sub *models.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *mockSubscriptionRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSubscriptionRepository) GetExpiringSoon(ctx context.Context, limit int) ([]*models.Subscription, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepository) GetByStatus(ctx context.Context, status string, limit int) ([]*models.Subscription, error) {
	args := m.Called(ctx, status, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

func (m *mockSubscriptionRepository) List(ctx context.Context, filters *repository.SubscriptionFilters) ([]*models.Subscription, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.Subscription), args.Int(1), args.Error(2)
}

// Mock PubSubHub service
type mockPubSubHubService struct {
	mock.Mock
}

func (m *mockPubSubHubService) Subscribe(ctx context.Context, req *service.SubscribeRequest) (*service.SubscribeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.SubscribeResponse), args.Error(1)
}

func (m *mockPubSubHubService) Unsubscribe(ctx context.Context, req *service.SubscribeRequest) (*service.SubscribeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.SubscribeResponse), args.Error(1)
}

func TestSubscriptionHandler_HandleCreate_Success(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.CallbackURL == reqBody.CallbackURL &&
			req.LeaseSeconds == reqBody.LeaseSeconds
	})).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.ChannelID == reqBody.ChannelID &&
			sub.CallbackURL == reqBody.CallbackURL &&
			sub.Status == models.StatusActive
	})).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response models.Subscription
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, reqBody.ChannelID, response.ChannelID)
	assert.Equal(t, reqBody.CallbackURL, response.CallbackURL)
	assert.Equal(t, models.StatusActive, response.Status)

	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_WithSecret(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_DefaultLeaseSeconds(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL: "https://example.com/webhook",
		// LeaseSeconds not specified - should default to 432000
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.LeaseSeconds == 432000
	})).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_InvalidJSON(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response.Error)
}

func TestSubscriptionHandler_HandleCreate_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reqBody CreateSubscriptionRequest
		errMsg  string
	}{
		{
			name: "missing channel_id",
			reqBody: CreateSubscriptionRequest{
				CallbackURL: "https://example.com/webhook",
			},
			errMsg: "channel_id is required",
		},
		{
			name: "invalid channel_id format",
			reqBody: CreateSubscriptionRequest{
				ChannelID:   "invalid",
				CallbackURL: "https://example.com/webhook",
			},
			errMsg: "invalid channel_id format",
		},
		{
			name: "missing callback_url",
			reqBody: CreateSubscriptionRequest{
				ChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
			},
			errMsg: "callback_url is required",
		},
		{
			name: "invalid callback_url format",
			reqBody: CreateSubscriptionRequest{
				ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
				CallbackURL: "not-a-url",
			},
			errMsg: "callback_url must be a valid HTTP or HTTPS URL",
		},
		{
			name: "negative lease_seconds",
			reqBody: CreateSubscriptionRequest{
				ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
				CallbackURL:  "https://example.com/webhook",
				LeaseSeconds: -100,
			},
			errMsg: "lease_seconds must be non-negative",
		},
		{
			name: "lease_seconds too large",
			reqBody: CreateSubscriptionRequest{
				ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
				CallbackURL:  "https://example.com/webhook",
				LeaseSeconds: 1000000,
			},
			errMsg: "lease_seconds cannot exceed 864000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := new(mockSubscriptionRepository)
			hubService := new(mockPubSubHubService)
			handler := NewSubscriptionHandler(repo, &service.PubSubHubService{}, "", nil)
			handler.hubService = hubService

			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)

			var response ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)
			assert.Contains(t, response.Message, tt.errMsg)
		})
	}
}

func TestSubscriptionHandler_HandleCreate_HubSubscriptionFailed(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL: "https://example.com/webhook",
	}
	body, _ := json.Marshal(reqBody)

	hubService.On("Subscribe", mock.Anything, mock.Anything).
		Return(nil, service.ErrSubscriptionFailed)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "failed to subscribe to hub", response.Error)

	hubService.AssertExpectations(t)
	repo.AssertNotCalled(t, "Create")
}

func TestSubscriptionHandler_HandleCreate_DatabaseError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL: "https://example.com/webhook",
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResp, nil)

	dbErr := errors.New("database connection failed")
	repo.On("Create", mock.Anything, mock.Anything).Return(dbErr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "failed to save subscription", response.Error)

	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_DuplicateSubscription(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL: "https://example.com/webhook",
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResp, nil)

	// Simulate duplicate key error using a proper pgconn error
	duplicateErr := fmt.Errorf("create subscription: %w (constraint: uq_channel_callback)", db.ErrDuplicateKey)
	repo.On("Create", mock.Anything, mock.Anything).Return(duplicateErr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "subscription already exists", response.Error)

	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestGetSubscriptionHandler_Success(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	handler := NewGetSubscriptionHandler(repo, nil)

	subscriptions := []*models.Subscription{
		{
			ID:          1,
			ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
			CallbackURL: "https://example.com/webhook1",
			Status:      models.StatusActive,
		},
		{
			ID:          2,
			ChannelID:   "UCxxxxxxxxxxxxxxxxxxxxxx",
			CallbackURL: "https://example.com/webhook2",
			Status:      models.StatusPending,
		},
	}

	repo.On("GetByChannelID", mock.Anything, "UCxxxxxxxxxxxxxxxxxxxxxx").
		Return(subscriptions, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, float64(2), response["count"])

	repo.AssertExpectations(t)
}

func TestGetSubscriptionHandler_MissingChannelID(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	handler := NewGetSubscriptionHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "channel_id")
}

func TestGetSubscriptionHandler_RepositoryError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	handler := NewGetSubscriptionHandler(repo, nil)

	repo.On("GetByChannelID", mock.Anything, "UCxxxxxxxxxxxxxxxxxxxxxx").
		Return(nil, errors.New("database error"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions?channel_id=UCxxxxxxxxxxxxxxxxxxxxxx", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	repo.AssertExpectations(t)
}

func TestGetSubscriptionHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	handler := NewGetSubscriptionHandler(repo, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestSubscriptionHandler_HandleCreate_AutomaticWebhookSecret(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	webhookSecret := "configured-webhook-secret"
	handler := NewSubscriptionHandler(repo, hubService, webhookSecret, nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
		// No Secret provided - should use configured webhookSecret
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.Secret != nil && *req.Secret == webhookSecret
	})).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_ExplicitSecretOverridesConfigured(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	webhookSecret := "configured-webhook-secret"
	handler := NewSubscriptionHandler(repo, hubService, webhookSecret, nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.Secret != nil && *req.Secret == webhookSecret
	})).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_EmptySecretUsesConfigured(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	webhookSecret := "configured-webhook-secret"
	handler := NewSubscriptionHandler(repo, hubService, webhookSecret, nil)

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.Secret != nil && *req.Secret == webhookSecret
	})).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestSubscriptionHandler_HandleCreate_NoSecretWhenConfiguredIsEmpty(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHubService)
	handler := NewSubscriptionHandler(repo, hubService, "", nil) // No configured secret

	reqBody := CreateSubscriptionRequest{
		ChannelID:    "UCxxxxxxxxxxxxxxxxxxxxxx",
		CallbackURL:  "https://example.com/webhook",
		LeaseSeconds: 432000,
		// No Secret provided and no configured secret
	}
	body, _ := json.Marshal(reqBody)

	hubResp := &service.SubscribeResponse{
		Accepted:   true,
		StatusCode: http.StatusAccepted,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResp, nil)

	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	hubService.AssertExpectations(t)
	repo.AssertExpectations(t)
}
