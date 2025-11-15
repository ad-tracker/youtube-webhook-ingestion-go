package repository

import (
	"context"
	"fmt"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionRepository defines operations for managing PubSubHubbub subscriptions.
type SubscriptionRepository interface {
	// Create creates a new subscription.
	Create(ctx context.Context, sub *models.Subscription) error

	// GetByID retrieves a subscription by ID.
	GetByID(ctx context.Context, id int64) (*models.Subscription, error)

	// GetByChannelID retrieves all subscriptions for a channel.
	GetByChannelID(ctx context.Context, channelID string) ([]*models.Subscription, error)

	// Update updates an existing subscription.
	Update(ctx context.Context, sub *models.Subscription) error

	// Delete deletes a subscription by ID.
	Delete(ctx context.Context, id int64) error

	// GetExpiringSoon retrieves subscriptions that will expire soon.
	GetExpiringSoon(ctx context.Context, limit int) ([]*models.Subscription, error)

	// GetByStatus retrieves subscriptions by status.
	GetByStatus(ctx context.Context, status string, limit int) ([]*models.Subscription, error)
}

type subscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewSubscriptionRepository creates a new SubscriptionRepository.
func NewSubscriptionRepository(pool *pgxpool.Pool) SubscriptionRepository {
	return &subscriptionRepository{pool: pool}
}

func (r *subscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	query := `
		INSERT INTO pubsub_subscriptions (
			channel_id, topic_url, callback_url, hub_url, lease_seconds,
			expires_at, status, secret, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		sub.ChannelID,
		sub.TopicURL,
		sub.CallbackURL,
		sub.HubURL,
		sub.LeaseSeconds,
		sub.ExpiresAt,
		sub.Status,
		sub.Secret,
		sub.CreatedAt,
		sub.UpdatedAt,
	).Scan(
		&sub.ID,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create subscription")
	}

	return nil
}

func (r *subscriptionRepository) GetByID(ctx context.Context, id int64) (*models.Subscription, error) {
	query := `
		SELECT id, channel_id, topic_url, callback_url, hub_url, lease_seconds,
		       expires_at, status, secret, last_verified_at, created_at, updated_at
		FROM pubsub_subscriptions
		WHERE id = $1
	`

	sub := &models.Subscription{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&sub.ID,
		&sub.ChannelID,
		&sub.TopicURL,
		&sub.CallbackURL,
		&sub.HubURL,
		&sub.LeaseSeconds,
		&sub.ExpiresAt,
		&sub.Status,
		&sub.Secret,
		&sub.LastVerifiedAt,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "get subscription by id")
	}

	return sub, nil
}

func (r *subscriptionRepository) GetByChannelID(ctx context.Context, channelID string) ([]*models.Subscription, error) {
	query := `
		SELECT id, channel_id, topic_url, callback_url, hub_url, lease_seconds,
		       expires_at, status, secret, last_verified_at, created_at, updated_at
		FROM pubsub_subscriptions
		WHERE channel_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, channelID)
	if err != nil {
		return nil, db.WrapError(err, "get subscriptions by channel id")
	}
	defer rows.Close()

	return scanSubscriptions(rows)
}

func (r *subscriptionRepository) Update(ctx context.Context, sub *models.Subscription) error {
	query := `
		UPDATE pubsub_subscriptions
		SET channel_id = $1,
		    topic_url = $2,
		    callback_url = $3,
		    hub_url = $4,
		    lease_seconds = $5,
		    expires_at = $6,
		    status = $7,
		    secret = $8,
		    last_verified_at = $9
		WHERE id = $10
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		sub.ChannelID,
		sub.TopicURL,
		sub.CallbackURL,
		sub.HubURL,
		sub.LeaseSeconds,
		sub.ExpiresAt,
		sub.Status,
		sub.Secret,
		sub.LastVerifiedAt,
		sub.ID,
	).Scan(&sub.UpdatedAt)

	if err != nil {
		return db.WrapError(err, "update subscription")
	}

	return nil
}

func (r *subscriptionRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM pubsub_subscriptions WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return db.WrapError(err, "delete subscription")
	}

	if result.RowsAffected() == 0 {
		return db.WrapError(pgx.ErrNoRows, "delete subscription")
	}

	return nil
}

func (r *subscriptionRepository) GetExpiringSoon(ctx context.Context, limit int) ([]*models.Subscription, error) {
	query := `
		SELECT id, channel_id, topic_url, callback_url, hub_url, lease_seconds,
		       expires_at, status, secret, last_verified_at, created_at, updated_at
		FROM pubsub_subscriptions
		WHERE status = $1 AND expires_at <= NOW() + INTERVAL '24 hours'
		ORDER BY expires_at ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, models.StatusActive, limit)
	if err != nil {
		return nil, db.WrapError(err, "get expiring subscriptions")
	}
	defer rows.Close()

	return scanSubscriptions(rows)
}

func (r *subscriptionRepository) GetByStatus(ctx context.Context, status string, limit int) ([]*models.Subscription, error) {
	query := `
		SELECT id, channel_id, topic_url, callback_url, hub_url, lease_seconds,
		       expires_at, status, secret, last_verified_at, created_at, updated_at
		FROM pubsub_subscriptions
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, status, limit)
	if err != nil {
		return nil, db.WrapError(err, "get subscriptions by status")
	}
	defer rows.Close()

	return scanSubscriptions(rows)
}

// Helper function to scan multiple subscriptions from query results
func scanSubscriptions(rows pgx.Rows) ([]*models.Subscription, error) {
	var subscriptions []*models.Subscription

	for rows.Next() {
		sub := &models.Subscription{}
		err := rows.Scan(
			&sub.ID,
			&sub.ChannelID,
			&sub.TopicURL,
			&sub.CallbackURL,
			&sub.HubURL,
			&sub.LeaseSeconds,
			&sub.ExpiresAt,
			&sub.Status,
			&sub.Secret,
			&sub.LastVerifiedAt,
			&sub.CreatedAt,
			&sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subscriptions = append(subscriptions, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return subscriptions, nil
}
