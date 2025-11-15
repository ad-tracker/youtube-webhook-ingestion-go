//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Create schema and tables
	_, err = pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS webhook_ingestion`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS webhook_ingestion.webhook_events (
			id UUID PRIMARY KEY,
			video_id VARCHAR(50),
			channel_id VARCHAR(50) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			payload TEXT,
			source_ip VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP,
			error_message TEXT,
			processed BOOLEAN DEFAULT FALSE,
			processing_status VARCHAR(20) DEFAULT 'PENDING',
			retry_count INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create webhook_events table: %v", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS webhook_ingestion.events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_type VARCHAR(50) NOT NULL,
			channel_id VARCHAR(50) NOT NULL,
			video_id VARCHAR(50),
			raw_xml TEXT,
			event_hash VARCHAR(64) UNIQUE NOT NULL,
			received_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create events table: %v", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS webhook_ingestion.subscriptions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			channel_id VARCHAR(50) UNIQUE NOT NULL,
			topic_url TEXT NOT NULL,
			callback_url TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			lease_expires_at TIMESTAMP,
			next_renewal_at TIMESTAMP,
			last_renewed_at TIMESTAMP,
			last_renewal_error TEXT,
			subscription_status VARCHAR(20) DEFAULT 'PENDING',
			lease_seconds INTEGER DEFAULT 432000,
			renewal_attempts INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create subscriptions table: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

func TestRepository_CreateWebhookEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	event := &models.WebhookEvent{
		ID:               uuid.New(),
		VideoID:          "test123",
		ChannelID:        "UCtest",
		EventType:        "TEST_EVENT",
		Payload:          `{"test": "data"}`,
		SourceIP:         "127.0.0.1",
		UserAgent:        "test-agent",
		Processed:        false,
		ProcessingStatus: models.ProcessingStatusPending,
		RetryCount:       0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.CreateWebhookEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateWebhookEvent() error = %v", err)
	}

	// Verify it was created
	retrieved, err := repo.GetWebhookEventByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetWebhookEventByID() error = %v", err)
	}

	if retrieved.VideoID != event.VideoID {
		t.Errorf("VideoID = %s, want %s", retrieved.VideoID, event.VideoID)
	}
	if retrieved.ChannelID != event.ChannelID {
		t.Errorf("ChannelID = %s, want %s", retrieved.ChannelID, event.ChannelID)
	}
}

func TestRepository_UpdateWebhookEventStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	// Create an event first
	eventID := uuid.New()
	event := &models.WebhookEvent{
		ID:               eventID,
		VideoID:          "test123",
		ChannelID:        "UCtest",
		EventType:        "TEST_EVENT",
		Payload:          `{"test": "data"}`,
		SourceIP:         "127.0.0.1",
		UserAgent:        "test-agent",
		Processed:        false,
		ProcessingStatus: models.ProcessingStatusPending,
		RetryCount:       0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.CreateWebhookEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateWebhookEvent() error = %v", err)
	}

	// Update status
	errorMsg := "test error"
	err = repo.UpdateWebhookEventStatus(ctx, eventID, models.ProcessingStatusFailed, &errorMsg)
	if err != nil {
		t.Fatalf("UpdateWebhookEventStatus() error = %v", err)
	}

	// Verify update
	retrieved, err := repo.GetWebhookEventByID(ctx, eventID)
	if err != nil {
		t.Fatalf("GetWebhookEventByID() error = %v", err)
	}

	if retrieved.ProcessingStatus != models.ProcessingStatusFailed {
		t.Errorf("ProcessingStatus = %s, want %s", retrieved.ProcessingStatus, models.ProcessingStatusFailed)
	}
}

func TestRepository_CreateEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	event := &models.Event{
		EventType:  "TEST_EVENT",
		ChannelID:  "UCtest",
		VideoID:    "test123",
		RawXML:     "<xml>test</xml>",
		EventHash:  "testhash123",
		ReceivedAt: time.Now(),
	}

	err := repo.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}

	// Verify ID and CreatedAt were set by database
	if event.ID == uuid.Nil {
		t.Error("Event ID should be set by database")
	}
	if event.CreatedAt.IsZero() {
		t.Error("Event CreatedAt should be set by database")
	}

	// Check if exists by hash
	exists, err := repo.EventExistsByHash(ctx, event.EventHash)
	if err != nil {
		t.Fatalf("EventExistsByHash() error = %v", err)
	}

	if !exists {
		t.Error("Event should exist by hash")
	}
}

func TestRepository_GetEventsByChannelID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	channelID := "UCtest123"

	// Create multiple events
	for i := 0; i < 3; i++ {
		event := &models.Event{
			EventType:  "TEST_EVENT",
			ChannelID:  channelID,
			VideoID:    "test123",
			RawXML:     "<xml>test</xml>",
			EventHash:  uuid.New().String(),
			ReceivedAt: time.Now(),
		}
		if err := repo.CreateEvent(ctx, event); err != nil {
			t.Fatalf("CreateEvent() error = %v", err)
		}
	}

	events, err := repo.GetEventsByChannelID(ctx, channelID, 10)
	if err != nil {
		t.Fatalf("GetEventsByChannelID() error = %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Got %d events, want 3", len(events))
	}
}

func TestRepository_CreateSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	sub := &models.Subscription{
		ChannelID:          "UCtest",
		TopicURL:           "https://example.com/topic",
		CallbackURL:        "https://example.com/callback",
		SubscriptionStatus: models.SubscriptionStatusPending,
		LeaseSeconds:       432000,
		RenewalAttempts:    0,
	}

	err := repo.CreateSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("CreateSubscription() error = %v", err)
	}

	// Verify ID, CreatedAt, and UpdatedAt were set by database
	if sub.ID == uuid.Nil {
		t.Error("Subscription ID should be set by database")
	}
	if sub.CreatedAt.IsZero() {
		t.Error("Subscription CreatedAt should be set by database")
	}
	if sub.UpdatedAt.IsZero() {
		t.Error("Subscription UpdatedAt should be set by database")
	}

	// Retrieve it
	retrieved, err := repo.GetSubscriptionByChannelID(ctx, sub.ChannelID)
	if err != nil {
		t.Fatalf("GetSubscriptionByChannelID() error = %v", err)
	}

	if retrieved.TopicURL != sub.TopicURL {
		t.Errorf("TopicURL = %s, want %s", retrieved.TopicURL, sub.TopicURL)
	}
}

func TestRepository_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := New(pool)
	ctx := context.Background()

	err := repo.Ping(ctx)
	if err != nil {
		t.Errorf("Ping() error = %v, expected nil", err)
	}
}
