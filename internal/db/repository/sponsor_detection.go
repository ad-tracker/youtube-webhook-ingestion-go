package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SponsorDetectionRepository defines operations for managing sponsor detection data
type SponsorDetectionRepository interface {
	// Prompt operations
	GetOrCreatePrompt(ctx context.Context, promptText, version, description string) (*models.SponsorDetectionPrompt, error)
	IncrementPromptUsageCount(ctx context.Context, promptID uuid.UUID) error
	GetPromptByID(ctx context.Context, promptID uuid.UUID) (*models.SponsorDetectionPrompt, error)
	ListPrompts(ctx context.Context, limit, offset int) ([]*models.SponsorDetectionPrompt, error)

	// Sponsor operations
	GetSponsorByNormalizedName(ctx context.Context, normalizedName string) (*models.Sponsor, error)
	CreateSponsor(ctx context.Context, sponsor *models.Sponsor) error
	UpdateSponsorLastSeen(ctx context.Context, sponsorID uuid.UUID, timestamp time.Time) error
	IncrementSponsorVideoCount(ctx context.Context, sponsorID uuid.UUID) error
	ListSponsors(ctx context.Context, sortBy string, limit, offset int) ([]*models.Sponsor, error)
	GetSponsorByID(ctx context.Context, sponsorID uuid.UUID) (*models.Sponsor, error)

	// Detection job operations
	CreateDetectionJob(ctx context.Context, job *models.SponsorDetectionJob) error
	UpdateDetectionJobStatus(ctx context.Context, jobID uuid.UUID, status string, errorMsg *string) error
	CompleteDetectionJob(ctx context.Context, jobID uuid.UUID, promptID *uuid.UUID, llmResponse string, processingTimeMs, sponsorCount int) error
	GetDetectionJobsByVideoID(ctx context.Context, videoID string) ([]*models.SponsorDetectionJob, error)
	GetLatestDetectionJobForVideo(ctx context.Context, videoID string) (*models.SponsorDetectionJob, error)
	GetDetectionJobByID(ctx context.Context, jobID uuid.UUID) (*models.SponsorDetectionJob, error)

	// Video-sponsor relationship operations
	CreateVideoSponsor(ctx context.Context, videoSponsor *models.VideoSponsor) error
	GetVideoSponsorsWithDetails(ctx context.Context, videoID string) ([]*models.VideoSponsorDetail, error)
	GetSponsorVideos(ctx context.Context, sponsorID uuid.UUID, limit, offset int) ([]*models.VideoSponsor, error)
	GetVideoSponsorsByJobID(ctx context.Context, jobID uuid.UUID) ([]*models.VideoSponsor, error)
	GetSponsorsByChannelID(ctx context.Context, channelID string, limit, offset int) ([]*models.Sponsor, error)

	// Composite transaction operation
	SaveDetectionResults(ctx context.Context, jobID uuid.UUID, videoID string, promptID *uuid.UUID, llmResults []models.LLMSponsorResult, llmRawResponse string, processingTimeMs int) error
}

type sponsorDetectionRepository struct {
	pool *pgxpool.Pool
}

// NewSponsorDetectionRepository creates a new SponsorDetectionRepository
func NewSponsorDetectionRepository(pool *pgxpool.Pool) SponsorDetectionRepository {
	return &sponsorDetectionRepository{pool: pool}
}

