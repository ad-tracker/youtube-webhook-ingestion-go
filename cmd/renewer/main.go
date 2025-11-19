package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultRenewalInterval = 6 * time.Hour // Check every 6 hours
	defaultBatchSize       = 100           // Process up to 100 subscriptions per run
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	config := loadConfig()

	logger.Info("subscription renewal service starting",
		"renewal_interval", config.RenewalInterval,
		"batch_size", config.BatchSize,
	)

	// Initialize database connection
	ctx := context.Background()
	pool, err := initDatabase(ctx, config.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("database connection established")

	// Initialize repository and service
	subscriptionRepo := repository.NewSubscriptionRepository(pool)
	pubSubHubService := service.NewPubSubHubService(&http.Client{}, logger)

	// Create renewal service
	renewalService := &RenewalService{
		repo:          subscriptionRepo,
		hubService:    pubSubHubService,
		logger:        logger,
		batchSize:     config.BatchSize,
		webhookSecret: config.WebhookSecret,
	}

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for periodic renewal checks
	ticker := time.NewTicker(config.RenewalInterval)
	defer ticker.Stop()

	// Run initial renewal check immediately
	logger.Info("running initial renewal check")
	if err := renewalService.RenewExpiring(ctx); err != nil {
		logger.Error("initial renewal check failed", "error", err)
	}

	// Main loop
	for {
		select {
		case <-ticker.C:
			logger.Info("running scheduled renewal check")
			if err := renewalService.RenewExpiring(ctx); err != nil {
				logger.Error("scheduled renewal check failed", "error", err)
			}
		case sig := <-shutdown:
			logger.Info("shutdown signal received", "signal", sig)
			logger.Info("renewal service stopped gracefully")
			return
		}
	}
}

// RenewalService handles automatic renewal of expiring subscriptions.
type RenewalService struct {
	repo          repository.SubscriptionRepository
	hubService    service.PubSubHub
	logger        *slog.Logger
	batchSize     int
	webhookSecret string
}

// RenewExpiring finds expiring subscriptions and renews them.
func (s *RenewalService) RenewExpiring(ctx context.Context) error {
	// Get subscriptions expiring within 24 hours
	subscriptions, err := s.repo.GetExpiringSoon(ctx, s.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get expiring subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		s.logger.Info("no subscriptions need renewal")
		return nil
	}

	s.logger.Info("found subscriptions to renew", "count", len(subscriptions))

	// Renew each subscription
	successCount := 0
	failureCount := 0

	for _, sub := range subscriptions {
		if err := s.renewSubscription(ctx, sub); err != nil {
			s.logger.Error("failed to renew subscription",
				"subscription_id", sub.ID,
				"channel_id", sub.ChannelID,
				"error", err,
			)
			failureCount++
		} else {
			s.logger.Info("successfully renewed subscription",
				"subscription_id", sub.ID,
				"channel_id", sub.ChannelID,
				"new_expires_at", sub.ExpiresAt,
			)
			successCount++
		}
	}

	s.logger.Info("renewal batch completed",
		"total", len(subscriptions),
		"successful", successCount,
		"failed", failureCount,
	)

	return nil
}

// renewSubscription renews a single subscription.
func (s *RenewalService) renewSubscription(ctx context.Context, sub *models.Subscription) error {
	// Create subscription request
	hubReq := &service.SubscribeRequest{
		HubURL:       sub.HubURL,
		TopicURL:     sub.TopicURL,
		CallbackURL:  sub.CallbackURL,
		LeaseSeconds: sub.LeaseSeconds,
		Secret:       &s.webhookSecret,
	}

	// Subscribe via PubSubHub
	hubResp, err := s.hubService.Subscribe(ctx, hubReq)
	if err != nil {
		// Mark as failed
		sub.MarkFailed()
		if updateErr := s.repo.Update(ctx, sub); updateErr != nil {
			s.logger.Error("failed to mark subscription as failed",
				"subscription_id", sub.ID,
				"error", updateErr,
			)
		}
		return fmt.Errorf("hub subscription failed: %w", err)
	}

	// Update subscription based on response
	if hubResp.Accepted {
		sub.MarkActive()
		sub.UpdateExpiry(sub.LeaseSeconds)
	} else {
		sub.MarkFailed()
	}

	// Save updated subscription
	if err := s.repo.Update(ctx, sub); err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	return nil
}

// Config holds application configuration.
type Config struct {
	DatabaseURL     string
	WebhookSecret   string
	RenewalInterval time.Duration
	BatchSize       int
}

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	config := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		WebhookSecret:   getEnv("WEBHOOK_SECRET", ""),
		RenewalInterval: parseDuration(getEnv("RENEWAL_INTERVAL", "6h")),
		BatchSize:       parseInt(getEnv("BATCH_SIZE", "100")),
	}

	if config.DatabaseURL == "" {
		slog.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	if config.WebhookSecret == "" {
		slog.Error("WEBHOOK_SECRET environment variable is required",
			"help", "This secret is used when renewing subscriptions with YouTube PubSubHub",
		)
		os.Exit(1)
	}

	return config
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseDuration parses a duration string.
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid duration, using default",
			"value", s,
			"default", defaultRenewalInterval,
			"error", err,
		)
		return defaultRenewalInterval
	}
	return d
}

// parseInt parses an integer string.
func parseInt(s string) int {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		slog.Warn("invalid integer, using default",
			"value", s,
			"default", defaultBatchSize,
			"error", err,
		)
		return defaultBatchSize
	}
	return i
}

// initDatabase initializes the database connection pool.
func initDatabase(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
