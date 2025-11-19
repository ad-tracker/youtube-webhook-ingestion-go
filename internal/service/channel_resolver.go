package service

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
)

// ChannelResolverService orchestrates channel resolution and enrichment
type ChannelResolverService struct {
	youtubeClient     *youtube.Client
	channelRepo       repository.ChannelRepository
	subscriptionRepo  repository.SubscriptionRepository
	enrichmentRepo    repository.ChannelEnrichmentRepository
	quotaManager      *quota.Manager
	pubSubHubService  *PubSubHubService
}

// NewChannelResolverService creates a new channel resolver service
func NewChannelResolverService(
	youtubeClient *youtube.Client,
	channelRepo repository.ChannelRepository,
	subscriptionRepo repository.SubscriptionRepository,
	enrichmentRepo repository.ChannelEnrichmentRepository,
	quotaManager *quota.Manager,
	pubSubHubService *PubSubHubService,
) *ChannelResolverService {
	return &ChannelResolverService{
		youtubeClient:    youtubeClient,
		channelRepo:      channelRepo,
		subscriptionRepo: subscriptionRepo,
		enrichmentRepo:   enrichmentRepo,
		quotaManager:     quotaManager,
		pubSubHubService: pubSubHubService,
	}
}

// ResolveChannelFromURLRequest represents the request to resolve a channel from a URL
type ResolveChannelFromURLRequest struct {
	URL         string
	CallbackURL string
}

// ResolveChannelFromURLResponse represents the response from resolving a channel
type ResolveChannelFromURLResponse struct {
	Channel      *models.Channel          `json:"channel"`
	Subscription *models.Subscription     `json:"subscription,omitempty"`
	Enrichment   *model.ChannelEnrichment `json:"enrichment,omitempty"`
	WasExisting  bool                     `json:"was_existing"`
}

// ResolveChannelFromURL resolves a YouTube channel from its URL and creates/updates records
func (s *ChannelResolverService) ResolveChannelFromURL(ctx context.Context, req ResolveChannelFromURLRequest) (*ResolveChannelFromURLResponse, error) {
	log.Printf("[ChannelResolver] Resolving channel from URL: %s", req.URL)

	// Step 1: Resolve the channel via YouTube API
	ytEnrichment, err := s.youtubeClient.ResolveChannelByURL(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve channel from YouTube: %w", err)
	}

	log.Printf("[ChannelResolver] Resolved channel: %s (%s)", ytEnrichment.Title, ytEnrichment.ChannelID)

	// Step 2: Check if channel already exists
	existingChannel, err := s.channelRepo.GetByID(ctx, ytEnrichment.ChannelID)
	wasExisting := false
	if err == nil && existingChannel != nil {
		log.Printf("[ChannelResolver] Channel already exists: %s", ytEnrichment.ChannelID)
		wasExisting = true
	}

	// Step 3: Create or update channel
	channel := &models.Channel{
		ChannelID:      ytEnrichment.ChannelID,
		Title:          ytEnrichment.Title,
		ChannelURL:     fmt.Sprintf("https://www.youtube.com/channel/%s", ytEnrichment.ChannelID),
		FirstSeenAt:    ytEnrichment.PublishedAt,
		LastUpdatedAt:  time.Now(),
	}

	if wasExisting {
		// Update existing channel
		channel.CreatedAt = existingChannel.CreatedAt
		err = s.channelRepo.Update(ctx, channel)
		if err != nil {
			return nil, fmt.Errorf("failed to update channel: %w", err)
		}
	} else {
		// Create new channel
		err = s.channelRepo.Create(ctx, channel)
		if err != nil {
			return nil, fmt.Errorf("failed to create channel: %w", err)
		}
	}

	// Step 4: Store enrichment data
	enrichment := s.mapYouTubeEnrichmentToModel(ytEnrichment)
	err = s.enrichmentRepo.Create(ctx, enrichment)
	if err != nil {
		log.Printf("[ChannelResolver] Warning: Failed to store enrichment: %v", err)
		// Don't fail the entire operation if enrichment storage fails
	}

	// Step 5: Track quota usage
	if s.quotaManager != nil {
		err = s.quotaManager.RecordQuotaUsage(ctx, ytEnrichment.QuotaCost, "channels_list")
		if err != nil {
			log.Printf("[ChannelResolver] Warning: Failed to track quota: %v", err)
		}
	}

	// Step 6: Create PubSubHubbub subscription if callback URL provided
	var subscription *models.Subscription
	if req.CallbackURL != "" {
		subscription, err = s.createSubscription(ctx, ytEnrichment.ChannelID, req.CallbackURL)
		if err != nil {
			log.Printf("[ChannelResolver] Warning: Failed to create subscription: %v", err)
			// Don't fail the entire operation if subscription creation fails
		}
	}

	return &ResolveChannelFromURLResponse{
		Channel:      channel,
		Subscription: subscription,
		Enrichment:   enrichment,
		WasExisting:  wasExisting,
	}, nil
}

// createSubscription creates a PubSubHubbub subscription for a channel
func (s *ChannelResolverService) createSubscription(ctx context.Context, channelID, callbackURL string) (*models.Subscription, error) {
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	subscription := &models.Subscription{
		ChannelID:   channelID,
		TopicURL:    topicURL,
		CallbackURL: callbackURL,
		HubURL:      "https://pubsubhubbub.appspot.com/subscribe",
		LeaseSeconds: 432000, // 5 days
		ExpiresAt:   time.Now().Add(432000 * time.Second),
		Status:      "pending",
	}

	// Create the subscription in the database
	err := s.subscriptionRepo.Create(ctx, subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription in database: %w", err)
	}

	// Subscribe to PubSubHubbub
	if s.pubSubHubService != nil {
		err = s.pubSubHubService.Subscribe(ctx, subscription)
		if err != nil {
			log.Printf("[ChannelResolver] Warning: Failed to subscribe to PubSubHubbub: %v", err)
			// Update status to failed
			subscription.Status = "failed"
			_ = s.subscriptionRepo.Update(ctx, subscription)
			return subscription, err
		}

		// Update status to active
		subscription.Status = "active"
		subscription.LastVerifiedAt = timePtr(time.Now())
		_ = s.subscriptionRepo.Update(ctx, subscription)
	}

	return subscription, nil
}

// mapYouTubeEnrichmentToModel converts YouTube API enrichment to database model
func (s *ChannelResolverService) mapYouTubeEnrichmentToModel(yt *youtube.ChannelEnrichment) *model.ChannelEnrichment {
	enrichment := &model.ChannelEnrichment{
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
		EnrichedAt:          time.Now(),
		APIResponseEtag:     strPtrIfNotEmpty(yt.APIResponseEtag),
		QuotaCost:           yt.QuotaCost,
		APIPartsRequested:   []string{"snippet", "contentDetails", "statistics", "brandingSettings"},
		RawAPIResponse:      make(map[string]interface{}),
	}

	if !yt.PublishedAt.IsZero() {
		enrichment.PublishedAt = &yt.PublishedAt
	}

	return enrichment
}

// Helper functions

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

func timePtr(t time.Time) *time.Time {
	return &t
}
