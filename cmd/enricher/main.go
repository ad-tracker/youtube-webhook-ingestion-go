package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/queue"
	"ad-tracker/youtube-webhook-ingestion/internal/service/quota"
	"ad-tracker/youtube-webhook-ingestion/internal/service/youtube"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultConcurrency    = 2
	defaultBatchSize      = 50
	defaultDailyQuota     = 10000
	defaultQuotaThreshold = 90 // Stop at 90% of quota
)

type Config struct {
	DatabaseURL       string
	RedisURL          string
	YouTubeAPIKey     string
	DailyQuota        int
	QuotaThreshold    int
	Concurrency       int
	BatchSize         int
	EnrichmentEnabled bool
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	config := loadConfig()

	logger.Info("enrichment service starting",
		"concurrency", config.Concurrency,
		"batch_size", config.BatchSize,
		"daily_quota", config.DailyQuota,
		"quota_threshold", config.QuotaThreshold,
		"enabled", config.EnrichmentEnabled,
	)

	// Check if enrichment is enabled
	if !config.EnrichmentEnabled {
		logger.Info("enrichment service is disabled via configuration")
		os.Exit(0)
	}

	// Validate YouTube API key
	if config.YouTubeAPIKey == "" {
		logger.Error("YOUTUBE_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Initialize database connection
	ctx := context.Background()
	pool, err := initDatabase(ctx, config.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("database connection established")

	// Initialize repositories
	enrichmentRepo := repository.NewEnrichmentRepository(pool)
	quotaRepo := repository.NewQuotaRepository(pool)
	jobRepo := repository.NewEnrichmentJobRepository(pool)

	// Initialize YouTube API client
	youtubeClient, err := youtube.NewClient(config.YouTubeAPIKey)
	if err != nil {
		logger.Error("failed to initialize YouTube client", "error", err)
		os.Exit(1)
	}

	logger.Info("YouTube API client initialized")

	// Initialize quota manager
	quotaManager := quota.NewManager(quotaRepo, config.DailyQuota, config.QuotaThreshold)

	// Check initial quota status
	quotaInfo, err := quotaManager.GetQuotaInfo(ctx)
	if err != nil {
		logger.Error("failed to get quota info", "error", err)
		os.Exit(1)
	}

	logger.Info("quota status",
		"used", quotaInfo.QuotaUsed,
		"limit", config.DailyQuota,
		"remaining", quotaInfo.QuotaRemaining,
		"operations", quotaInfo.OperationsCount,
	)

	// Check if quota is already exhausted
	exhausted, err := quotaManager.IsQuotaExhausted(ctx)
	if err != nil {
		logger.Error("failed to check quota status", "error", err)
		os.Exit(1)
	}

	if exhausted {
		logger.Warn("daily quota threshold already reached, service will not process tasks",
			"used", quotaInfo.QuotaUsed,
			"threshold", config.QuotaThreshold,
		)
		// Don't exit - still start server to drain queue when quota resets
	}

	// Initialize task handler
	handler := queue.NewEnrichmentHandler(
		youtubeClient,
		quotaManager,
		enrichmentRepo,
		jobRepo,
		config.BatchSize,
	)

	// Initialize and start asynq server
	server, err := queue.NewServer(config.RedisURL, config.Concurrency, handler)
	if err != nil {
		logger.Error("failed to create queue server", "error", err)
		os.Exit(1)
	}

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting task processing server")
		if err := server.Run(); err != nil {
			serverErr <- err
		}
	}()

	logger.Info("enrichment service started successfully")

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		logger.Error("server error", "error", err)
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig)
		server.Stop()
		logger.Info("enrichment service stopped gracefully")
	}
}

func loadConfig() *Config {
	// Get environment variables with defaults
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	youtubeAPIKey := os.Getenv("YOUTUBE_API_KEY")

	// Parse numeric configs
	dailyQuota := getEnvInt("YOUTUBE_DAILY_QUOTA", defaultDailyQuota)
	quotaThreshold := getEnvInt("QUOTA_THRESHOLD_PERCENT", defaultQuotaThreshold)
	concurrency := getEnvInt("ENRICHMENT_WORKERS", defaultConcurrency)
	batchSize := getEnvInt("ENRICHMENT_BATCH_SIZE", defaultBatchSize)

	// Parse boolean config
	enrichmentEnabled := getEnvBool("ENRICHMENT_ENABLED", true)

	return &Config{
		DatabaseURL:       databaseURL,
		RedisURL:          redisURL,
		YouTubeAPIKey:     youtubeAPIKey,
		DailyQuota:        dailyQuota,
		QuotaThreshold:    quotaThreshold,
		Concurrency:       concurrency,
		BatchSize:         batchSize,
		EnrichmentEnabled: enrichmentEnabled,
	}
}

func initDatabase(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func getEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(val)
	if err != nil {
		slog.Warn("invalid integer value for environment variable, using default",
			"key", key,
			"value", val,
			"default", defaultValue,
		)
		return defaultValue
	}

	return intVal
}

func getEnvBool(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		slog.Warn("invalid boolean value for environment variable, using default",
			"key", key,
			"value", val,
			"default", defaultValue,
		)
		return defaultValue
	}

	return boolVal
}
