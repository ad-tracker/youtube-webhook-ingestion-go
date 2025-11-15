package service

import (
	"context"
	"fmt"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/parser"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EventProcessor processes YouTube webhook events.
type EventProcessor interface {
	// ProcessEvent processes a raw Atom feed XML from a webhook notification.
	// It parses the XML, saves the event, and updates projection tables in a transaction.
	ProcessEvent(ctx context.Context, rawXML string) error
}

type eventProcessor struct {
	pool               *pgxpool.Pool
	webhookEventRepo   repository.WebhookEventRepository
	videoRepo          repository.VideoRepository
	channelRepo        repository.ChannelRepository
	videoUpdateRepo    repository.VideoUpdateRepository
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
	}
}

func (p *eventProcessor) ProcessEvent(ctx context.Context, rawXML string) error {
	// Parse the Atom feed
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

	// Process projections in a transaction
	processingErr := p.processProjections(ctx, webhookEvent.ID, videoData)

	// Mark the event as processed (with error if projections failed)
	var errMsg string
	if processingErr != nil {
		errMsg = processingErr.Error()
	}

	if err := p.webhookEventRepo.MarkEventProcessed(ctx, webhookEvent.ID, errMsg); err != nil {
		// Log this but don't return - the event was already created
		// In production, you'd want proper logging here
		return fmt.Errorf("mark event processed: %w (original error: %v)", err, processingErr)
	}

	// Return the original processing error if there was one
	if processingErr != nil {
		return fmt.Errorf("process projections: %w", processingErr)
	}

	return nil
}

func (p *eventProcessor) processProjections(ctx context.Context, webhookEventID int64, videoData *parser.VideoData) error {
	// Start a transaction for projection updates
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback is safe to call even if committed

	// Get existing video to determine update type
	existingVideo, err := p.videoRepo.GetVideoByID(ctx, videoData.VideoID)
	if err != nil && !db.IsNotFound(err) {
		return fmt.Errorf("get existing video: %w", err)
	}

	// Determine update type
	updateType := p.determineUpdateType(existingVideo, videoData)

	// Upsert channel
	channel := models.NewChannel(
		videoData.ChannelID,
		"", // Channel title not available in feed
		fmt.Sprintf("https://www.youtube.com/channel/%s", videoData.ChannelID),
	)
	if err := p.channelRepo.UpsertChannel(ctx, channel); err != nil {
		return fmt.Errorf("upsert channel: %w", err)
	}

	// Upsert video
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

	// Create video update record
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

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (p *eventProcessor) determineUpdateType(existingVideo *models.Video, videoData *parser.VideoData) models.UpdateType {
	// If video doesn't exist, it's a new video
	if existingVideo == nil {
		return models.UpdateTypeNewVideo
	}

	// If title changed, it's a title update
	if existingVideo.Title != videoData.Title {
		return models.UpdateTypeTitleUpdate
	}

	// Otherwise, it's an unknown update type
	return models.UpdateTypeUnknown
}
