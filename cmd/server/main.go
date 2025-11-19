package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/handler"
	"ad-tracker/youtube-webhook-ingestion/internal/middleware"
	"ad-tracker/youtube-webhook-ingestion/internal/queue"
	"ad-tracker/youtube-webhook-ingestion/internal/service"
	"ad-tracker/youtube-webhook-ingestion/internal/service/quota"
	"ad-tracker/youtube-webhook-ingestion/internal/service/youtube"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultPort        = "8080"
	defaultWebhookPath = "/webhook"
	shutdownTimeout    = 30 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	config := loadConfig()

	ctx := context.Background()
	pool, err := initDatabase(ctx, config.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("database connection established",
		"max_conns", pool.Config().MaxConns,
	)

	webhookEventRepo := repository.NewWebhookEventRepository(pool)
	videoRepo := repository.NewVideoRepository(pool)
	channelRepo := repository.NewChannelRepository(pool)
	videoUpdateRepo := repository.NewVideoUpdateRepository(pool)
	subscriptionRepo := repository.NewSubscriptionRepository(pool)
	videoEnrichmentRepo := repository.NewEnrichmentRepository(pool)
	channelEnrichmentRepo := repository.NewChannelEnrichmentRepository(pool)
	quotaRepo := repository.NewQuotaRepository(pool)

	processor := service.NewEventProcessor(
		pool,
		webhookEventRepo,
		videoRepo,
		channelRepo,
		videoUpdateRepo,
	)

	// Initialize queue client for enrichment (optional)
	// If Redis URL is configured, set up enrichment job enqueueing
	if config.RedisURL != "" {
		jobRepo := repository.NewEnrichmentJobRepository(pool)
		queueClient, err := queue.NewClient(config.RedisURL, jobRepo)
		if err != nil {
			logger.Warn("failed to initialize queue client, enrichment jobs will not be enqueued",
				"error", err,
			)
		} else {
			processor.SetQueueClient(queueClient)
			logger.Info("queue client initialized, enrichment jobs will be enqueued for new videos")
		}
	}

	pubSubHubService := service.NewPubSubHubService(&http.Client{}, logger)

	// YouTube API client (optional - only if API key is provided)
	var youtubeClient *youtube.Client
	var quotaManager *quota.Manager
	var channelResolverService *service.ChannelResolverService

	if config.YouTubeAPIKey != "" {
		var err error
		youtubeClient, err = youtube.NewClient(config.YouTubeAPIKey)
		if err != nil {
			logger.Warn("failed to initialize YouTube API client, URL-based channel addition will not be available",
				"error", err,
			)
		} else {
			quotaManager = quota.NewManager(quotaRepo, 10000, 90)

			channelResolverService = service.NewChannelResolverService(
				youtubeClient,
				channelRepo,
				subscriptionRepo,
				channelEnrichmentRepo,
				quotaManager,
				pubSubHubService,
			)

			logger.Info("YouTube API client initialized, URL-based channel addition is available")
		}
	} else {
		logger.Info("YouTube API key not configured (YOUTUBE_API_KEY), URL-based channel addition will not be available")
	}

	webhookHandler := handler.NewWebhookHandler(processor, config.WebhookSecret, logger)

	webhookEventHandler := handler.NewWebhookEventHandler(webhookEventRepo, logger)
	channelHandler := handler.NewChannelHandler(channelRepo, logger)
	videoHandler := handler.NewVideoHandler(videoRepo, logger)
	videoUpdateHandler := handler.NewVideoUpdateHandler(videoUpdateRepo, logger)
	subscriptionCRUDHandler := handler.NewSubscriptionCRUDHandler(subscriptionRepo, pubSubHubService, config.WebhookSecret, logger)
	enrichmentHandler := handler.NewEnrichmentHandler(videoEnrichmentRepo, channelEnrichmentRepo, logger)

	// Channel from URL handler (only if YouTube API is available)
	var channelFromURLHandler *handler.ChannelFromURLHandler
	if channelResolverService != nil {
		channelFromURLHandler = handler.NewChannelFromURLHandler(channelResolverService, logger)
	}

	authMiddleware := middleware.NewAPIKeyAuth(config.APIKeys, logger)

	mux := http.NewServeMux()

	mux.Handle(config.WebhookPath, webhookHandler)

	mux.Handle("/api/v1/webhook-events", authMiddleware.Middleware(webhookEventHandler))
	mux.Handle("/api/v1/webhook-events/", authMiddleware.Middleware(webhookEventHandler))
	mux.Handle("/api/v1/channels", authMiddleware.Middleware(channelHandler))
	mux.Handle("/api/v1/channels/", authMiddleware.Middleware(channelHandler))

	// Channel from URL endpoint (only available if YouTube API is configured)
	if channelFromURLHandler != nil {
		mux.Handle("/api/v1/channels/from-url", authMiddleware.Middleware(http.HandlerFunc(channelFromURLHandler.HandleCreateFromURL)))
	}

	mux.Handle("/api/v1/videos", authMiddleware.Middleware(videoHandler))
	mux.Handle("/api/v1/videos/", authMiddleware.Middleware(videoHandler))
	mux.Handle("/api/v1/video-updates", authMiddleware.Middleware(videoUpdateHandler))
	mux.Handle("/api/v1/video-updates/", authMiddleware.Middleware(videoUpdateHandler))
	mux.Handle("/api/v1/subscriptions", authMiddleware.Middleware(subscriptionCRUDHandler))
	mux.Handle("/api/v1/subscriptions/", authMiddleware.Middleware(subscriptionCRUDHandler))
	mux.Handle("/api/v1/enrichments/", authMiddleware.Middleware(enrichmentHandler))

	mux.HandleFunc("/health", handleHealth(pool))

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      loggingMiddleware(logger)(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server starting",
			"port", config.Port,
			"webhook_path", config.WebhookPath,
		)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error", "error", err)
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			if err := server.Close(); err != nil {
				logger.Error("failed to close server", "error", err)
			}
			os.Exit(1)
		}

		logger.Info("server stopped gracefully")
	}
}

