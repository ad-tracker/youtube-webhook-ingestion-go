package youtube

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"ad-tracker/youtube-webhook-ingestion/internal/model"
)

// Client wraps the YouTube Data API v3 client
type Client struct {
	service *youtube.Service
	apiKey  string
}

// NewClient creates a new YouTube API client
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("YouTube API key is required")
	}

	service, err := youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return &Client{
		service: service,
		apiKey:  apiKey,
	}, nil
}

// FetchVideos retrieves comprehensive data for up to 50 videos in a single batch
// Returns enrichment data and the quota cost of the operation
func (c *Client) FetchVideos(ctx context.Context, videoIDs []string) ([]*model.VideoEnrichment, int, error) {
	if len(videoIDs) == 0 {
		return nil, 0, fmt.Errorf("no video IDs provided")
	}

	if len(videoIDs) > 50 {
		return nil, 0, fmt.Errorf("too many video IDs (max 50, got %d)", len(videoIDs))
	}

	// Request all available parts for comprehensive data
	parts := []string{
		"snippet",
		"contentDetails",
		"statistics",
		"status",
		"topicDetails",
		"recordingDetails",
		"liveStreamingDetails",
		"player",
	}

	call := c.service.Videos.List(parts).Id(videoIDs...).Context(ctx)

	response, err := call.Do()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch videos from YouTube API: %w", err)
	}

	// Quota cost calculation:
	// videos.list base cost = 1 unit
	// Each additional part beyond the first adds approximately 2 units
	// For 8 parts, typical cost is ~5-6 units
	quotaCost := 5 // Conservative estimate

	enrichments := make([]*model.VideoEnrichment, 0, len(response.Items))

	for _, item := range response.Items {
		enrichment := c.mapVideoToEnrichment(item, parts, response.Etag)
		enrichments = append(enrichments, enrichment)
	}

	return enrichments, quotaCost, nil
}

