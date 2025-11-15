// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
//
//nolint:govet // fieldalignment: Accept minor memory overhead for better readability
type Config struct {
	RabbitMQ RabbitMQConfig
	Logging  LoggingConfig
	Database DatabaseConfig
	Server   ServerConfig
	Webhook  WebhookConfig
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	Port            int
	ShutdownTimeout time.Duration
}

// DatabaseConfig contains database connection configuration.
//
//nolint:govet // fieldalignment: Accept minor memory overhead for better readability
type DatabaseConfig struct {
	Host           string
	Name           string
	User           string
	Password       string
	Port           int
	MaxConnections int
	MinConnections int
	MaxIdleTime    time.Duration
	MaxLifetime    time.Duration
}

// RabbitMQConfig contains RabbitMQ connection and queue configuration.
//
//nolint:govet // fieldalignment: Accept minor memory overhead for better readability
type RabbitMQConfig struct {
	Host       string
	User       string
	Password   string
	Exchange   string
	Queue      string
	RoutingKey string
	Port       int
}

// WebhookConfig contains webhook processing configuration.
type WebhookConfig struct {
	MaxPayloadSize    int64
	ValidationEnabled bool
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	Level string
	File  string
}

// Load loads configuration from file and environment variables.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set defaults
	setDefaults()

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("APP")

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found, use defaults and env vars
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults() {
	// Server
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.shutdowntimeout", 30*time.Second)

	// Database
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.name", "adtracker")
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.maxconnections", 10)
	viper.SetDefault("database.minconnections", 5)
	viper.SetDefault("database.maxidletime", 10*time.Minute)
	viper.SetDefault("database.maxlifetime", 1*time.Hour)

	// RabbitMQ
	viper.SetDefault("rabbitmq.host", "localhost")
	viper.SetDefault("rabbitmq.port", 5672)
	viper.SetDefault("rabbitmq.user", "guest")
	viper.SetDefault("rabbitmq.password", "guest")
	viper.SetDefault("rabbitmq.exchange", "youtube.webhooks")
	viper.SetDefault("rabbitmq.queue", "youtube.webhooks.raw")
	viper.SetDefault("rabbitmq.routingkey", "webhook.received")

	// Webhook
	viper.SetDefault("webhook.maxpayloadsize", 1048576) // 1MB
	viper.SetDefault("webhook.validationenabled", true)

	// Logging
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.file", "")
}