// Config holds application configuration.
type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	WebhookSecret string
	WebhookPath   string
	APIKeys       []string
	YouTubeAPIKey string
}

// loadConfig loads configuration from environment variables.
func loadConfig() *Config {
	config := &Config{
		Port:          getEnv("PORT", defaultPort),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisURL:      getEnv("REDIS_URL", ""),
		WebhookSecret: getEnv("WEBHOOK_SECRET", ""),
		WebhookPath:   getEnv("WEBHOOK_PATH", defaultWebhookPath),
		APIKeys:       parseAPIKeys(getEnv("API_KEYS", "")),
		YouTubeAPIKey: getEnv("YOUTUBE_API_KEY", ""),
	}

	if config.DatabaseURL == "" {
		slog.Error("DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	if len(config.APIKeys) == 0 {
		slog.Warn("no API keys configured - subscription endpoints will reject all requests",
			"env_var", "API_KEYS",
		)
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

// parseAPIKeys parses a comma-separated list of API keys.
// Empty strings and whitespace are trimmed from each key.
func parseAPIKeys(apiKeysEnv string) []string {
	if apiKeysEnv == "" {
		return nil
	}

	parts := strings.Split(apiKeysEnv, ",")
	keys := make([]string, 0, len(parts))

	for _, key := range parts {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}

	return keys
}

// initDatabase initializes the database connection pool.
func initDatabase(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
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

// handleHealth returns a health check handler that verifies database connectivity.
func handleHealth(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		// Check database connectivity
		if err := pool.Ping(ctx); err != nil {
			slog.Error("health check failed", "error", err)
			http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","database":"connected"}`))
	}
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// Ensure responseWriter implements http.ResponseWriter
var _ http.ResponseWriter = (*responseWriter)(nil)