// GetOrCreatePrompt gets an existing prompt by hash or creates a new one
func (r *sponsorDetectionRepository) GetOrCreatePrompt(ctx context.Context, promptText, version, description string) (*models.SponsorDetectionPrompt, error) {
	// Calculate SHA-256 hash of prompt text
	hash := sha256.Sum256([]byte(promptText))
	promptHash := hex.EncodeToString(hash[:])

	// Try to get existing prompt by hash
	query := `
		SELECT id, prompt_text, prompt_hash, version, description, usage_count, created_at
		FROM sponsor_detection_prompts
		WHERE prompt_hash = $1
	`

	var prompt models.SponsorDetectionPrompt
	err := r.pool.QueryRow(ctx, query, promptHash).Scan(
		&prompt.ID,
		&prompt.PromptText,
		&prompt.PromptHash,
		&prompt.Version,
		&prompt.Description,
		&prompt.UsageCount,
		&prompt.CreatedAt,
	)

	if err == nil {
		// Prompt already exists
		return &prompt, nil
	}

	if err != pgx.ErrNoRows {
		return nil, db.WrapError(err, "get prompt by hash")
	}

	// Prompt doesn't exist, create it
	insertQuery := `
		INSERT INTO sponsor_detection_prompts (prompt_text, prompt_hash, version, description, usage_count, created_at)
		VALUES ($1, $2, $3, $4, 0, NOW())
		RETURNING id, prompt_text, prompt_hash, version, description, usage_count, created_at
	`

	var versionPtr, descriptionPtr *string
	if version != "" {
		versionPtr = &version
	}
	if description != "" {
		descriptionPtr = &description
	}

	err = r.pool.QueryRow(ctx, insertQuery, promptText, promptHash, versionPtr, descriptionPtr).Scan(
		&prompt.ID,
		&prompt.PromptText,
		&prompt.PromptHash,
		&prompt.Version,
		&prompt.Description,
		&prompt.UsageCount,
		&prompt.CreatedAt,
	)

	if err != nil {
		return nil, db.WrapError(err, "create prompt")
	}

	return &prompt, nil
}

// IncrementPromptUsageCount increments the usage count for a prompt
func (r *sponsorDetectionRepository) IncrementPromptUsageCount(ctx context.Context, promptID uuid.UUID) error {
	query := `
		UPDATE sponsor_detection_prompts
		SET usage_count = usage_count + 1
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, promptID)
	if err != nil {
		return db.WrapError(err, "increment prompt usage count")
	}

	return nil
}

// GetPromptByID retrieves a prompt by ID
func (r *sponsorDetectionRepository) GetPromptByID(ctx context.Context, promptID uuid.UUID) (*models.SponsorDetectionPrompt, error) {
	query := `
		SELECT id, prompt_text, prompt_hash, version, description, usage_count, created_at
		FROM sponsor_detection_prompts
		WHERE id = $1
	`

	var prompt models.SponsorDetectionPrompt
	err := r.pool.QueryRow(ctx, query, promptID).Scan(
		&prompt.ID,
		&prompt.PromptText,
		&prompt.PromptHash,
		&prompt.Version,
		&prompt.Description,
		&prompt.UsageCount,
		&prompt.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, db.WrapError(err, "get prompt by ID")
	}

	return &prompt, nil
}

// ListPrompts retrieves prompts with pagination
func (r *sponsorDetectionRepository) ListPrompts(ctx context.Context, limit, offset int) ([]*models.SponsorDetectionPrompt, error) {
	query := `
		SELECT id, prompt_text, prompt_hash, version, description, usage_count, created_at
		FROM sponsor_detection_prompts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "list prompts")
	}
	defer rows.Close()

	var prompts []*models.SponsorDetectionPrompt
	for rows.Next() {
		var prompt models.SponsorDetectionPrompt
		err := rows.Scan(
			&prompt.ID,
			&prompt.PromptText,
			&prompt.PromptHash,
			&prompt.Version,
			&prompt.Description,
			&prompt.UsageCount,
			&prompt.CreatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan prompt")
		}
		prompts = append(prompts, &prompt)
	}

	return prompts, nil
}

// GetSponsorByNormalizedName retrieves a sponsor by normalized name (case-insensitive)
func (r *sponsorDetectionRepository) GetSponsorByNormalizedName(ctx context.Context, normalizedName string) (*models.Sponsor, error) {
	query := `
		SELECT id, name, normalized_name, category, website_url, description,
		       first_seen_at, last_seen_at, video_count, created_at, updated_at
		FROM sponsors
		WHERE normalized_name = $1
	`

	var sponsor models.Sponsor
	err := r.pool.QueryRow(ctx, query, normalizedName).Scan(
		&sponsor.ID,
		&sponsor.Name,
		&sponsor.NormalizedName,
		&sponsor.Category,
		&sponsor.WebsiteURL,
		&sponsor.Description,
		&sponsor.FirstSeenAt,
		&sponsor.LastSeenAt,
		&sponsor.VideoCount,
		&sponsor.CreatedAt,
		&sponsor.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, db.WrapError(err, "get sponsor by normalized name")
	}

	return &sponsor, nil
}

