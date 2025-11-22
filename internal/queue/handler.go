package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/model"
	"ad-tracker/youtube-webhook-ingestion/internal/service/quota"
	"ad-tracker/youtube-webhook-ingestion/internal/service/youtube"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// EnrichmentHandler handles video enrichment tasks
type EnrichmentHandler struct {
	youtubeClient           *youtube.Client
	quotaManager            *quota.Manager
	enrichmentRepo          repository.EnrichmentRepository
	channelEnrichmentRepo   repository.ChannelEnrichmentRepository
	jobRepo                 repository.EnrichmentJobRepository
	sponsorDetectionRepo    repository.SponsorDetectionRepository
	ollamaClient            interface{} // Will be *ollama.Client, but use interface{} to avoid circular deps
	callbackManager         *CallbackManager
	batchSize               int
	sponsorDetectionEnabled bool
}

// NewEnrichmentHandler creates a new enrichment task handler
func NewEnrichmentHandler(
	youtubeClient *youtube.Client,
	quotaManager *quota.Manager,
	enrichmentRepo repository.EnrichmentRepository,
	channelEnrichmentRepo repository.ChannelEnrichmentRepository,
	jobRepo repository.EnrichmentJobRepository,
	batchSize int,
) *EnrichmentHandler {
	if batchSize <= 0 || batchSize > 50 {
		batchSize = 50
	}

	return &EnrichmentHandler{
		youtubeClient:           youtubeClient,
		quotaManager:            quotaManager,
		enrichmentRepo:          enrichmentRepo,
		channelEnrichmentRepo:   channelEnrichmentRepo,
		jobRepo:                 jobRepo,
		batchSize:               batchSize,
		callbackManager:         NewCallbackManager(),
		sponsorDetectionEnabled: false, // Default to disabled, will be set via SetSponsorDetection
	}
}

// SetCallbackManager sets the callback manager (for dependency injection)
func (h *EnrichmentHandler) SetCallbackManager(cm *CallbackManager) {
	h.callbackManager = cm
}

// SetSponsorDetection configures sponsor detection dependencies
func (h *EnrichmentHandler) SetSponsorDetection(ollamaClient interface{}, sponsorDetectionRepo repository.SponsorDetectionRepository, enabled bool) {
	h.ollamaClient = ollamaClient
	h.sponsorDetectionRepo = sponsorDetectionRepo
	h.sponsorDetectionEnabled = enabled
}

