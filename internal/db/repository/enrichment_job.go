package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobFilters contains filters for listing enrichment jobs
type JobFilters struct {
	Status string
	Limit  int
	Offset int
}

// EnrichmentJobRepository defines operations for managing enrichment jobs
type EnrichmentJobRepository interface {
	// CreateJob creates a new enrichment job
	CreateJob(ctx context.Context, job *model.EnrichmentJob) error

	// GetJobByID retrieves a job by ID
	GetJobByID(ctx context.Context, id int64) (*model.EnrichmentJob, error)

	// GetJobByAsynqID retrieves a job by asynq task ID
	GetJobByAsynqID(ctx context.Context, asynqTaskID string) (*model.EnrichmentJob, error)

	// UpdateJobStatus updates job status
	UpdateJobStatus(ctx context.Context, id int64, status string, errorMsg *string) error

	// MarkJobProcessing marks a job as processing (sets started_at)
	MarkJobProcessing(ctx context.Context, id int64) error

	// MarkJobCompleted marks a job as completed
	MarkJobCompleted(ctx context.Context, id int64) error

	// MarkJobFailed marks a job as failed with error message
	MarkJobFailed(ctx context.Context, id int64, errorMsg string, stackTrace *string) error

	// IncrementAttempts increments job attempt count
	IncrementAttempts(ctx context.Context, id int64) error

	// GetPendingJobs retrieves pending jobs
	GetPendingJobs(ctx context.Context, limit int) ([]*model.EnrichmentJob, error)

	// GetJobsByStatus retrieves jobs by status
	GetJobsByStatus(ctx context.Context, status string, limit int) ([]*model.EnrichmentJob, error)

	// ListJobs retrieves jobs with optional filters, returns jobs and total count
	ListJobs(ctx context.Context, filters JobFilters) ([]*model.EnrichmentJob, int, error)

	// GetJobStats retrieves job statistics
	GetJobStats(ctx context.Context) (map[string]int, error)
}

type enrichmentJobRepository struct {
	pool *pgxpool.Pool
}

// NewEnrichmentJobRepository creates a new EnrichmentJobRepository
func NewEnrichmentJobRepository(pool *pgxpool.Pool) EnrichmentJobRepository {
	return &enrichmentJobRepository{pool: pool}
}

func (r *enrichmentJobRepository) CreateJob(ctx context.Context, job *model.EnrichmentJob) error {
	query := `
		INSERT INTO enrichment_jobs (
			asynq_task_id, job_type, video_id, status, priority,
			scheduled_at, attempts, max_attempts, metadata,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	metadataJSON, _ := json.Marshal(job.Metadata)
	now := time.Now()

	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = now
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 3
	}

	err := r.pool.QueryRow(ctx, query,
		job.AsynqTaskID,
		job.JobType,
		job.VideoID,
		job.Status,
		job.Priority,
		job.ScheduledAt,
		job.Attempts,
		job.MaxAttempts,
		metadataJSON,
		now,
		now,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return db.WrapError(err, "create enrichment job")
	}

	return nil
}

func (r *enrichmentJobRepository) GetJobByID(ctx context.Context, id int64) (*model.EnrichmentJob, error) {
	query := `
		SELECT id, asynq_task_id, job_type, video_id, status, priority,
		       scheduled_at, started_at, completed_at,
		       attempts, max_attempts, next_retry_at,
		       error_message, error_stack_trace, metadata,
		       created_at, updated_at
		FROM enrichment_jobs
		WHERE id = $1
	`

	job := &model.EnrichmentJob{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.AsynqTaskID, &job.JobType, &job.VideoID,
		&job.Status, &job.Priority,
		&job.ScheduledAt, &job.StartedAt, &job.CompletedAt,
		&job.Attempts, &job.MaxAttempts, &job.NextRetryAt,
		&job.ErrorMessage, &job.ErrorStackTrace, &metadataJSON,
		&job.CreatedAt, &job.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, db.WrapError(err, "get job by ID")
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &job.Metadata)
	}

	return job, nil
}

func (r *enrichmentJobRepository) GetJobByAsynqID(ctx context.Context, asynqTaskID string) (*model.EnrichmentJob, error) {
	query := `
		SELECT id, asynq_task_id, job_type, video_id, status, priority,
		       scheduled_at, started_at, completed_at,
		       attempts, max_attempts, next_retry_at,
		       error_message, error_stack_trace, metadata,
		       created_at, updated_at
		FROM enrichment_jobs
		WHERE asynq_task_id = $1
	`

	job := &model.EnrichmentJob{}
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, asynqTaskID).Scan(
		&job.ID, &job.AsynqTaskID, &job.JobType, &job.VideoID,
		&job.Status, &job.Priority,
		&job.ScheduledAt, &job.StartedAt, &job.CompletedAt,
		&job.Attempts, &job.MaxAttempts, &job.NextRetryAt,
		&job.ErrorMessage, &job.ErrorStackTrace, &metadataJSON,
		&job.CreatedAt, &job.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, db.WrapError(err, "get job by asynq ID")
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &job.Metadata)
	}

	return job, nil
}

func (r *enrichmentJobRepository) UpdateJobStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
	query := `
		UPDATE enrichment_jobs
		SET status = $2, error_message = $3, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, status, errorMsg)
	if err != nil {
		return db.WrapError(err, "update job status")
	}

	return nil
}