// CreateSponsor creates a new sponsor
func (r *sponsorDetectionRepository) CreateSponsor(ctx context.Context, sponsor *models.Sponsor) error {
	query := `
		INSERT INTO sponsors (name, normalized_name, category, website_url, description,
		                      first_seen_at, last_seen_at, video_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, NOW(), NOW())
		RETURNING id, first_seen_at, last_seen_at, video_count, created_at, updated_at
	`

	now := time.Now()
	if sponsor.FirstSeenAt.IsZero() {
		sponsor.FirstSeenAt = now
	}
	if sponsor.LastSeenAt.IsZero() {
		sponsor.LastSeenAt = now
	}

	err := r.pool.QueryRow(ctx, query,
		sponsor.Name,
		sponsor.NormalizedName,
		sponsor.Category,
		sponsor.WebsiteURL,
		sponsor.Description,
		sponsor.FirstSeenAt,
		sponsor.LastSeenAt,
	).Scan(
		&sponsor.ID,
		&sponsor.FirstSeenAt,
		&sponsor.LastSeenAt,
		&sponsor.VideoCount,
		&sponsor.CreatedAt,
		&sponsor.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create sponsor")
	}

	return nil
}

// UpdateSponsorLastSeen updates the last seen timestamp for a sponsor
func (r *sponsorDetectionRepository) UpdateSponsorLastSeen(ctx context.Context, sponsorID uuid.UUID, timestamp time.Time) error {
	query := `
		UPDATE sponsors
		SET last_seen_at = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.pool.Exec(ctx, query, timestamp, sponsorID)
	if err != nil {
		return db.WrapError(err, "update sponsor last seen")
	}

	return nil
}

// IncrementSponsorVideoCount increments the video count for a sponsor
func (r *sponsorDetectionRepository) IncrementSponsorVideoCount(ctx context.Context, sponsorID uuid.UUID) error {
	query := `
		UPDATE sponsors
		SET video_count = video_count + 1, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, sponsorID)
	if err != nil {
		return db.WrapError(err, "increment sponsor video count")
	}

	return nil
}

// ListSponsors retrieves sponsors with pagination and sorting
func (r *sponsorDetectionRepository) ListSponsors(ctx context.Context, sortBy string, limit, offset int) ([]*models.Sponsor, error) {
	// Validate sortBy to prevent SQL injection
	validSorts := map[string]string{
		"video_count": "video_count DESC",
		"name":        "name ASC",
		"last_seen":   "last_seen_at DESC",
		"created":     "created_at DESC",
	}

	orderBy, ok := validSorts[sortBy]
	if !ok {
		orderBy = validSorts["video_count"] // Default sort
	}

	query := fmt.Sprintf(`
		SELECT id, name, normalized_name, category, website_url, description,
		       first_seen_at, last_seen_at, video_count, created_at, updated_at
		FROM sponsors
		ORDER BY %s
		LIMIT $1 OFFSET $2
	`, orderBy)

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "list sponsors")
	}
	defer rows.Close()

	var sponsors []*models.Sponsor
	for rows.Next() {
		var sponsor models.Sponsor
		err := rows.Scan(
			&sponsor.ID,
			&sponsor.Name,
			&sponsor.NormalizedName,
			&sponsor.Category,
			&sponsor.WebsiteURL,
			&sponsor.Description,
			&sponsor.FirstSeenAt,
			&sponsor.LastSeenAt,
			&sponsor.VideoCount,
			&sponsor.CreatedAt,
			&sponsor.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan sponsor")
		}
		sponsors = append(sponsors, &sponsor)
	}

	return sponsors, nil
}

