package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/config"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/ad-tracker/youtube-webhook-ingestion-go/pkg/logger"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// MessagePublisher handles publishing webhook events to RabbitMQ.
type MessagePublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.RabbitMQConfig
	mu      sync.RWMutex
}

// NewMessagePublisher creates and initializes a new RabbitMQ message publisher.
func NewMessagePublisher(cfg *config.RabbitMQConfig) (*MessagePublisher, error) {
	mp := &MessagePublisher{
		config: cfg,
	}

	if err := mp.connect(); err != nil {
		return nil, err
	}

	return mp, nil
}

func (mp *MessagePublisher) connect() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	connURL := fmt.Sprintf("amqp://%s:%s@%s:%d/",
		mp.config.User, mp.config.Password, mp.config.Host, mp.config.Port)

	conn, err := amqp.Dial(connURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close() //nolint:errcheck // Cleanup on error path
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Enable publisher confirms
	if confirmErr := ch.Confirm(false); confirmErr != nil {
		_ = ch.Close()   //nolint:errcheck // Cleanup on error path
		_ = conn.Close() //nolint:errcheck // Cleanup on error path
		return fmt.Errorf("failed to enable publisher confirms: %w", confirmErr)
	}

	// Declare exchange
	if exchangeErr := ch.ExchangeDeclare(
		mp.config.Exchange, // name
		"topic",            // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	); exchangeErr != nil {
		_ = ch.Close()   //nolint:errcheck // Cleanup on error path
		_ = conn.Close() //nolint:errcheck // Cleanup on error path
		return fmt.Errorf("failed to declare exchange: %w", exchangeErr)
	}

	// Declare queue
	_, err = ch.QueueDeclare(
		mp.config.Queue, // name
		true,            // durable
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		amqp.Table{
			"x-message-ttl":    86400000, // 24 hours
			"x-message-length": 100000,   // max 100k messages
		},
	)
	if err != nil {
		_ = ch.Close()   //nolint:errcheck // Cleanup on error path
		_ = conn.Close() //nolint:errcheck // Cleanup on error path
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	if bindErr := ch.QueueBind(
		mp.config.Queue,      // queue name
		mp.config.RoutingKey, // routing key
		mp.config.Exchange,   // exchange
		false,
		nil,
	); bindErr != nil {
		_ = ch.Close()   //nolint:errcheck // Cleanup on error path
		_ = conn.Close() //nolint:errcheck // Cleanup on error path
		return fmt.Errorf("failed to bind queue: %w", bindErr)
	}

	mp.conn = conn
	mp.channel = ch

	logger.Log.Info("Connected to RabbitMQ",
		zap.String("exchange", mp.config.Exchange),
		zap.String("queue", mp.config.Queue),
	)

	return nil
}

// PublishEvent publishes a webhook event to RabbitMQ with publisher confirms.
func (mp *MessagePublisher) PublishEvent(ctx context.Context, event *models.WebhookEvent) error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if mp.channel == nil {
		return fmt.Errorf("channel is not initialized")
	}

	// Serialize event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with confirmation
	confirms := mp.channel.NotifyPublish(make(chan amqp.Confirmation, 1))

	err = mp.channel.PublishWithContext(
		ctx,
		mp.config.Exchange,   // exchange
		mp.config.RoutingKey, // routing key
		true,                 // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    event.ID.String(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for confirmation with timeout
	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("message was not acknowledged by broker")
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for publish confirmation")
	case <-ctx.Done():
		return ctx.Err()
	}

	logger.Log.Debug("Published event to RabbitMQ",
		zap.String("eventId", event.ID.String()),
		zap.String("routingKey", mp.config.RoutingKey),
	)

	return nil
}

// Close closes the RabbitMQ connection and channel.
func (mp *MessagePublisher) Close() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	var errs []error
	if mp.channel != nil {
		if err := mp.channel.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if mp.conn != nil {
		if err := mp.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing publisher: %v", errs)
	}

	logger.Log.Info("RabbitMQ publisher closed")
	return nil
}

// IsHealthy checks if the RabbitMQ connection is healthy.
func (mp *MessagePublisher) IsHealthy() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	return mp.conn != nil && !mp.conn.IsClosed() && mp.channel != nil
}
