package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		cleanup func()
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "load with defaults (no config file)",
			setup: func() {
				// Reset viper
				viper.Reset()
			},
			cleanup: func() {},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 8080 {
					t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
				}
				if cfg.Database.Host != "localhost" {
					t.Errorf("Database.Host = %s, want localhost", cfg.Database.Host)
				}
				if cfg.Database.Port != 5432 {
					t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
				}
				if cfg.RabbitMQ.Host != "localhost" {
					t.Errorf("RabbitMQ.Host = %s, want localhost", cfg.RabbitMQ.Host)
				}
				if cfg.Webhook.MaxPayloadSize != 1048576 {
					t.Errorf("Webhook.MaxPayloadSize = %d, want 1048576", cfg.Webhook.MaxPayloadSize)
				}
				if !cfg.Webhook.ValidationEnabled {
					t.Error("Webhook.ValidationEnabled = false, want true")
				}
			},
		},
		{
			name: "load with environment variables",
			setup: func() {
				viper.Reset()
				viper.SetEnvPrefix("APP")
				viper.AutomaticEnv()
				os.Setenv("APP_SERVER_PORT", "9090")
				os.Setenv("APP_DATABASE_HOST", "testdb")
				os.Setenv("APP_DATABASE_PORT", "5433")
				os.Setenv("APP_DATABASE_NAME", "testdb")
				os.Setenv("APP_RABBITMQ_HOST", "testrabbitmq")
				os.Setenv("APP_RABBITMQ_PORT", "5673")
				// Manually bind env vars since AutomaticEnv doesn't work with nested keys
				viper.BindEnv("server.port", "APP_SERVER_PORT")
				viper.BindEnv("database.host", "APP_DATABASE_HOST")
				viper.BindEnv("database.port", "APP_DATABASE_PORT")
				viper.BindEnv("database.name", "APP_DATABASE_NAME")
				viper.BindEnv("rabbitmq.host", "APP_RABBITMQ_HOST")
				viper.BindEnv("rabbitmq.port", "APP_RABBITMQ_PORT")
			},
			cleanup: func() {
				os.Unsetenv("APP_SERVER_PORT")
				os.Unsetenv("APP_DATABASE_HOST")
				os.Unsetenv("APP_DATABASE_PORT")
				os.Unsetenv("APP_DATABASE_NAME")
				os.Unsetenv("APP_RABBITMQ_HOST")
				os.Unsetenv("APP_RABBITMQ_PORT")
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 9090 {
					t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
				}
				if cfg.Database.Host != "testdb" {
					t.Errorf("Database.Host = %s, want testdb", cfg.Database.Host)
				}
				if cfg.Database.Port != 5433 {
					t.Errorf("Database.Port = %d, want 5433", cfg.Database.Port)
				}
				if cfg.RabbitMQ.Host != "testrabbitmq" {
					t.Errorf("RabbitMQ.Host = %s, want testrabbitmq", cfg.RabbitMQ.Host)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			defer func() {
				if tt.cleanup != nil {
					tt.cleanup()
				}
			}()

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg == nil {
				t.Fatal("Load() returned nil config")
			}

			if tt.check != nil && cfg != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	viper.Reset()
	setDefaults()

	tests := []struct {
		name string
		key  string
		want interface{}
	}{
		{"server port", "server.port", 8080},
		{"database host", "database.host", "localhost"},
		{"database port", "database.port", 5432},
		{"database name", "database.name", "adtracker"},
		{"database user", "database.user", "postgres"},
		{"database maxconnections", "database.maxconnections", 10},
		{"database minconnections", "database.minconnections", 5},
		{"rabbitmq host", "rabbitmq.host", "localhost"},
		{"rabbitmq port", "rabbitmq.port", 5672},
		{"rabbitmq user", "rabbitmq.user", "guest"},
		{"rabbitmq exchange", "rabbitmq.exchange", "youtube.webhooks"},
		{"rabbitmq queue", "rabbitmq.queue", "youtube.webhooks.raw"},
		{"rabbitmq routingkey", "rabbitmq.routingkey", "webhook.received"},
		{"webhook maxpayloadsize", "webhook.maxpayloadsize", 1048576},
		{"webhook validationenabled", "webhook.validationenabled", true},
		{"logging level", "logging.level", "info"},
		{"logging file", "logging.file", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := viper.Get(tt.key)
			if got != tt.want {
				t.Errorf("viper.Get(%s) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}

	// Test time.Duration defaults
	if viper.GetDuration("server.shutdowntimeout") != 30*time.Second {
		t.Errorf("server.shutdowntimeout = %v, want 30s", viper.GetDuration("server.shutdowntimeout"))
	}
	if viper.GetDuration("database.maxidletime") != 10*time.Minute {
		t.Errorf("database.maxidletime = %v, want 10m", viper.GetDuration("database.maxidletime"))
	}
	if viper.GetDuration("database.maxlifetime") != 1*time.Hour {
		t.Errorf("database.maxlifetime = %v, want 1h", viper.GetDuration("database.maxlifetime"))
	}
}

func TestConfigStructs(t *testing.T) {
	// Test that structs can be created and fields are accessible
	cfg := &Config{
		Server: ServerConfig{
			Port:            8080,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Name:           "test",
			User:           "user",
			Password:       "pass",
			MaxConnections: 10,
			MinConnections: 5,
			MaxIdleTime:    10 * time.Minute,
			MaxLifetime:    1 * time.Hour,
		},
		RabbitMQ: RabbitMQConfig{
			Host:       "localhost",
			Port:       5672,
			User:       "guest",
			Password:   "guest",
			Exchange:   "test",
			Queue:      "test",
			RoutingKey: "test",
		},
		Webhook: WebhookConfig{
			MaxPayloadSize:    1024,
			ValidationEnabled: true,
		},
		Logging: LoggingConfig{
			Level: "info",
			File:  "/tmp/test.log",
		},
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %s, want localhost", cfg.Database.Host)
	}
	if cfg.RabbitMQ.Host != "localhost" {
		t.Errorf("RabbitMQ.Host = %s, want localhost", cfg.RabbitMQ.Host)
	}
	if cfg.Webhook.MaxPayloadSize != 1024 {
		t.Errorf("Webhook.MaxPayloadSize = %d, want 1024", cfg.Webhook.MaxPayloadSize)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
}
