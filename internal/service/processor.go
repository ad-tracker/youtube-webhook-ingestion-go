package service

import (
	"context"
	"fmt"
	"log"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/parser"
	"ad-tracker/youtube-webhook-ingestion/internal/queue"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EventProcessor processes YouTube webhook events.
type EventProcessor interface {
	// ProcessEvent processes a raw Atom feed XML from a webhook notification.
	// It parses the XML, saves the event, and updates projection tables in a transaction.
	ProcessEvent(ctx context.Context, rawXML string) error

	// SetQueueClient sets the queue client for enrichment job enqueueing (optional)
	SetQueueClient(client *queue.Client)
}

type eventProcessor struct {
	pool             *pgxpool.Pool
	webhookEventRepo repository.WebhookEventRepository
	videoRepo        repository.VideoRepository
	channelRepo      repository.ChannelRepository
	videoUpdateRepo  repository.VideoUpdateRepository
	queueClient      *queue.Client // Optional - for enqueueing enrichment jobs
}

// NewEventProcessor creates a new EventProcessor with the given repositories.
func NewEventProcessor(
	pool *pgxpool.Pool,
	webhookEventRepo repository.WebhookEventRepository,
	videoRepo repository.VideoRepository,
	channelRepo repository.ChannelRepository,
	videoUpdateRepo repository.VideoUpdateRepository,
) EventProcessor {
	return &eventProcessor{
		pool:             pool,
		webhookEventRepo: webhookEventRepo,
		videoRepo:        videoRepo,
		channelRepo:      channelRepo,
		videoUpdateRepo:  videoUpdateRepo,
		queueClient:      nil, // Will be set via SetQueueClient if available
	}
}

// SetQueueClient sets the queue client for enrichment job enqueueing (optional)
func (p *eventProcessor) SetQueueClient(client *queue.Client) {
	p.queueClient = client
}

func (p *eventProcessor) ProcessEvent(ctx context.Context, rawXML string) error {
	videoData, err := parser.ParseAtomFeed(rawXML)
	if err != nil {
		return fmt.Errorf("parse atom feed: %w", err)
	}

	// Handle deleted videos - we still create the webhook event but don't update projections
	if videoData.IsDeleted {
		_, err := p.webhookEventRepo.CreateWebhookEvent(ctx, rawXML, "", "")
		if err != nil {
			return fmt.Errorf("create webhook event for deleted video: %w", err)
		}
		return nil
	}

	// Create the webhook event first (outside transaction)
	// This ensures we have an immutable record even if projection updates fail
	webhookEvent, err := p.webhookEventRepo.CreateWebhookEvent(
		ctx,
		rawXML,
		videoData.VideoID,
		videoData.ChannelID,
	)
	if err != nil {
		// If this is a duplicate, it's not an error - just ignore it
		if db.IsDuplicateKey(err) {
			return nil
		}
		return fmt.Errorf("create webhook event: %w", err)
	}

	// Check if video exists BEFORE processing projections
	// This is critical for determining if we should enqueue enrichment jobs
	existingVideo, err := p.videoRepo.GetVideoByID(ctx, videoData.VideoID)
	if err != nil && !db.IsNotFound(err) {
		return fmt.Errorf("check if video exists: %w", err)
	}
	isNewVideo := (existingVideo == nil)

	// Process projections in a transaction
	processingErr := p.processProjections(ctx, webhookEvent.ID, videoData)

	// Mark the event as processed (with error if projections failed)
	var errMsg string
	if processingErr != nil {
		errMsg = processingErr.Error()
	}

	if err := p.webhookEventRepo.MarkEventProcessed(ctx, webhookEvent.ID, errMsg); err != nil {
		// Log this but don't return - the event was already created
		return fmt.Errorf("mark event processed: %w (original error: %v)", err, processingErr)
	}

	if processingErr != nil {
		return fmt.Errorf("process projections: %w", processingErr)
	}

	// Enqueue enrichment job if queue client is available
	// Only enqueue for new videos to avoid overwhelming the queue
	if p.queueClient != nil && isNewVideo {
		log.Printf("[EventProcessor] New video detected: %s (channel: %s), enqueueing enrichment job", videoData.VideoID, videoData.ChannelID)
		// Enqueue enrichment job (don't fail the webhook if this fails)
		if err := p.queueClient.EnqueueVideoEnrichment(ctx, videoData.VideoID, videoData.ChannelID, 0); err != nil {
			log.Printf("[EventProcessor] Failed to enqueue enrichment job for video %s: %v", videoData.VideoID, err)
			// Don't return error - the video was still processed successfully
		} else {
			log.Printf("[EventProcessor] Successfully enqueued enrichment job for new video: %s", videoData.VideoID)
		}
	} else if p.queueClient != nil {
		log.Printf("[EventProcessor] Video %s already exists, skipping enrichment", videoData.VideoID)
	}

	return nil
}

func (p *eventProcessor) processProjections(ctx context.Context, webhookEventID int64, videoData *parser.VideoData) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback is safe to call even if committed

	existingVideo, err := p.videoRepo.GetVideoByID(ctx, videoData.VideoID)
	if err != nil && !db.IsNotFound(err) {
		return fmt.Errorf("get existing video: %w", err)
	}

	updateType := p.determineUpdateType(existingVideo, videoData)

	channel := models.NewChannel(
		videoData.ChannelID,
		"", // Channel title not available in feed
		fmt.Sprintf("https://www.youtube.com/channel/%s", videoData.ChannelID),
	)
	if err := p.channelRepo.UpsertChannel(ctx, channel); err != nil {
		return fmt.Errorf("upsert channel: %w", err)
	}

	video := models.NewVideo(
		videoData.VideoID,
		videoData.ChannelID,
		videoData.Title,
		videoData.VideoURL,
		videoData.PublishedAt,
	)
	if err := p.videoRepo.UpsertVideo(ctx, video); err != nil {
		return fmt.Errorf("upsert video: %w", err)
	}

	videoUpdate := models.NewVideoUpdate(
		webhookEventID,
		videoData.VideoID,
		videoData.ChannelID,
		videoData.Title,
		videoData.PublishedAt,
		videoData.UpdatedAt,
		updateType,
	)
	if err := p.videoUpdateRepo.CreateVideoUpdate(ctx, videoUpdate); err != nil {
		return fmt.Errorf("create video update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (p *eventProcessor) determineUpdateType(existingVideo *models.Video, videoData *parser.VideoData) models.UpdateType {
	if existingVideo == nil {
		return models.UpdateTypeNewVideo
	}

	if existingVideo.Title != videoData.Title {
		return models.UpdateTypeTitleUpdate
	}

	return models.UpdateTypeUnknown
}
