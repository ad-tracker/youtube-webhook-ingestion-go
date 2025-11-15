package repository

import (
	"context"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoRepository_UpsertVideo(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("creates new video", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel first (foreign key dependency)
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create video
		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "UC123", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)

		require.NoError(t, err)
		assert.NotZero(t, video.FirstSeenAt)
		assert.NotZero(t, video.LastUpdatedAt)
		assert.Equal(t, publishedAt.Unix(), video.PublishedAt.Unix())
	})

	t.Run("updates existing video", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create video
		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "UC123", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		firstSeenAt := video.FirstSeenAt
		createdAt := video.CreatedAt

		time.Sleep(10 * time.Millisecond)

		// Update video
		video.Update("Updated Video Title", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		// Verify first_seen_at and created_at didn't change
		assert.Equal(t, firstSeenAt.Unix(), video.FirstSeenAt.Unix())
		assert.Equal(t, createdAt.Unix(), video.CreatedAt.Unix())

		// Verify last_updated_at and updated_at changed
		assert.True(t, video.LastUpdatedAt.After(firstSeenAt))
		assert.True(t, video.UpdatedAt.After(createdAt))

		// Verify title was updated
		retrieved, err := videoRepo.GetVideoByID(ctx, "video123")
		require.NoError(t, err)
		assert.Equal(t, "Updated Video Title", retrieved.Title)
	})

	t.Run("fails with invalid channel_id", func(t *testing.T) {
		td.TruncateTables(t)

		// Try to create video without channel
		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "nonexistent", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err := videoRepo.UpsertVideo(ctx, video)

		require.Error(t, err)
		assert.True(t, db.IsForeignKeyViolation(err))
	})
}

func TestVideoRepository_GetVideoByID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves video successfully", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create video
		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "UC123", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		// Retrieve video
		retrieved, err := videoRepo.GetVideoByID(ctx, "video123")
		require.NoError(t, err)
		assert.Equal(t, video.VideoID, retrieved.VideoID)
		assert.Equal(t, video.Title, retrieved.Title)
		assert.Equal(t, video.ChannelID, retrieved.ChannelID)
	})

	t.Run("returns error for non-existent video", func(t *testing.T) {
		td.TruncateTables(t)

		_, err := videoRepo.GetVideoByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestVideoRepository_GetVideosByChannelID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves videos for channel", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channels
		channel1 := models.NewChannel("UC123", "Channel 1", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel1)
		require.NoError(t, err)

		channel2 := models.NewChannel("UC456", "Channel 2", "https://youtube.com/channel/UC456")
		err = channelRepo.UpsertChannel(ctx, channel2)
		require.NoError(t, err)

		// Create videos for channel1
		publishedAt1 := time.Now().Add(-48 * time.Hour)
		video1 := models.NewVideo("video1", "UC123", "Video 1", "https://youtube.com/watch?v=video1", publishedAt1)
		err = videoRepo.UpsertVideo(ctx, video1)
		require.NoError(t, err)

		publishedAt2 := time.Now().Add(-24 * time.Hour)
		video2 := models.NewVideo("video2", "UC123", "Video 2", "https://youtube.com/watch?v=video2", publishedAt2)
		err = videoRepo.UpsertVideo(ctx, video2)
		require.NoError(t, err)

		// Create video for channel2
		video3 := models.NewVideo("video3", "UC456", "Video 3", "https://youtube.com/watch?v=video3", publishedAt1)
		err = videoRepo.UpsertVideo(ctx, video3)
		require.NoError(t, err)

		// Get videos for channel1
		videos, err := videoRepo.GetVideosByChannelID(ctx, "UC123", 10)
		require.NoError(t, err)
		assert.Len(t, videos, 2)
		// Should be ordered by published_at DESC
		assert.Equal(t, "video2", videos[0].VideoID)
		assert.Equal(t, "video1", videos[1].VideoID)
	})

	t.Run("respects limit", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create multiple videos
		for i := 0; i < 5; i++ {
			publishedAt := time.Now().Add(-time.Duration(i) * time.Hour)
			video := models.NewVideo(
				"video"+string(rune('1'+i)),
				"UC123",
				"Video "+string(rune('1'+i)),
				"https://youtube.com/watch?v=video"+string(rune('1'+i)),
				publishedAt,
			)
			err = videoRepo.UpsertVideo(ctx, video)
			require.NoError(t, err)
		}

		videos, err := videoRepo.GetVideosByChannelID(ctx, "UC123", 3)
		require.NoError(t, err)
		assert.Len(t, videos, 3)
	})
}

func TestVideoRepository_ListVideos(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("lists videos with pagination", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create multiple videos
		for i := 0; i < 5; i++ {
			publishedAt := time.Now().Add(-time.Duration(i) * time.Hour)
			video := models.NewVideo(
				"video"+string(rune('1'+i)),
				"UC123",
				"Video "+string(rune('1'+i)),
				"https://youtube.com/watch?v=video"+string(rune('1'+i)),
				publishedAt,
			)
			err = videoRepo.UpsertVideo(ctx, video)
			require.NoError(t, err)
		}

		// Get first page
		videos, err := videoRepo.ListVideos(ctx, 3, 0)
		require.NoError(t, err)
		assert.Len(t, videos, 3)

		// Get second page
		videos, err = videoRepo.ListVideos(ctx, 3, 3)
		require.NoError(t, err)
		assert.Len(t, videos, 2)
	})
}

func TestVideoRepository_GetVideosByPublishedDate(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves videos published since timestamp", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create old video
		oldPublishedAt := time.Now().Add(-48 * time.Hour)
		video1 := models.NewVideo("video1", "UC123", "Old Video", "https://youtube.com/watch?v=video1", oldPublishedAt)
		err = videoRepo.UpsertVideo(ctx, video1)
		require.NoError(t, err)

		since := time.Now().Add(-30 * time.Hour)

		// Create recent videos
		recentPublishedAt1 := time.Now().Add(-24 * time.Hour)
		video2 := models.NewVideo("video2", "UC123", "Recent Video 1", "https://youtube.com/watch?v=video2", recentPublishedAt1)
		err = videoRepo.UpsertVideo(ctx, video2)
		require.NoError(t, err)

		recentPublishedAt2 := time.Now().Add(-12 * time.Hour)
		video3 := models.NewVideo("video3", "UC123", "Recent Video 2", "https://youtube.com/watch?v=video3", recentPublishedAt2)
		err = videoRepo.UpsertVideo(ctx, video3)
		require.NoError(t, err)

		videos, err := videoRepo.GetVideosByPublishedDate(ctx, since, 10)
		require.NoError(t, err)
		assert.Len(t, videos, 2)
		assert.Equal(t, "video3", videos[0].VideoID)
		assert.Equal(t, "video2", videos[1].VideoID)
	})
}