// mapVideoToEnrichment converts YouTube API video response to our enrichment model
func (c *Client) mapVideoToEnrichment(video *youtube.Video, partsRequested []string, etag string) *model.VideoEnrichment {
	enrichment := &model.VideoEnrichment{
		VideoID:           video.Id,
		APIResponseEtag:   strPtr(etag),
		APIPartsRequested: partsRequested,
		QuotaCost:         5, // Will be set by caller
		RawAPIResponse:    make(map[string]interface{}),
	}

	// Store raw response for future reference
	// We'll store a simplified version to avoid deep copying
	enrichment.RawAPIResponse = map[string]interface{}{
		"id":   video.Id,
		"etag": video.Etag,
		"kind": video.Kind,
	}

	// Map Snippet data
	if video.Snippet != nil {
		enrichment.Description = strPtr(video.Snippet.Description)
		enrichment.ChannelTitle = strPtr(video.Snippet.ChannelTitle)
		enrichment.DefaultLanguage = strPtr(video.Snippet.DefaultLanguage)
		enrichment.DefaultAudioLanguage = strPtr(video.Snippet.DefaultAudioLanguage)
		enrichment.CategoryID = strPtr(video.Snippet.CategoryId)

		if video.Snippet.Tags != nil {
			enrichment.Tags = video.Snippet.Tags
		} else {
			enrichment.Tags = []string{}
		}

		// Map thumbnails
		if video.Snippet.Thumbnails != nil {
			if video.Snippet.Thumbnails.Default != nil {
				enrichment.ThumbnailDefaultURL = strPtr(video.Snippet.Thumbnails.Default.Url)
				enrichment.ThumbnailDefaultWidth = intPtr(int(video.Snippet.Thumbnails.Default.Width))
				enrichment.ThumbnailDefaultHeight = intPtr(int(video.Snippet.Thumbnails.Default.Height))
			}
			if video.Snippet.Thumbnails.Medium != nil {
				enrichment.ThumbnailMediumURL = strPtr(video.Snippet.Thumbnails.Medium.Url)
				enrichment.ThumbnailMediumWidth = intPtr(int(video.Snippet.Thumbnails.Medium.Width))
				enrichment.ThumbnailMediumHeight = intPtr(int(video.Snippet.Thumbnails.Medium.Height))
			}
			if video.Snippet.Thumbnails.High != nil {
				enrichment.ThumbnailHighURL = strPtr(video.Snippet.Thumbnails.High.Url)
				enrichment.ThumbnailHighWidth = intPtr(int(video.Snippet.Thumbnails.High.Width))
				enrichment.ThumbnailHighHeight = intPtr(int(video.Snippet.Thumbnails.High.Height))
			}
			if video.Snippet.Thumbnails.Standard != nil {
				enrichment.ThumbnailStandardURL = strPtr(video.Snippet.Thumbnails.Standard.Url)
				enrichment.ThumbnailStandardWidth = intPtr(int(video.Snippet.Thumbnails.Standard.Width))
				enrichment.ThumbnailStandardHeight = intPtr(int(video.Snippet.Thumbnails.Standard.Height))
			}
			if video.Snippet.Thumbnails.Maxres != nil {
				enrichment.ThumbnailMaxresURL = strPtr(video.Snippet.Thumbnails.Maxres.Url)
				enrichment.ThumbnailMaxresWidth = intPtr(int(video.Snippet.Thumbnails.Maxres.Width))
				enrichment.ThumbnailMaxresHeight = intPtr(int(video.Snippet.Thumbnails.Maxres.Height))
			}
		}
	}

	// Map ContentDetails
	if video.ContentDetails != nil {
		enrichment.Duration = strPtr(video.ContentDetails.Duration)
		enrichment.Dimension = strPtr(video.ContentDetails.Dimension)
		enrichment.Definition = strPtr(video.ContentDetails.Definition)
		enrichment.Caption = strPtr(video.ContentDetails.Caption)
		enrichment.LicensedContent = boolPtr(video.ContentDetails.LicensedContent)
		enrichment.Projection = strPtr(video.ContentDetails.Projection)

		// Content rating is complex, store as map
		if video.ContentDetails.ContentRating != nil {
			enrichment.ContentRating = make(map[string]interface{})
			// YouTube API returns various rating systems, we'll store them generically
			// Examples: acbRating, cbfcRating, mpaaRating, etc.
			// For now, store as empty map and let raw JSON capture full details
		}
	}

	// Map Statistics
	if video.Statistics != nil {
		enrichment.ViewCount = int64Ptr(int64(video.Statistics.ViewCount))
		enrichment.LikeCount = int64Ptr(int64(video.Statistics.LikeCount))
		enrichment.DislikeCount = int64Ptr(int64(video.Statistics.DislikeCount))
		enrichment.FavoriteCount = int64Ptr(int64(video.Statistics.FavoriteCount))
		enrichment.CommentCount = int64Ptr(int64(video.Statistics.CommentCount))
	}

	// Map Status
	if video.Status != nil {
		enrichment.UploadStatus = strPtr(video.Status.UploadStatus)
		enrichment.FailureReason = strPtr(video.Status.FailureReason)
		enrichment.RejectionReason = strPtr(video.Status.RejectionReason)
		enrichment.PrivacyStatus = strPtr(video.Status.PrivacyStatus)
		enrichment.License = strPtr(video.Status.License)
		enrichment.Embeddable = boolPtr(video.Status.Embeddable)
		enrichment.PublicStatsViewable = boolPtr(video.Status.PublicStatsViewable)
		enrichment.MadeForKids = boolPtr(video.Status.MadeForKids)
		enrichment.SelfDeclaredMadeForKids = boolPtr(video.Status.SelfDeclaredMadeForKids)
	}

	// Map TopicDetails
	if video.TopicDetails != nil && video.TopicDetails.TopicCategories != nil {
		enrichment.TopicCategories = video.TopicDetails.TopicCategories
	} else {
		enrichment.TopicCategories = []string{}
	}

	// Map RecordingDetails (location data)
	if video.RecordingDetails != nil {
		enrichment.LocationDescription = strPtr(video.RecordingDetails.LocationDescription)
		if video.RecordingDetails.Location != nil {
			enrichment.LocationLatitude = float64Ptr(video.RecordingDetails.Location.Latitude)
			enrichment.LocationLongitude = float64Ptr(video.RecordingDetails.Location.Longitude)
		}
	}

	// Map LiveStreamingDetails
	if video.LiveStreamingDetails != nil {
		enrichment.LiveBroadcastContent = strPtr(video.Status.PrivacyStatus) // Note: actual field would be in snippet
		enrichment.ConcurrentViewers = int64Ptr(int64(video.LiveStreamingDetails.ConcurrentViewers))

		if video.LiveStreamingDetails.ScheduledStartTime != "" {
			if t, err := parseYouTubeTime(video.LiveStreamingDetails.ScheduledStartTime); err == nil {
				enrichment.ScheduledStartTime = &t
			}
		}
		if video.LiveStreamingDetails.ActualStartTime != "" {
			if t, err := parseYouTubeTime(video.LiveStreamingDetails.ActualStartTime); err == nil {
				enrichment.ActualStartTime = &t
			}
		}
		if video.LiveStreamingDetails.ActualEndTime != "" {
			if t, err := parseYouTubeTime(video.LiveStreamingDetails.ActualEndTime); err == nil {
				enrichment.ActualEndTime = &t
			}
		}
	}

	return enrichment
}

