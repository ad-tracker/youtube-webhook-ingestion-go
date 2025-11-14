package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/config"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/handler"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/repository"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/service"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/validation"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Log.Info("Starting YouTube Webhook Ingestion Service")

	// Initialize database connection
	dbConfig := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s pool_max_conns=%d pool_min_conns=%d",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name,
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.MaxConnections, cfg.Database.MinConnections,
	)

	db, err := pgxpool.New(context.Background(), dbConfig)
	if err != nil {
		logger.Log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	logger.Log.Info("Connected to database",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
		zap.String("database", cfg.Database.Name),
	)

	// Initialize repository
	repo := repository.New(db)

	// Initialize RabbitMQ publisher
	publisher, err := service.NewMessagePublisher(&cfg.RabbitMQ)
	if err != nil {
		logger.Log.Fatal("Failed to initialize RabbitMQ publisher", zap.Error(err))
	}
	defer publisher.Close()

	// Initialize validator
	validator := validation.New(cfg.Webhook.MaxPayloadSize, cfg.Webhook.ValidationEnabled)

	// Initialize webhook service
	webhookService := service.NewWebhookService(repo, publisher, validator)

	// Initialize handlers
	webhookHandler := handler.NewWebhookHandler(webhookService)
	healthHandler := handler.NewHealthHandler(repo, publisher)

	// Set Gin mode
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Setup router
	router := gin.Default()

	// API routes
	api := router.Group("/api/v1")
	{
		webhooks := api.Group("/webhooks")
		{
			webhooks.POST("/youtube", webhookHandler.HandleYouTubeWebhook)
			webhooks.GET("/health", webhookHandler.HealthCheck)
		}
	}

	// Actuator routes
	actuator := router.Group("/actuator")
	{
		health := actuator.Group("/health")
		{
			health.GET("/liveness", healthHandler.LivenessProbe)
			health.GET("/readiness", healthHandler.ReadinessProbe)
			health.GET("", healthHandler.ReadinessProbe) // Default health endpoint
		}

		// Prometheus metrics
		actuator.GET("/metrics/prometheus", gin.WrapH(promhttp.Handler()))
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Log.Info("Starting HTTP server",
			zap.Int("port", cfg.Server.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Log.Info("Server exited successfully")
}