// ProcessTask implements asynq.HandlerFunc
func (h *EnrichmentHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	// Parse payload
	payload, err := UnmarshalEnrichVideoPayload(task.Payload())
	if err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[Handler] Processing video enrichment: video_id=%s, task_id=%s", payload.VideoID, task.ResultWriter().TaskID())

	// Get job from database
	job, err := h.jobRepo.GetJobByAsynqID(ctx, task.ResultWriter().TaskID())
	if err != nil {
		log.Printf("[Handler] Warning: could not find job in database: %v", err)
		// Continue processing even if job tracking fails
	}

	// Mark job as processing
	if job != nil {
		if err := h.jobRepo.MarkJobProcessing(ctx, job.ID); err != nil {
			log.Printf("[Handler] Warning: failed to mark job as processing: %v", err)
		}
	}

	// Check quota availability
	// Official cost: 1 unit per video enrichment (videos.list API call)
	available, quotaInfo, err := h.quotaManager.CheckQuotaAvailable(ctx, 1)
	if err != nil {
		return fmt.Errorf("failed to check quota: %w", err)
	}

	if !available {
		log.Printf("[Handler] Quota exhausted or threshold reached: %d/%d used", quotaInfo.QuotaUsed, quotaInfo.QuotaLimit)
		// Return non-retryable error to avoid hammering the quota
		return fmt.Errorf("quota exhausted: %d/%d used", quotaInfo.QuotaUsed, quotaInfo.QuotaLimit)
	}

	// Fetch video data from YouTube API
	enrichments, quotaCost, err := h.youtubeClient.FetchVideos(ctx, []string{payload.VideoID})
	if err != nil {
		// Record failure in job
		if job != nil {
			h.jobRepo.MarkJobFailed(ctx, job.ID, err.Error(), nil)
		}
		return fmt.Errorf("failed to fetch video from YouTube API: %w", err)
	}

	if len(enrichments) == 0 {
		errMsg := fmt.Sprintf("no data returned for video %s", payload.VideoID)
		if job != nil {
			h.jobRepo.MarkJobFailed(ctx, job.ID, errMsg, nil)
		}
		return fmt.Errorf("no data returned for video %s", payload.VideoID)
	}

	// Store enrichment in database
	enrichment := enrichments[0]
	enrichment.QuotaCost = quotaCost

	if err := h.enrichmentRepo.CreateEnrichment(ctx, enrichment); err != nil {
		// Record failure in job
		if job != nil {
			h.jobRepo.MarkJobFailed(ctx, job.ID, err.Error(), nil)
		}
		return fmt.Errorf("failed to store enrichment: %w", err)
	}

	// Note: Quota tracking is now handled automatically by the YouTube client
	// when FetchVideos() is called (via the QuotaTracker interface),
	// so we don't need to manually record quota usage here to avoid double-counting.

	// Mark job as completed
	if job != nil {
		if err := h.jobRepo.MarkJobCompleted(ctx, job.ID); err != nil {
			log.Printf("[Handler] Warning: failed to mark job as completed: %v", err)
		}
	}

	log.Printf("[Handler] Successfully enriched video: video_id=%s, quota_cost=%d", payload.VideoID, quotaCost)

	// Trigger callbacks after successful enrichment
	if h.callbackManager != nil {
		h.callbackManager.TriggerCallbacks(ctx, payload.VideoID, payload.ChannelID, enrichment)
	}

	return nil
}

// HandleEnrichVideoTask returns an asynq.HandlerFunc for video enrichment
func (h *EnrichmentHandler) HandleEnrichVideoTask() asynq.HandlerFunc {
	return h.ProcessTask
}

// HandleEnrichChannelTask handles channel enrichment tasks
func (h *EnrichmentHandler) HandleEnrichChannelTask() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		// Parse payload
		payload, err := UnmarshalEnrichChannelPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		log.Printf("[Handler] Processing channel enrichment: channel_id=%s, task_id=%s", payload.ChannelID, task.ResultWriter().TaskID())

		// Get job from database
		job, err := h.jobRepo.GetJobByAsynqID(ctx, task.ResultWriter().TaskID())
		if err != nil {
			log.Printf("[Handler] Warning: could not find job in database: %v", err)
			// Continue processing even if job tracking fails
		}

		// Mark job as processing
		if job != nil {
			if err := h.jobRepo.MarkJobProcessing(ctx, job.ID); err != nil {
				log.Printf("[Handler] Warning: failed to mark job as processing: %v", err)
			}
		}

		// Check quota availability
		// Estimate: 1 unit per channel enrichment (channels.list API call)
		available, quotaInfo, err := h.quotaManager.CheckQuotaAvailable(ctx, 1)
		if err != nil {
			return fmt.Errorf("failed to check quota: %w", err)
		}

		if !available {
			log.Printf("[Handler] Quota exhausted or threshold reached: %d/%d used", quotaInfo.QuotaUsed, quotaInfo.QuotaLimit)
			// Return non-retryable error to avoid hammering the quota
			return fmt.Errorf("quota exhausted: %d/%d used", quotaInfo.QuotaUsed, quotaInfo.QuotaLimit)
		}

		// Fetch channel data from YouTube API
		ytEnrichment, err := h.youtubeClient.GetChannelDetails(ctx, payload.ChannelID)
		if err != nil {
			// Record failure in job
			if job != nil {
				h.jobRepo.MarkJobFailed(ctx, job.ID, err.Error(), nil)
			}
			return fmt.Errorf("failed to fetch channel from YouTube API: %w", err)
		}

		// Convert YouTube enrichment to model
		enrichment := mapYouTubeChannelEnrichmentToModel(ytEnrichment)

		// Store enrichment in database
		if err := h.channelEnrichmentRepo.Create(ctx, enrichment); err != nil {
			// Record failure in job
			if job != nil {
				h.jobRepo.MarkJobFailed(ctx, job.ID, err.Error(), nil)
			}
			return fmt.Errorf("failed to store channel enrichment: %w", err)
		}

		// Note: Quota tracking is now handled automatically by the YouTube client
		// when GetChannelDetails() is called (via the QuotaTracker interface),
		// so we don't need to manually record quota usage here to avoid double-counting.

		// Mark job as completed
		if job != nil {
			if err := h.jobRepo.MarkJobCompleted(ctx, job.ID); err != nil {
				log.Printf("[Handler] Warning: failed to mark job as completed: %v", err)
			}
		}

		log.Printf("[Handler] Successfully enriched channel: channel_id=%s", payload.ChannelID)
		return nil
	}
}

