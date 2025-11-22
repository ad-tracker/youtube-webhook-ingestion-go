package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/model"

	"github.com/hibiken/asynq"
)

// Client wraps asynq client for enqueueing tasks
type Client struct {
	asynqClient *asynq.Client
	jobRepo     repository.EnrichmentJobRepository
}

// NewClient creates a new queue client
func NewClient(redisAddr string, jobRepo repository.EnrichmentJobRepository) (*Client, error) {
	// Parse Redis URL to extract connection details (host, password, db, TLS)
	redisOpt, err := ParseRedisURL(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	asynqClient := asynq.NewClient(redisOpt)

	return &Client{
		asynqClient: asynqClient,
		jobRepo:     jobRepo,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.asynqClient.Close()
}

// EnqueueVideoEnrichment enqueues a video enrichment task
func (c *Client) EnqueueVideoEnrichment(ctx context.Context, videoID, channelID string, priority int) error {
	// Create payload
	payload, err := NewEnrichVideoTask(videoID, channelID, priority, map[string]interface{}{
		"source":      "webhook",
		"enqueued_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to create task payload: %w", err)
	}

	// Marshal payload
	payloadBytes, err := payload.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create asynq task
	task := asynq.NewTask(TypeEnrichVideo, payloadBytes)

	// Enqueue task
	info, err := c.asynqClient.Enqueue(task,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("[Queue] Enqueued video enrichment: video_id=%s, task_id=%s", videoID, info.ID)

	// Record job in database for tracking
	job := &model.EnrichmentJob{
		AsynqTaskID: strPtr(info.ID),
		JobType:     TypeEnrichVideo,
		VideoID:     videoID,
		Status:      "pending",
		Priority:    priority,
		ScheduledAt: time.Now(),
		MaxAttempts: 3,
		Metadata: map[string]interface{}{
			"channel_id": channelID,
			"source":     "webhook",
		},
	}

	if err := c.jobRepo.CreateJob(ctx, job); err != nil {
		// Log but don't fail - the asynq task is already queued
		log.Printf("[Queue] Warning: failed to record job in database: %v", err)
	}

	return nil
}

// EnqueueVideoEnrichmentBatch enqueues multiple video enrichment tasks
func (c *Client) EnqueueVideoEnrichmentBatch(ctx context.Context, videoIDs []string, channelID string, priority int) error {
	for _, videoID := range videoIDs {
		if err := c.EnqueueVideoEnrichment(ctx, videoID, channelID, priority); err != nil {
			log.Printf("[Queue] Failed to enqueue video %s: %v", videoID, err)
			// Continue with other videos
		}
	}

	log.Printf("[Queue] Enqueued %d video enrichment tasks", len(videoIDs))
	return nil
}

// EnqueueChannelEnrichment enqueues a channel enrichment task
func (c *Client) EnqueueChannelEnrichment(ctx context.Context, channelID string) error {
	// Create payload
	payload, err := NewEnrichChannelTask(channelID, 0, map[string]interface{}{
		"source":      "manual",
		"enqueued_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to create task payload: %w", err)
	}

	// Marshal payload
	payloadBytes, err := payload.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create asynq task
	task := asynq.NewTask(TypeEnrichChannel, payloadBytes)

	// Enqueue task
	info, err := c.asynqClient.Enqueue(task,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Queue("default"),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("[Queue] Enqueued channel enrichment: channel_id=%s, task_id=%s", channelID, info.ID)

	// Note: We don't record channel enrichment jobs in enrichment_jobs table
	// because video_id is a required foreign key field. Channel enrichment
	// jobs are tracked through asynq only.
	// TODO: Consider adding a migration to make video_id nullable or add channel_id field

	return nil
}

// EnqueueSponsorDetection enqueues a sponsor detection task
func (c *Client) EnqueueSponsorDetection(ctx context.Context, videoID, title, description, detectionJobID string, priority int) error {
	// Create payload
	payload, err := NewSponsorDetectionTask(videoID, title, description, detectionJobID, map[string]interface{}{
		"source":      "enrichment_callback",
		"enqueued_at": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to create sponsor detection task payload: %w", err)
	}

	// Marshal payload
	payloadBytes, err := payload.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal sponsor detection payload: %w", err)
	}

	// Create asynq task
	task := asynq.NewTask(TypeSponsorDetection, payloadBytes)

	// Enqueue task to sponsor_detection queue (separate from enrichment)
	info, err := c.asynqClient.Enqueue(task,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Queue("sponsor_detection"),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue sponsor detection task: %w", err)
	}

	log.Printf("[Queue] Enqueued sponsor detection: video_id=%s, detection_job_id=%s, task_id=%s", videoID, detectionJobID, info.ID)

	// Record job in enrichment_jobs table for tracking
	// Note: We reuse the enrichment_jobs table since it supports multiple job types
	job := &model.EnrichmentJob{
		AsynqTaskID: strPtr(info.ID),
		JobType:     TypeSponsorDetection,
		VideoID:     videoID,
		Status:      "pending",
		Priority:    priority,
		ScheduledAt: time.Now(),
		MaxAttempts: 3,
		Metadata: map[string]interface{}{
			"detection_job_id": detectionJobID,
			"source":           "enrichment_callback",
		},
	}

	if err := c.jobRepo.CreateJob(ctx, job); err != nil {
		// Log but don't fail - the asynq task is already queued
		log.Printf("[Queue] Warning: failed to record sponsor detection job in database: %v", err)
	}

	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
