package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockSubscriptionRepository mocks the SubscriptionRepository interface
type mockSubscriptionRepository struct {
	mock.Mock
}

func (m *mockSubscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	args := m.Called(ctx, sub)
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
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Subscription), args.Int(1), args.Error(2)
}

// mockPubSubHub mocks the PubSubHub interface
type mockPubSubHub struct {
	mock.Mock
}

func (m *mockPubSubHub) Subscribe(ctx context.Context, req *service.SubscribeRequest) (*service.SubscribeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.SubscribeResponse), args.Error(1)
}

func (m *mockPubSubHub) Unsubscribe(ctx context.Context, req *service.SubscribeRequest) (*service.SubscribeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.SubscribeResponse), args.Error(1)
}

// Helper function to create a test subscription
func createTestSubscription(id int64, channelID string, expiresIn time.Duration) *models.Subscription {
	return &models.Subscription{
		ID:           id,
		ChannelID:    channelID,
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + channelID,
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		LeaseSeconds: 432000,
		ExpiresAt:    time.Now().Add(expiresIn),
		Status:       models.StatusActive,
		CreatedAt:    time.Now().Add(-48 * time.Hour),
		UpdatedAt:    time.Now().Add(-48 * time.Hour),
	}
}

// Helper function to create a test logger that discards output
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func TestRenewalService_RenewExpiring_Success(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	// Create test subscriptions
	subscriptions := []*models.Subscription{
		createTestSubscription(1, "UCtest1", 12*time.Hour),
		createTestSubscription(2, "UCtest2", 6*time.Hour),
		createTestSubscription(3, "UCtest3", 1*time.Hour),
	}

	// Mock repository to return expiring subscriptions
	repo.On("GetExpiringSoon", mock.Anything, 100).Return(subscriptions, nil)

	// Mock hub service to accept all renewals
	hubResponse := &service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResponse, nil).Times(3)

	// Mock repository to update each subscription
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.Status == models.StatusActive
	})).Return(nil).Times(3)

	// Execute renewal
	err := renewalService.RenewExpiring(context.Background())

	require.NoError(t, err)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_RenewExpiring_NoSubscriptions(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	// Mock repository to return empty list
	repo.On("GetExpiringSoon", mock.Anything, 100).Return([]*models.Subscription{}, nil)

	// Execute renewal
	err := renewalService.RenewExpiring(context.Background())

	require.NoError(t, err)
	repo.AssertExpectations(t)
	hubService.AssertNotCalled(t, "Subscribe")
	repo.AssertNotCalled(t, "Update")
}

func TestRenewalService_RenewExpiring_GetExpiringSoonError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	// Mock repository to return an error
	dbErr := errors.New("database connection failed")
	repo.On("GetExpiringSoon", mock.Anything, 100).Return(nil, dbErr)

	// Execute renewal
	err := renewalService.RenewExpiring(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get expiring subscriptions")
	assert.ErrorIs(t, err, dbErr)
	repo.AssertExpectations(t)
}