// FetchVideosBatch is an alias for FetchVideos for clarity
func (c *Client) FetchVideosBatch(ctx context.Context, videoIDs []string) ([]*model.VideoEnrichment, int, error) {
	return c.FetchVideos(ctx, videoIDs)
}

// Helper functions for pointer conversions

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func float64Ptr(f float64) *float64 {
	return &f
}

// parseYouTubeTime parses RFC3339 timestamps from YouTube API
func parseYouTubeTime(s string) (time.Time, error) {
	// YouTube API returns RFC3339 format
	return time.Parse(time.RFC3339, s)
}

// LogAPICall logs API usage for debugging
func (c *Client) LogAPICall(method string, quotaCost int, videoCount int) {
	log.Printf("[YouTube API] %s - Videos: %d, Quota Cost: %d", method, videoCount, quotaCost)
}

// BatchVideoIDs splits a large list of video IDs into batches of 50
func BatchVideoIDs(videoIDs []string, batchSize int) [][]string {
	if batchSize <= 0 || batchSize > 50 {
		batchSize = 50
	}

	var batches [][]string
	for i := 0; i < len(videoIDs); i += batchSize {
		end := i + batchSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batches = append(batches, videoIDs[i:end])
	}

	return batches
}

// ParseVideoDuration converts ISO 8601 duration to seconds
// Example: "PT4M13S" -> 253 seconds
func ParseVideoDuration(duration string) (int, error) {
	if !strings.HasPrefix(duration, "PT") {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	// Remove PT prefix
	duration = strings.TrimPrefix(duration, "PT")

	var hours, minutes, seconds int

	// Parse hours
	if hIdx := strings.Index(duration, "H"); hIdx != -1 {
		h, err := strconv.Atoi(duration[:hIdx])
		if err != nil {
			return 0, err
		}
		hours = h
		duration = duration[hIdx+1:]
	}

	// Parse minutes
	if mIdx := strings.Index(duration, "M"); mIdx != -1 {
		m, err := strconv.Atoi(duration[:mIdx])
		if err != nil {
			return 0, err
		}
		minutes = m
		duration = duration[mIdx+1:]
	}

	// Parse seconds
	if sIdx := strings.Index(duration, "S"); sIdx != -1 {
		s, err := strconv.Atoi(duration[:sIdx])
		if err != nil {
			return 0, err
		}
		seconds = s
	}

	return hours*3600 + minutes*60 + seconds, nil
}
