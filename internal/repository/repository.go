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

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// WebhookEvent methods

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

func (r *Repository) EventExistsByHash(ctx context.Context, hash string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM webhook_ingestion.events WHERE event_hash = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, hash).Scan(&exists)
	return exists, err
}

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

func ComputeEventHash(rawXML string) string {
	hash := sha256.Sum256([]byte(rawXML))
	return hex.EncodeToString(hash[:])
}

// Health check
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

// Transaction support
func (r *Repository) BeginTx(ctx context.Context) (context.Context, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, "tx", tx), nil
}

func (r *Repository) CommitTx(ctx context.Context) error {
	tx, ok := ctx.Value("tx").(interface{ Commit(context.Context) error })
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Commit(ctx)
}

func (r *Repository) RollbackTx(ctx context.Context) error {
	tx, ok := ctx.Value("tx").(interface{ Rollback(context.Context) error })
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	return tx.Rollback(ctx)
}
