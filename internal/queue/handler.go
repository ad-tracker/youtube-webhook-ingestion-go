package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/model"
	"ad-tracker/youtube-webhook-ingestion/internal/service/quota"
	"ad-tracker/youtube-webhook-ingestion/internal/service/youtube"

	"github.com/hibiken/asynq"
)

// EnrichmentHandler handles video enrichment tasks
type EnrichmentHandler struct {
	youtubeClient        *youtube.Client
	quotaManager         *quota.Manager
	enrichmentRepo       repository.EnrichmentRepository
	channelEnrichmentRepo repository.ChannelEnrichmentRepository
	jobRepo              repository.EnrichmentJobRepository
	batchSize            int
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
		youtubeClient:        youtubeClient,
		quotaManager:         quotaManager,
		enrichmentRepo:       enrichmentRepo,
		channelEnrichmentRepo: channelEnrichmentRepo,
		jobRepo:              jobRepo,
		batchSize:            batchSize,
	}
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
	// Estimate: 5 units per video enrichment
	available, quotaInfo, err := h.quotaManager.CheckQuotaAvailable(ctx, 5)
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

	// Record quota usage
	if err := h.quotaManager.RecordQuotaUsage(ctx, quotaCost, "videos_list"); err != nil {
		log.Printf("[Handler] Warning: failed to record quota usage: %v", err)
		// Don't fail the task for quota tracking errors
	}

	// Mark job as completed
	if job != nil {
		if err := h.jobRepo.MarkJobCompleted(ctx, job.ID); err != nil {
			log.Printf("[Handler] Warning: failed to mark job as completed: %v", err)
		}
	}

	log.Printf("[Handler] Successfully enriched video: video_id=%s, quota_cost=%d", payload.VideoID, quotaCost)
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

		// Record quota usage (1 unit for channels.list)
		if err := h.quotaManager.RecordQuotaUsage(ctx, 1, "channels_list"); err != nil {
			log.Printf("[Handler] Warning: failed to record quota usage: %v", err)
			// Don't fail the task for quota tracking errors
		}

		// Mark job as completed
		if job != nil {
			if err := h.jobRepo.MarkJobCompleted(ctx, job.ID); err != nil {
				log.Printf("[Handler] Warning: failed to mark job as completed: %v", err)
			}
		}

		log.Printf("[Handler] Successfully enriched channel: channel_id=%s, quota_cost=1", payload.ChannelID)
		return nil
	}
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

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 10,
			},
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