// GetSponsorByID retrieves a sponsor by ID
func (r *sponsorDetectionRepository) GetSponsorByID(ctx context.Context, sponsorID uuid.UUID) (*models.Sponsor, error) {
	query := `
		SELECT id, name, normalized_name, category, website_url, description,
		       first_seen_at, last_seen_at, video_count, created_at, updated_at
		FROM sponsors
		WHERE id = $1
	`

	var sponsor models.Sponsor
	err := r.pool.QueryRow(ctx, query, sponsorID).Scan(
		&sponsor.ID,
		&sponsor.Name,
		&sponsor.NormalizedName,
		&sponsor.Category,
		&sponsor.WebsiteURL,
		&sponsor.Description,
		&sponsor.FirstSeenAt,
		&sponsor.LastSeenAt,
		&sponsor.VideoCount,
		&sponsor.CreatedAt,
		&sponsor.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, db.WrapError(err, "get sponsor by ID")
	}

	return &sponsor, nil
}

// CreateDetectionJob creates a new detection job
func (r *sponsorDetectionRepository) CreateDetectionJob(ctx context.Context, job *models.SponsorDetectionJob) error {
	query := `
		INSERT INTO sponsor_detection_jobs (video_id, prompt_id, llm_model, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, sponsors_detected_count, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		job.VideoID,
		job.PromptID,
		job.LLMModel,
		job.Status,
	).Scan(
		&job.ID,
		&job.SponsorsDetectedCount,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create detection job")
	}

	return nil
}

// UpdateDetectionJobStatus updates the status of a detection job
func (r *sponsorDetectionRepository) UpdateDetectionJobStatus(ctx context.Context, jobID uuid.UUID, status string, errorMsg *string) error {
	query := `
		UPDATE sponsor_detection_jobs
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.pool.Exec(ctx, query, status, errorMsg, jobID)
	if err != nil {
		return db.WrapError(err, "update detection job status")
	}

	return nil
}

// CompleteDetectionJob marks a job as completed with results
func (r *sponsorDetectionRepository) CompleteDetectionJob(ctx context.Context, jobID uuid.UUID, promptID *uuid.UUID, llmResponse string, processingTimeMs, sponsorCount int) error {
	query := `
		UPDATE sponsor_detection_jobs
		SET status = 'completed',
		    prompt_id = $1,
		    llm_response_raw = $2,
		    processing_time_ms = $3,
		    sponsors_detected_count = $4,
		    detected_at = NOW(),
		    updated_at = NOW()
		WHERE id = $5
	`

	_, err := r.pool.Exec(ctx, query, promptID, llmResponse, processingTimeMs, sponsorCount, jobID)
	if err != nil {
		return db.WrapError(err, "complete detection job")
	}

	return nil
}

// GetDetectionJobsByVideoID retrieves all detection jobs for a video
func (r *sponsorDetectionRepository) GetDetectionJobsByVideoID(ctx context.Context, videoID string) ([]*models.SponsorDetectionJob, error) {
	query := `
		SELECT id, video_id, prompt_id, llm_model, llm_response_raw,
		       sponsors_detected_count, processing_time_ms, status, error_message,
		       detected_at, created_at, updated_at
		FROM sponsor_detection_jobs
		WHERE video_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, videoID)
	if err != nil {
		return nil, db.WrapError(err, "get detection jobs by video ID")
	}
	defer rows.Close()

	var jobs []*models.SponsorDetectionJob
	for rows.Next() {
		var job models.SponsorDetectionJob
		err := rows.Scan(
			&job.ID,
			&job.VideoID,
			&job.PromptID,
			&job.LLMModel,
			&job.LLMResponseRaw,
			&job.SponsorsDetectedCount,
			&job.ProcessingTimeMs,
			&job.Status,
			&job.ErrorMessage,
			&job.DetectedAt,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan detection job")
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// GetLatestDetectionJobForVideo retrieves the most recent detection job for a video
func (r *sponsorDetectionRepository) GetLatestDetectionJobForVideo(ctx context.Context, videoID string) (*models.SponsorDetectionJob, error) {
	query := `
		SELECT id, video_id, prompt_id, llm_model, llm_response_raw,
		       sponsors_detected_count, processing_time_ms, status, error_message,
		       detected_at, created_at, updated_at
		FROM sponsor_detection_jobs
		WHERE video_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var job models.SponsorDetectionJob
	err := r.pool.QueryRow(ctx, query, videoID).Scan(
		&job.ID,
		&job.VideoID,
		&job.PromptID,
		&job.LLMModel,
		&job.LLMResponseRaw,
		&job.SponsorsDetectedCount,
		&job.ProcessingTimeMs,
		&job.Status,
		&job.ErrorMessage,
		&job.DetectedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, db.WrapError(err, "get latest detection job for video")
	}

	return &job, nil
}