// HandleSponsorDetectionTask handles sponsor detection tasks
func (h *EnrichmentHandler) HandleSponsorDetectionTask() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		// Parse payload
		payload, err := UnmarshalSponsorDetectionPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("failed to unmarshal sponsor detection payload: %w", err)
		}

		log.Printf("[Handler] Processing sponsor detection: video_id=%s, detection_job_id=%s, task_id=%s",
			payload.VideoID, payload.DetectionJobID, task.ResultWriter().TaskID())

		// Skip if description is empty
		if payload.Description == "" {
			log.Printf("[Handler] Skipping sponsor detection for video %s: no description", payload.VideoID)

			// Mark job as skipped in database
			if payload.DetectionJobID != "" {
				if jobID, err := parseUUID(payload.DetectionJobID); err == nil {
					h.sponsorDetectionRepo.UpdateDetectionJobStatus(ctx, jobID, "skipped", strPtr("No description available"))
				}
			}

			// Don't retry - this is expected for videos without descriptions
			return nil
		}

		// Parse detection job ID
		detectionJobID, err := parseUUID(payload.DetectionJobID)
		if err != nil {
			return fmt.Errorf("invalid detection job ID: %w", err)
		}

		// Cast ollama client
		ollamaClient, ok := h.ollamaClient.(interface {
			AnalyzeVideoForSponsors(context.Context, string, string) (*models.LLMAnalysisResponse, string, error)
			GetPromptText(string, string) string
		})
		if !ok || ollamaClient == nil {
			errMsg := "ollama client not configured"
			h.sponsorDetectionRepo.UpdateDetectionJobStatus(ctx, detectionJobID, "failed", &errMsg)
			return fmt.Errorf("ollama client not configured")
		}

		// Start timing
		startTime := time.Now()

		// Get the prompt text for storage
		promptText := ollamaClient.GetPromptText(payload.Title, payload.Description)

		// Get or create prompt in database (for deduplication)
		prompt, err := h.sponsorDetectionRepo.GetOrCreatePrompt(ctx, promptText, "v1.0", "Initial sponsor detection prompt")
		if err != nil {
			errMsg := fmt.Sprintf("failed to get/create prompt: %v", err)
			h.sponsorDetectionRepo.UpdateDetectionJobStatus(ctx, detectionJobID, "failed", &errMsg)
			return fmt.Errorf("failed to get/create prompt: %w", err)
		}

		// Call Ollama LLM for sponsor analysis
		analysisResp, rawResponse, err := ollamaClient.AnalyzeVideoForSponsors(ctx, payload.Title, payload.Description)
		if err != nil {
			// Check if it's a timeout or connection error (should retry)
			errMsg := err.Error()
			h.sponsorDetectionRepo.UpdateDetectionJobStatus(ctx, detectionJobID, "failed", &errMsg)
			return fmt.Errorf("failed to analyze video for sponsors: %w", err)
		}

		// Calculate processing time
		processingTimeMs := int(time.Since(startTime).Milliseconds())

		// Save detection results atomically (creates sponsors, video_sponsors, updates job)
		err = h.sponsorDetectionRepo.SaveDetectionResults(
			ctx,
			detectionJobID,
			payload.VideoID,
			&prompt.ID,
			analysisResp.Sponsors,
			rawResponse,
			processingTimeMs,
		)

		if err != nil {
			errMsg := fmt.Sprintf("failed to save detection results: %v", err)
			h.sponsorDetectionRepo.UpdateDetectionJobStatus(ctx, detectionJobID, "failed", &errMsg)
			return fmt.Errorf("failed to save detection results: %w", err)
		}

		log.Printf("[Handler] Successfully completed sponsor detection: video_id=%s, sponsors_detected=%d, processing_time_ms=%d",
			payload.VideoID, len(analysisResp.Sponsors), processingTimeMs)

		return nil
	}
}

