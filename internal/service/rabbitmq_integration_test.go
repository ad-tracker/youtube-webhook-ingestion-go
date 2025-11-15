//go:build integration
// +build integration

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/config"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	loggerInitOnce sync.Once
	loggerInitErr  error
)

func initTestLogger() error {
	loggerInitOnce.Do(func() {
		loggerInitErr = logger.Init("test", "development")
	})
	return loggerInitErr
}

func setupTestRabbitMQ(t *testing.T) (*config.RabbitMQConfig, func()) {
	// Initialize logger for tests
	if err := initTestLogger(); err != nil {
		t.Fatalf("Failed to initialize test logger: %v", err)
	}

	ctx := context.Background()

	rabbitmqContainer, err := rabbitmq.Run(ctx,
		"rabbitmq:3.13-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Server startup complete").
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start rabbitmq container: %v", err)
	}

	host, err := rabbitmqContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	port, err := rabbitmqContainer.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}

	cfg := &config.RabbitMQConfig{
		Host:       host,
		Port:       port.Int(),
		User:       "guest",
		Password:   "guest",
		Exchange:   "test.exchange",
		Queue:      "test.queue",
		RoutingKey: "test.key",
	}

	cleanup := func() {
		if err := rabbitmqContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return cfg, cleanup
}

func TestNewMessagePublisher(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg, cleanup := setupTestRabbitMQ(t)
	defer cleanup()

	// Allow some time for RabbitMQ to be fully ready
	time.Sleep(2 * time.Second)

	mp, err := NewMessagePublisher(cfg)
	if err != nil {
		t.Fatalf("NewMessagePublisher() error = %v", err)
	}
	defer mp.Close()

	if mp == nil {
		t.Fatal("NewMessagePublisher() returned nil")
	}
}

func TestMessagePublisher_PublishEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg, cleanup := setupTestRabbitMQ(t)
	defer cleanup()

	time.Sleep(2 * time.Second)

	mp, err := NewMessagePublisher(cfg)
	if err != nil {
		t.Fatalf("NewMessagePublisher() error = %v", err)
	}
	defer mp.Close()

	ctx := context.Background()
	event := &models.WebhookEvent{
		ID:               uuid.New(),
		VideoID:          "test123",
		ChannelID:        "UCtest",
		EventType:        "TEST_EVENT",
		Payload:          `{"test": "message"}`,
		SourceIP:         "127.0.0.1",
		UserAgent:        "test-agent",
		Processed:        false,
		ProcessingStatus: models.ProcessingStatusPending,
		RetryCount:       0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err = mp.PublishEvent(ctx, event)
	if err != nil {
		t.Errorf("PublishEvent() error = %v", err)
	}
}

func TestMessagePublisher_IsHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg, cleanup := setupTestRabbitMQ(t)
	defer cleanup()

	time.Sleep(2 * time.Second)

	mp, err := NewMessagePublisher(cfg)
	if err != nil {
		t.Fatalf("NewMessagePublisher() error = %v", err)
	}
	defer mp.Close()

	if !mp.IsHealthy() {
		t.Error("IsHealthy() = false, want true")
	}

	// Close and check unhealthy
	mp.Close()
	if mp.IsHealthy() {
		t.Error("IsHealthy() after Close() = true, want false")
	}
}

func TestMessagePublisher_Reconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg, cleanup := setupTestRabbitMQ(t)
	defer cleanup()

	time.Sleep(2 * time.Second)

	mp, err := NewMessagePublisher(cfg)
	if err != nil {
		t.Fatalf("NewMessagePublisher() error = %v", err)
	}
	defer mp.Close()

	// Close the connection
	if mp.conn != nil {
		mp.conn.Close()
	}

	// Try to publish (should fail since connection is closed and no auto-reconnect)
	ctx := context.Background()
	event := &models.WebhookEvent{
		ID:               uuid.New(),
		VideoID:          "test123",
		ChannelID:        "UCtest",
		EventType:        "TEST_EVENT",
		Payload:          `{"test": "reconnect"}`,
		SourceIP:         "127.0.0.1",
		UserAgent:        "test-agent",
		Processed:        false,
		ProcessingStatus: models.ProcessingStatusPending,
		RetryCount:       0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// This should fail since connection is closed, but shouldn't panic
	_ = mp.PublishEvent(ctx, event)
}