// GetDetectionJobByID retrieves a detection job by ID
func (r *sponsorDetectionRepository) GetDetectionJobByID(ctx context.Context, jobID uuid.UUID) (*models.SponsorDetectionJob, error) {
	query := `
		SELECT id, video_id, prompt_id, llm_model, llm_response_raw,
		       sponsors_detected_count, processing_time_ms, status, error_message,
		       detected_at, created_at, updated_at
		FROM sponsor_detection_jobs
		WHERE id = $1
	`

	var job models.SponsorDetectionJob
	err := r.pool.QueryRow(ctx, query, jobID).Scan(
		&job.ID,
		&job.VideoID,
		&job.PromptID,
		&job.LLMModel,
		&job.LLMResponseRaw,
		&job.SponsorsDetectedCount,
		&job.ProcessingTimeMs,
		&job.Status,
		&job.ErrorMessage,
		&job.DetectedAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, db.WrapError(err, "get detection job by ID")
	}

	return &job, nil
}

// CreateVideoSponsor creates a video-sponsor relationship
func (r *sponsorDetectionRepository) CreateVideoSponsor(ctx context.Context, videoSponsor *models.VideoSponsor) error {
	query := `
		INSERT INTO video_sponsors (video_id, sponsor_id, detection_job_id, confidence, evidence, detected_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, detected_at, created_at, updated_at
	`

	now := time.Now()
	if videoSponsor.DetectedAt.IsZero() {
		videoSponsor.DetectedAt = now
	}

	err := r.pool.QueryRow(ctx, query,
		videoSponsor.VideoID,
		videoSponsor.SponsorID,
		videoSponsor.DetectionJobID,
		videoSponsor.Confidence,
		videoSponsor.Evidence,
		videoSponsor.DetectedAt,
	).Scan(
		&videoSponsor.ID,
		&videoSponsor.DetectedAt,
		&videoSponsor.CreatedAt,
		&videoSponsor.UpdatedAt,
	)

	if err != nil {
		return db.WrapError(err, "create video sponsor")
	}

	return nil
}

// GetVideoSponsorsWithDetails retrieves all sponsors for a video with sponsor details (JOIN)
func (r *sponsorDetectionRepository) GetVideoSponsorsWithDetails(ctx context.Context, videoID string) ([]*models.VideoSponsorDetail, error) {
	query := `
		SELECT vs.id, vs.video_id, vs.sponsor_id, vs.detection_job_id,
		       vs.confidence, vs.evidence, vs.detected_at, vs.created_at, vs.updated_at,
		       s.name AS sponsor_name, s.category AS sponsor_category
		FROM video_sponsors vs
		JOIN sponsors s ON vs.sponsor_id = s.id
		WHERE vs.video_id = $1
		ORDER BY vs.confidence DESC, vs.detected_at DESC
	`

	rows, err := r.pool.Query(ctx, query, videoID)
	if err != nil {
		return nil, db.WrapError(err, "get video sponsors with details")
	}
	defer rows.Close()

	var details []*models.VideoSponsorDetail
	for rows.Next() {
		var detail models.VideoSponsorDetail
		err := rows.Scan(
			&detail.ID,
			&detail.VideoID,
			&detail.SponsorID,
			&detail.DetectionJobID,
			&detail.Confidence,
			&detail.Evidence,
			&detail.DetectedAt,
			&detail.CreatedAt,
			&detail.UpdatedAt,
			&detail.SponsorName,
			&detail.SponsorCategory,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan video sponsor detail")
		}
		details = append(details, &detail)
	}

	return details, nil
}