// parseUUID is a helper to parse UUID strings
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// Server wraps asynq server for processing tasks
type Server struct {
	asynqServer *asynq.Server
	mux         *asynq.ServeMux
}

// NewServer creates a new task processing server
func NewServer(redisAddr string, concurrency int, handler *EnrichmentHandler) (*Server, error) {
	// Parse Redis URL to extract connection details (host, password, db, TLS)
	redisOpt, err := ParseRedisURL(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	// Build queue map dynamically
	queues := map[string]int{
		"default": 10, // Enrichment queue
	}

	// Add sponsor_detection queue if enabled
	if handler.sponsorDetectionEnabled {
		queues["sponsor_detection"] = 5 // Lower priority than enrichment
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency:    concurrency,
			Queues:         queues,
			StrictPriority: false, // Process all queues fairly
			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("[Server] Task failed: type=%s, error=%v", task.Type(), err)
			}),
		},
	)

	mux := asynq.NewServeMux()

	// Register handlers
	mux.HandleFunc(TypeEnrichVideo, handler.HandleEnrichVideoTask())
	mux.HandleFunc(TypeEnrichChannel, handler.HandleEnrichChannelTask())

	// Register sponsor detection handler if enabled
	if handler.sponsorDetectionEnabled {
		mux.HandleFunc(TypeSponsorDetection, handler.HandleSponsorDetectionTask())
	}

	return &Server{
		asynqServer: srv,
		mux:         mux,
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	log.Println("[Server] Starting task processing server...")
	return s.asynqServer.Start(s.mux)
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	log.Println("[Server] Shutting down task processing server...")
	s.asynqServer.Shutdown()
}

// Run starts the server and blocks until shutdown
func (s *Server) Run() error {
	return s.Start()
}

// mapYouTubeChannelEnrichmentToModel converts youtube.ChannelEnrichment to model.ChannelEnrichment
func mapYouTubeChannelEnrichmentToModel(yt *youtube.ChannelEnrichment) *model.ChannelEnrichment {
	return &model.ChannelEnrichment{
		ChannelID:           yt.ChannelID,
		Description:         strPtrIfNotEmpty(yt.Description),
		CustomURL:           strPtrIfNotEmpty(yt.CustomURL),
		Country:             strPtrIfNotEmpty(yt.Country),
		ThumbnailDefaultURL: strPtrIfNotEmpty(yt.ThumbnailDefaultURL),
		ThumbnailMediumURL:  strPtrIfNotEmpty(yt.ThumbnailMediumURL),
		ThumbnailHighURL:    strPtrIfNotEmpty(yt.ThumbnailHighURL),
		ViewCount:           int64PtrIfNotZero(yt.ViewCount),
		SubscriberCount:     int64PtrIfNotZero(yt.SubscriberCount),
		VideoCount:          int64PtrIfNotZero(yt.VideoCount),
		BannerImageURL:      strPtrIfNotEmpty(yt.BannerImageURL),
		Keywords:            strPtrIfNotEmpty(yt.Keywords),
		TopicCategories:     []string{},
		PublishedAt:         &yt.PublishedAt,
		EnrichedAt:          time.Now(),
		APIResponseEtag:     strPtrIfNotEmpty(yt.APIResponseEtag),
		QuotaCost:           yt.QuotaCost,
		APIPartsRequested:   []string{"snippet", "contentDetails", "statistics", "brandingSettings"},
		RawAPIResponse:      make(map[string]interface{}),
	}
}

func strPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int64PtrIfNotZero(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}