func TestRenewalService_RenewExpiring_PartialSuccess(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	// Create test subscriptions
	subscriptions := []*models.Subscription{
		createTestSubscription(1, "UCtest1", 12*time.Hour),
		createTestSubscription(2, "UCtest2", 6*time.Hour),
		createTestSubscription(3, "UCtest3", 1*time.Hour),
	}

	repo.On("GetExpiringSoon", mock.Anything, 100).Return(subscriptions, nil)

	// First subscription succeeds
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.TopicURL == subscriptions[0].TopicURL
	})).Return(&service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}, nil).Once()

	// Second subscription fails
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.TopicURL == subscriptions[1].TopicURL
	})).Return(nil, errors.New("hub connection failed")).Once()

	// Third subscription succeeds
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.TopicURL == subscriptions[2].TopicURL
	})).Return(&service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}, nil).Once()

	// Update should be called twice for successful renewals and once for failed
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.Status == models.StatusActive && (sub.ID == 1 || sub.ID == 3)
	})).Return(nil).Twice()

	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.Status == models.StatusFailed && sub.ID == 2
	})).Return(nil).Once()

	// Execute renewal - should not return error even if some renewals fail
	err := renewalService.RenewExpiring(context.Background())

	require.NoError(t, err)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_renewSubscription_Success(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := createTestSubscription(1, "UCtest1", 12*time.Hour)

	// Mock hub service to accept renewal
	hubResponse := &service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}
	hubService.On("Subscribe", mock.Anything, mock.MatchedBy(func(req *service.SubscribeRequest) bool {
		return req.HubURL == subscription.HubURL &&
			req.TopicURL == subscription.TopicURL &&
			req.LeaseSeconds == subscription.LeaseSeconds
	})).Return(hubResponse, nil)

	// Mock repository to update subscription
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.ID == subscription.ID && sub.Status == models.StatusActive
	})).Return(nil)

	// Execute renewal
	err := renewalService.renewSubscription(context.Background(), subscription)

	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, subscription.Status)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_renewSubscription_HubRejected(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := createTestSubscription(1, "UCtest1", 12*time.Hour)

	// Mock hub service to reject renewal
	hubResponse := &service.SubscribeResponse{
		Accepted:   false,
		StatusCode: 400,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResponse, nil)

	// Mock repository to update subscription with failed status
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.ID == subscription.ID && sub.Status == models.StatusFailed
	})).Return(nil)

	// Execute renewal
	err := renewalService.renewSubscription(context.Background(), subscription)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, subscription.Status)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_renewSubscription_HubError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := createTestSubscription(1, "UCtest1", 12*time.Hour)

	// Mock hub service to return error
	hubErr := errors.New("network timeout")
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(nil, hubErr)

	// Mock repository to update subscription with failed status
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.ID == subscription.ID && sub.Status == models.StatusFailed
	})).Return(nil)

	// Execute renewal
	err := renewalService.renewSubscription(context.Background(), subscription)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hub subscription failed")
	assert.ErrorIs(t, err, hubErr)
	assert.Equal(t, models.StatusFailed, subscription.Status)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_renewSubscription_UpdateError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := createTestSubscription(1, "UCtest1", 12*time.Hour)

	// Mock hub service to accept renewal
	hubResponse := &service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(hubResponse, nil)

	// Mock repository to return error on update
	dbErr := errors.New("database update failed")
	repo.On("Update", mock.Anything, mock.Anything).Return(dbErr)

	// Execute renewal
	err := renewalService.renewSubscription(context.Background(), subscription)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update subscription")
	assert.ErrorIs(t, err, dbErr)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_renewSubscription_MarkFailedUpdateError(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := createTestSubscription(1, "UCtest1", 12*time.Hour)

	// Mock hub service to return error
	hubErr := errors.New("network timeout")
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(nil, hubErr)

	// Mock repository to fail when marking as failed
	updateErr := errors.New("database update failed")
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *models.Subscription) bool {
		return sub.Status == models.StatusFailed
	})).Return(updateErr)

	// Execute renewal - should still return hub error, but also log update error
	err := renewalService.renewSubscription(context.Background(), subscription)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hub subscription failed")
	assert.ErrorIs(t, err, hubErr)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
		wantExit bool
	}{
		{
			name: "all values set",
			envVars: map[string]string{
				"DATABASE_URL":     "postgres://localhost/testdb",
				"WEBHOOK_SECRET":   "test-secret-123",
				"WEBHOOK_URL":      "https://example.com/webhook",
				"RENEWAL_INTERVAL": "4h",
				"BATCH_SIZE":       "50",
			},
			expected: &Config{
				DatabaseURL:     "postgres://localhost/testdb",
				WebhookSecret:   "test-secret-123",
				WebhookURL:      "https://example.com/webhook",
				RenewalInterval: 4 * time.Hour,
				BatchSize:       50,
			},
			wantExit: false,
		},
		{
			name: "default values",
			envVars: map[string]string{
				"DATABASE_URL":   "postgres://localhost/testdb",
				"WEBHOOK_SECRET": "test-secret-123",
				"WEBHOOK_URL":    "https://example.com/webhook",
			},
			expected: &Config{
				DatabaseURL:     "postgres://localhost/testdb",
				WebhookSecret:   "test-secret-123",
				WebhookURL:      "https://example.com/webhook",
				RenewalInterval: 6 * time.Hour,
				BatchSize:       100,
			},
			wantExit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config := loadConfig()

			assert.Equal(t, tt.expected.DatabaseURL, config.DatabaseURL)
			assert.Equal(t, tt.expected.WebhookSecret, config.WebhookSecret)
			assert.Equal(t, tt.expected.WebhookURL, config.WebhookURL)
			assert.Equal(t, tt.expected.RenewalInterval, config.RenewalInterval)
			assert.Equal(t, tt.expected.BatchSize, config.BatchSize)
		})
	}
}

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "valid duration - hours",
			input:    "4h",
			expected: 4 * time.Hour,
		},
		{
			name:     "valid duration - minutes",
			input:    "30m",
			expected: 30 * time.Minute,
		},
		{
			name:     "valid duration - seconds",
			input:    "120s",
			expected: 120 * time.Second,
		},
		{
			name:     "valid duration - combined",
			input:    "2h30m",
			expected: 2*time.Hour + 30*time.Minute,
		},
		{
			name:     "invalid duration - fallback to default",
			input:    "invalid",
			expected: defaultRenewalInterval,
		},
		{
			name:     "empty string - fallback to default",
			input:    "",
			expected: defaultRenewalInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := parseDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid integer",
			input:    "50",
			expected: 50,
		},
		{
			name:     "valid integer - zero",
			input:    "0",
			expected: 0,
		},
		{
			name:     "valid integer - large",
			input:    "10000",
			expected: 10000,
		},
		{
			name:     "invalid integer - text",
			input:    "invalid",
			expected: defaultBatchSize,
		},
		{
			name:     "invalid integer - empty",
			input:    "",
			expected: defaultBatchSize,
		},
		{
			name:     "invalid integer - float",
			input:    "50.5",
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := parseInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "environment variable set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "environment variable not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenewalService_RenewExpiring_WithSecret(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := &models.Subscription{
		ID:           1,
		ChannelID:    "UCtest1",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest1",
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		LeaseSeconds: 432000,
		ExpiresAt:    time.Now().Add(12 * time.Hour),
		Status:       models.StatusActive,
	}

	repo.On("GetExpiringSoon", mock.Anything, 100).Return([]*models.Subscription{subscription}, nil)

	// Verify subscription is passed to hub service
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(&service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}, nil)

	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	err := renewalService.RenewExpiring(context.Background())

	require.NoError(t, err)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_RenewExpiring_WithoutSecret(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	subscription := &models.Subscription{
		ID:           1,
		ChannelID:    "UCtest1",
		TopicURL:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest1",
		HubURL:       "https://pubsubhubbub.appspot.com/subscribe",
		LeaseSeconds: 432000,
		ExpiresAt:    time.Now().Add(12 * time.Hour),
		Status:       models.StatusActive,
	}

	repo.On("GetExpiringSoon", mock.Anything, 100).Return([]*models.Subscription{subscription}, nil)

	// Verify subscription is passed to hub service
	hubService.On("Subscribe", mock.Anything, mock.Anything).Return(&service.SubscribeResponse{
		Accepted:     true,
		StatusCode:   202,
		LeaseSeconds: 432000,
	}, nil)

	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	err := renewalService.RenewExpiring(context.Background())

	require.NoError(t, err)
	repo.AssertExpectations(t)
	hubService.AssertExpectations(t)
}

func TestRenewalService_RenewExpiring_ContextCancellation(t *testing.T) {
	t.Parallel()

	repo := new(mockSubscriptionRepository)
	hubService := new(mockPubSubHub)
	renewalService := &RenewalService{
		repo:       repo,
		hubService: hubService,
		logger:     newTestLogger(),
		batchSize:  100,
		webhookURL:    "https://example.com/webhook",
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Mock repository to return context cancelled error
	repo.On("GetExpiringSoon", ctx, 100).Return(nil, context.Canceled)

	// Execute renewal with cancelled context
	err := renewalService.RenewExpiring(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	repo.AssertExpectations(t)
}