// GetSponsorVideos retrieves all videos for a sponsor
func (r *sponsorDetectionRepository) GetSponsorVideos(ctx context.Context, sponsorID uuid.UUID, limit, offset int) ([]*models.VideoSponsor, error) {
	query := `
		SELECT id, video_id, sponsor_id, detection_job_id, confidence, evidence,
		       detected_at, created_at, updated_at
		FROM video_sponsors
		WHERE sponsor_id = $1
		ORDER BY detected_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, sponsorID, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "get sponsor videos")
	}
	defer rows.Close()

	var videoSponsors []*models.VideoSponsor
	for rows.Next() {
		var vs models.VideoSponsor
		err := rows.Scan(
			&vs.ID,
			&vs.VideoID,
			&vs.SponsorID,
			&vs.DetectionJobID,
			&vs.Confidence,
			&vs.Evidence,
			&vs.DetectedAt,
			&vs.CreatedAt,
			&vs.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan video sponsor")
		}
		videoSponsors = append(videoSponsors, &vs)
	}

	return videoSponsors, nil
}

// GetVideoSponsorsByJobID retrieves all video-sponsor relationships for a detection job
func (r *sponsorDetectionRepository) GetVideoSponsorsByJobID(ctx context.Context, jobID uuid.UUID) ([]*models.VideoSponsor, error) {
	query := `
		SELECT id, video_id, sponsor_id, detection_job_id, confidence, evidence,
		       detected_at, created_at, updated_at
		FROM video_sponsors
		WHERE detection_job_id = $1
		ORDER BY confidence DESC
	`

	rows, err := r.pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, db.WrapError(err, "get video sponsors by job ID")
	}
	defer rows.Close()

	var videoSponsors []*models.VideoSponsor
	for rows.Next() {
		var vs models.VideoSponsor
		err := rows.Scan(
			&vs.ID,
			&vs.VideoID,
			&vs.SponsorID,
			&vs.DetectionJobID,
			&vs.Confidence,
			&vs.Evidence,
			&vs.DetectedAt,
			&vs.CreatedAt,
			&vs.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan video sponsor")
		}
		videoSponsors = append(videoSponsors, &vs)
	}

	return videoSponsors, nil
}

// SaveDetectionResults atomically saves all detection results in a transaction
func (r *sponsorDetectionRepository) SaveDetectionResults(
	ctx context.Context,
	jobID uuid.UUID,
	videoID string,
	promptID *uuid.UUID,
	llmResults []models.LLMSponsorResult,
	llmRawResponse string,
	processingTimeMs int,
) error {
	// Start transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return db.WrapError(err, "begin transaction")
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	sponsorCount := len(llmResults)

	// Process each LLM result
	for _, result := range llmResults {
		// Normalize sponsor name (lowercase for matching)
		normalizedName := strings.ToLower(strings.TrimSpace(result.Name))

		// Get or create sponsor
		var sponsorID uuid.UUID
		var existingSponsor models.Sponsor

		getSponsorQuery := `
			SELECT id, name, normalized_name, category, website_url, description,
			       first_seen_at, last_seen_at, video_count, created_at, updated_at
			FROM sponsors
			WHERE normalized_name = $1
		`

		err = tx.QueryRow(ctx, getSponsorQuery, normalizedName).Scan(
			&existingSponsor.ID,
			&existingSponsor.Name,
			&existingSponsor.NormalizedName,
			&existingSponsor.Category,
			&existingSponsor.WebsiteURL,
			&existingSponsor.Description,
			&existingSponsor.FirstSeenAt,
			&existingSponsor.LastSeenAt,
			&existingSponsor.VideoCount,
			&existingSponsor.CreatedAt,
			&existingSponsor.UpdatedAt,
		)

		if err == pgx.ErrNoRows {
			// Sponsor doesn't exist, create it
			createSponsorQuery := `
				INSERT INTO sponsors (name, normalized_name, first_seen_at, last_seen_at, video_count, created_at, updated_at)
				VALUES ($1, $2, $3, $4, 0, NOW(), NOW())
				RETURNING id
			`

			err = tx.QueryRow(ctx, createSponsorQuery, result.Name, normalizedName, now, now).Scan(&sponsorID)
			if err != nil {
				return db.WrapError(err, "create sponsor in transaction")
			}
		} else if err != nil {
			return db.WrapError(err, "get sponsor in transaction")
		} else {
			// Sponsor exists, use its ID and update last_seen_at
			sponsorID = existingSponsor.ID

			updateSponsorQuery := `
				UPDATE sponsors
				SET last_seen_at = $1, updated_at = NOW()
				WHERE id = $2
			`

			_, err = tx.Exec(ctx, updateSponsorQuery, now, sponsorID)
			if err != nil {
				return db.WrapError(err, "update sponsor last seen in transaction")
			}
		}

		// Create video_sponsor relationship
		createVideoSponsorQuery := `
			INSERT INTO video_sponsors (video_id, sponsor_id, detection_job_id, confidence, evidence, detected_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			ON CONFLICT (video_id, sponsor_id, detection_job_id) DO NOTHING
		`

		_, err = tx.Exec(ctx, createVideoSponsorQuery,
			videoID,
			sponsorID,
			jobID,
			result.Confidence,
			result.Evidence,
			now,
		)

		if err != nil {
			return db.WrapError(err, "create video sponsor in transaction")
		}

		// Increment sponsor video count
		// Note: This is a simplified approach. In production, you might want to use a
		// periodic job to recalculate video_count to ensure accuracy.
		incrementCountQuery := `
			UPDATE sponsors
			SET video_count = (
				SELECT COUNT(DISTINCT video_id)
				FROM video_sponsors
				WHERE sponsor_id = $1
			)
			WHERE id = $1
		`

		_, err = tx.Exec(ctx, incrementCountQuery, sponsorID)
		if err != nil {
			return db.WrapError(err, "increment sponsor video count in transaction")
		}
	}

	// Update detection job as completed
	updateJobQuery := `
		UPDATE sponsor_detection_jobs
		SET status = 'completed',
		    prompt_id = $1,
		    llm_response_raw = $2,
		    processing_time_ms = $3,
		    sponsors_detected_count = $4,
		    detected_at = $5,
		    updated_at = NOW()
		WHERE id = $6
	`

	_, err = tx.Exec(ctx, updateJobQuery,
		promptID,
		llmRawResponse,
		processingTimeMs,
		sponsorCount,
		now,
		jobID,
	)

	if err != nil {
		return db.WrapError(err, "update detection job in transaction")
	}

	// Increment prompt usage count if promptID is provided
	if promptID != nil {
		incrementPromptQuery := `
			UPDATE sponsor_detection_prompts
			SET usage_count = usage_count + 1
			WHERE id = $1
		`

		_, err = tx.Exec(ctx, incrementPromptQuery, *promptID)
		if err != nil {
			return db.WrapError(err, "increment prompt usage in transaction")
		}
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		return db.WrapError(err, "commit transaction")
	}

	return nil
}

// GetSponsorsByChannelID retrieves all sponsors that appear in videos from a specific channel
func (r *sponsorDetectionRepository) GetSponsorsByChannelID(ctx context.Context, channelID string, limit, offset int) ([]*models.Sponsor, error) {
	query := `
		SELECT DISTINCT s.id, s.name, s.normalized_name, s.category, s.website_url, s.description,
		       s.first_seen_at, s.last_seen_at, s.video_count, s.created_at, s.updated_at
		FROM sponsors s
		JOIN video_sponsors vs ON s.id = vs.sponsor_id
		JOIN videos v ON vs.video_id = v.video_id
		WHERE v.channel_id = $1
		ORDER BY s.video_count DESC, s.name ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, channelID, limit, offset)
	if err != nil {
		return nil, db.WrapError(err, "get sponsors by channel ID")
	}
	defer rows.Close()

	var sponsors []*models.Sponsor
	for rows.Next() {
		var sponsor models.Sponsor
		err := rows.Scan(
			&sponsor.ID,
			&sponsor.Name,
			&sponsor.NormalizedName,
			&sponsor.Category,
			&sponsor.WebsiteURL,
			&sponsor.Description,
			&sponsor.FirstSeenAt,
			&sponsor.LastSeenAt,
			&sponsor.VideoCount,
			&sponsor.CreatedAt,
			&sponsor.UpdatedAt,
		)
		if err != nil {
			return nil, db.WrapError(err, "scan sponsor")
		}
		sponsors = append(sponsors, &sponsor)
	}

	return sponsors, nil
}
