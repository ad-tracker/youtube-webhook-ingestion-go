// Package repository provides database operations for the webhook ingestion service.
package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ad-tracker/youtube-webhook-ingestion-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey string

const txKey ctxKey = "tx"

// Repository handles all database operations for the webhook ingestion service.
type Repository struct {
	db *pgxpool.Pool
}

// New creates a new Repository instance with the provided database connection pool.
func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// WebhookEvent methods

// CreateWebhookEvent inserts a new webhook event into the database.
func (r *Repository) CreateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	query := `
		INSERT INTO webhook_ingestion.webhook_events
		(id, video_id, channel_id, event_type, payload, source_ip, user_agent, processed, processing_status, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		event.ID, event.VideoID, event.ChannelID, event.EventType, event.Payload,
		event.SourceIP, event.UserAgent, event.Processed, event.ProcessingStatus, event.RetryCount,
	)
	return err
}

// UpdateWebhookEventStatus updates the processing status of a webhook event.
func (r *Repository) UpdateWebhookEventStatus(ctx context.Context, id uuid.UUID, status models.ProcessingStatus, errorMsg *string) error {
	now := time.Now()
	query := `
		UPDATE webhook_ingestion.webhook_events
		SET processing_status = $2, error_message = $3, processed = $4, processed_at = $5, updated_at = $6
		WHERE id = $1
	`
	processed := status == models.ProcessingStatusCompleted
	_, err := r.db.Exec(ctx, query, id, status, errorMsg, processed, now, now)
	return err
}

// GetWebhookEventByID retrieves a webhook event by its ID.
func (r *Repository) GetWebhookEventByID(ctx context.Context, id uuid.UUID) (*models.WebhookEvent, error) {
	query := `
		SELECT id, video_id, channel_id, event_type, payload, source_ip, user_agent,
		       processed, processing_status, error_message, retry_count,
		       created_at, updated_at, processed_at
		FROM webhook_ingestion.webhook_events
		WHERE id = $1
	`
	var event models.WebhookEvent
	err := r.db.QueryRow(ctx, query, id).Scan(
		&event.ID, &event.VideoID, &event.ChannelID, &event.EventType, &event.Payload,
		&event.SourceIP, &event.UserAgent, &event.Processed, &event.ProcessingStatus,
		&event.ErrorMessage, &event.RetryCount, &event.CreatedAt, &event.UpdatedAt, &event.ProcessedAt,
	)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// Event methods (immutable audit trail)

// CreateEvent inserts a new event into the immutable audit trail.
func (r *Repository) CreateEvent(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO webhook_ingestion.events
		(event_type, channel_id, video_id, raw_xml, event_hash, received_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		event.EventType, event.ChannelID, event.VideoID, event.RawXML, event.EventHash, event.ReceivedAt,
	).Scan(&event.ID, &event.CreatedAt)
	return err
}

// EventExistsByHash checks if an event with the given hash already exists.
func (r *Repository) EventExistsByHash(ctx context.Context, hash string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM webhook_ingestion.events WHERE event_hash = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, hash).Scan(&exists)
	return exists, err
}

// GetEventsByChannelID retrieves events for a specific channel ID with a limit.
func (r *Repository) GetEventsByChannelID(ctx context.Context, channelID string, limit int) ([]models.Event, error) {
	query := `
		SELECT id, event_type, channel_id, video_id, raw_xml, event_hash, received_at, created_at
		FROM webhook_ingestion.events
		WHERE channel_id = $1
		ORDER BY received_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(
			&event.ID, &event.EventType, &event.ChannelID, &event.VideoID,
			&event.RawXML, &event.EventHash, &event.ReceivedAt, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Subscription methods

// CreateSubscription inserts a new subscription into the database.
func (r *Repository) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	query := `
		INSERT INTO webhook_ingestion.subscriptions
		(channel_id, topic_url, callback_url, subscription_status, lease_seconds)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		sub.ChannelID, sub.TopicURL, sub.CallbackURL, sub.SubscriptionStatus, sub.LeaseSeconds,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
	return err
}

// GetSubscriptionByChannelID retrieves a subscription for a specific channel ID.
func (r *Repository) GetSubscriptionByChannelID(ctx context.Context, channelID string) (*models.Subscription, error) {
	query := `
		SELECT id, channel_id, topic_url, callback_url, subscription_status, lease_seconds,
		       lease_expires_at, next_renewal_at, last_renewed_at, renewal_attempts,
		       last_renewal_error, created_at, updated_at
		FROM webhook_ingestion.subscriptions
		WHERE channel_id = $1
	`
	var sub models.Subscription
	var leaseExpiresAt, nextRenewalAt, lastRenewedAt sql.NullTime
	var lastRenewalError sql.NullString

	err := r.db.QueryRow(ctx, query, channelID).Scan(
		&sub.ID, &sub.ChannelID, &sub.TopicURL, &sub.CallbackURL, &sub.SubscriptionStatus,
		&sub.LeaseSeconds, &leaseExpiresAt, &nextRenewalAt, &lastRenewedAt,
		&sub.RenewalAttempts, &lastRenewalError, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if leaseExpiresAt.Valid {
		sub.LeaseExpiresAt = &leaseExpiresAt.Time
	}
	if nextRenewalAt.Valid {
		sub.NextRenewalAt = &nextRenewalAt.Time
	}
	if lastRenewedAt.Valid {
		sub.LastRenewedAt = &lastRenewedAt.Time
	}
	if lastRenewalError.Valid {
		sub.LastRenewalError = &lastRenewalError.String
	}

	return &sub, nil
}

// UpdateSubscriptionStatus updates the status of a subscription.
func (r *Repository) UpdateSubscriptionStatus(ctx context.Context, id uuid.UUID, status models.SubscriptionStatus) error {
	query := `
		UPDATE webhook_ingestion.subscriptions
		SET subscription_status = $2, updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, status, time.Now())
	return err
}

// Utility functions

// ComputeEventHash computes a SHA-256 hash of the raw XML content for deduplication.
func ComputeEventHash(rawXML string) string {
	hash := sha256.Sum256([]byte(rawXML))
	return hex.EncodeToString(hash[:])
}

// Ping checks the database connection health.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

// Transaction support

// BeginTx starts a new database transaction and returns a context with the transaction.
func (r *Repository) BeginTx(ctx context.Context) (context.Context, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, txKey, tx), nil
}

// CommitTx commits the transaction stored in the context.
func (r *Repository) CommitTx(ctx context.Context) error {
	tx, ok := ctx.Value(txKey).(interface{ Commit(context.Context) error })
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Commit(ctx)
}

// RollbackTx rolls back the transaction stored in the context.
func (r *Repository) RollbackTx(ctx context.Context) error {
	tx, ok := ctx.Value(txKey).(interface{ Rollback(context.Context) error })
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Rollback(ctx)
}