func (r *enrichmentJobRepository) MarkJobProcessing(ctx context.Context, id int64) error {
	query := `
		UPDATE enrichment_jobs
		SET status = 'processing', started_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return db.WrapError(err, "mark job processing")
	}

	return nil
}

func (r *enrichmentJobRepository) MarkJobCompleted(ctx context.Context, id int64) error {
	query := `
		UPDATE enrichment_jobs
		SET status = 'completed', completed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return db.WrapError(err, "mark job completed")
	}

	return nil
}

func (r *enrichmentJobRepository) MarkJobFailed(ctx context.Context, id int64, errorMsg string, stackTrace *string) error {
	query := `
		UPDATE enrichment_jobs
		SET status = 'failed',
		    completed_at = NOW(),
		    error_message = $2,
		    error_stack_trace = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, errorMsg, stackTrace)
	if err != nil {
		return db.WrapError(err, "mark job failed")
	}

	return nil
}

func (r *enrichmentJobRepository) IncrementAttempts(ctx context.Context, id int64) error {
	query := `
		UPDATE enrichment_jobs
		SET attempts = attempts + 1, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return db.WrapError(err, "increment attempts")
	}

	return nil
}

func (r *enrichmentJobRepository) GetPendingJobs(ctx context.Context, limit int) ([]*model.EnrichmentJob, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, asynq_task_id, job_type, video_id, status, priority,
		       scheduled_at, attempts, max_attempts,
		       created_at, updated_at
		FROM enrichment_jobs
		WHERE status = 'pending'
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, db.WrapError(err, "get pending jobs")
	}
	defer rows.Close()

	var jobs []*model.EnrichmentJob
	for rows.Next() {
		job := &model.EnrichmentJob{}
		err := rows.Scan(
			&job.ID, &job.AsynqTaskID, &job.JobType, &job.VideoID,
			&job.Status, &job.Priority, &job.ScheduledAt,
			&job.Attempts, &job.MaxAttempts,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan pending job")
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (r *enrichmentJobRepository) GetJobsByStatus(ctx context.Context, status string, limit int) ([]*model.EnrichmentJob, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, asynq_task_id, job_type, video_id, status, priority,
		       scheduled_at, started_at, completed_at,
		       attempts, max_attempts,
		       error_message,
		       created_at, updated_at
		FROM enrichment_jobs
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, status, limit)
	if err != nil {
		return nil, db.WrapError(err, "get jobs by status")
	}
	defer rows.Close()

	var jobs []*model.EnrichmentJob
	for rows.Next() {
		job := &model.EnrichmentJob{}
		err := rows.Scan(
			&job.ID, &job.AsynqTaskID, &job.JobType, &job.VideoID,
			&job.Status, &job.Priority,
			&job.ScheduledAt, &job.StartedAt, &job.CompletedAt,
			&job.Attempts, &job.MaxAttempts,
			&job.ErrorMessage,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan job")
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (r *enrichmentJobRepository) ListJobs(ctx context.Context, filters JobFilters) ([]*model.EnrichmentJob, int, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	// Build WHERE clause for both queries
	whereClause := ""
	countArgs := make([]interface{}, 0)
	if filters.Status != "" {
		whereClause = " WHERE status = $1"
		countArgs = append(countArgs, filters.Status)
	}

	// Get total count
	countQuery := "SELECT COUNT(*)::int FROM enrichment_jobs" + whereClause
	var total int
	err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, db.WrapError(err, "count jobs")
	}

	// Build main query
	query := `
		SELECT id, asynq_task_id, job_type, video_id, status, priority,
		       scheduled_at, started_at, completed_at,
		       attempts, max_attempts, next_retry_at,
		       error_message, error_stack_trace, metadata,
		       created_at, updated_at
		FROM enrichment_jobs
	` + whereClause

	// Order by created_at DESC
	query += " ORDER BY created_at DESC"

	// Add limit and offset
	args := make([]interface{}, 0)
	args = append(args, countArgs...)
	argIndex := len(args) + 1
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, db.WrapError(err, "list jobs")
	}
	defer rows.Close()

	var jobs []*model.EnrichmentJob
	for rows.Next() {
		job := &model.EnrichmentJob{}
		var metadataJSON []byte

		err := rows.Scan(
			&job.ID, &job.AsynqTaskID, &job.JobType, &job.VideoID,
			&job.Status, &job.Priority,
			&job.ScheduledAt, &job.StartedAt, &job.CompletedAt,
			&job.Attempts, &job.MaxAttempts, &job.NextRetryAt,
			&job.ErrorMessage, &job.ErrorStackTrace, &metadataJSON,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, 0, db.WrapError(err, "scan job")
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &job.Metadata)
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, db.WrapError(err, "iterate jobs")
	}

	return jobs, total, nil
}

func (r *enrichmentJobRepository) GetJobStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*)::int as count
		FROM enrichment_jobs
		WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'
		GROUP BY status
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, db.WrapError(err, "get job stats")
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, db.WrapError(err, "scan job stats")
		}
		stats[status] = count
	}

	return stats, nil
}
