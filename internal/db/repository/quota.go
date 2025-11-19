package repository

import (
	"context"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// QuotaRepository defines operations for managing API quota usage
type QuotaRepository interface {
	// GetTodaysQuota retrieves today's quota usage
	GetTodaysQuota(ctx context.Context) (*model.QuotaInfo, error)

	// IncrementQuota increments today's quota usage
	IncrementQuota(ctx context.Context, quotaCost int, operationType string) error

	// GetQuotaForDate retrieves quota usage for a specific date
	GetQuotaForDate(ctx context.Context, date time.Time) (*model.APIQuotaUsage, error)

	// GetQuotaHistory retrieves quota usage history
	GetQuotaHistory(ctx context.Context, days int) ([]*model.APIQuotaUsage, error)

	// CheckQuotaAvailable checks if enough quota is available
	CheckQuotaAvailable(ctx context.Context, requiredQuota int) (bool, error)
}

type quotaRepository struct {
	pool *pgxpool.Pool
}

// NewQuotaRepository creates a new QuotaRepository
func NewQuotaRepository(pool *pgxpool.Pool) QuotaRepository {
	return &quotaRepository{pool: pool}
}

func (r *quotaRepository) GetTodaysQuota(ctx context.Context) (*model.QuotaInfo, error) {
	query := `SELECT * FROM get_todays_quota_usage()`

	info := &model.QuotaInfo{}
	err := r.pool.QueryRow(ctx, query).Scan(
		&info.QuotaUsed,
		&info.QuotaLimit,
		&info.QuotaRemaining,
		&info.OperationsCount,
	)

	if err != nil {
		return nil, db.WrapError(err, "get todays quota")
	}

	return info, nil
}

func (r *quotaRepository) IncrementQuota(ctx context.Context, quotaCost int, operationType string) error {
	if operationType == "" {
		operationType = "other"
	}

	query := `SELECT increment_quota_usage($1, $2)`
	_, err := r.pool.Exec(ctx, query, quotaCost, operationType)
	if err != nil {
		return db.WrapError(err, "increment quota")
	}

	return nil
}

func (r *quotaRepository) GetQuotaForDate(ctx context.Context, date time.Time) (*model.APIQuotaUsage, error) {
	query := `
		SELECT id, date, quota_used, quota_limit, operations_count,
		       videos_list_calls, channels_list_calls, other_calls,
		       created_at, updated_at
		FROM api_quota_usage
		WHERE date = $1
	`

	usage := &model.APIQuotaUsage{}
	err := r.pool.QueryRow(ctx, query, date.Format("2006-01-02")).Scan(
		&usage.ID,
		&usage.Date,
		&usage.QuotaUsed,
		&usage.QuotaLimit,
		&usage.OperationsCount,
		&usage.VideosListCalls,
		&usage.ChannelsListCalls,
		&usage.OtherCalls,
		&usage.CreatedAt,
		&usage.UpdatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "get quota for date")
	}

	return usage, nil
}

func (r *quotaRepository) GetQuotaHistory(ctx context.Context, days int) ([]*model.APIQuotaUsage, error) {
	if days <= 0 {
		days = 7
	}

	query := `
		SELECT id, date, quota_used, quota_limit, operations_count,
		       videos_list_calls, channels_list_calls, other_calls,
		       created_at, updated_at
		FROM api_quota_usage
		WHERE date >= CURRENT_DATE - INTERVAL '1 day' * $1
		ORDER BY date DESC
	`

	rows, err := r.pool.Query(ctx, query, days)
	if err != nil {
		return nil, db.WrapError(err, "get quota history")
	}
	defer rows.Close()

	var history []*model.APIQuotaUsage
	for rows.Next() {
		usage := &model.APIQuotaUsage{}
		err := rows.Scan(
			&usage.ID,
			&usage.Date,
			&usage.QuotaUsed,
			&usage.QuotaLimit,
			&usage.OperationsCount,
			&usage.VideosListCalls,
			&usage.ChannelsListCalls,
			&usage.OtherCalls,
			&usage.CreatedAt,
			&usage.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan quota history")
		}
		history = append(history, usage)
	}

	return history, nil
}

func (r *quotaRepository) CheckQuotaAvailable(ctx context.Context, requiredQuota int) (bool, error) {
	info, err := r.GetTodaysQuota(ctx)
	if err != nil {
		return false, err
	}

	return info.QuotaRemaining >= requiredQuota, nil
}
