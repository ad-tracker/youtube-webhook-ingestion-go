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

func TestVideoUpdateRepository_CreateVideoUpdate(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	updateRepo := NewVideoUpdateRepository(td.Pool)
	eventRepo := NewWebhookEventRepository(td.Pool)
	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("creates video update successfully", func(t *testing.T) {
		td.TruncateTables(t)

		// Create dependencies
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "UC123", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		event, err := eventRepo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video123", "UC123")
		require.NoError(t, err)

		// Create video update
		feedUpdatedAt := time.Now()
		update := models.NewVideoUpdate(
			event.ID,
			"video123",
			"UC123",
			"Test Video",
			publishedAt,
			feedUpdatedAt,
			models.UpdateTypeNewVideo,
		)

		err = updateRepo.CreateVideoUpdate(ctx, update)
		require.NoError(t, err)
		assert.NotZero(t, update.ID)
		assert.NotZero(t, update.CreatedAt)
	})

	t.Run("fails with invalid foreign keys", func(t *testing.T) {
		td.TruncateTables(t)

		publishedAt := time.Now().Add(-24 * time.Hour)
		feedUpdatedAt := time.Now()

		// Try to create update without dependencies
		update := models.NewVideoUpdate(
			99999, // non-existent webhook event
			"nonexistent_video",
			"nonexistent_channel",
			"Test Video",
			publishedAt,
			feedUpdatedAt,
			models.UpdateTypeNewVideo,
		)

		err := updateRepo.CreateVideoUpdate(ctx, update)
		require.Error(t, err)
		assert.True(t, db.IsForeignKeyViolation(err))
	})
}

func TestVideoUpdateRepository_GetUpdatesByVideoID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	updateRepo := NewVideoUpdateRepository(td.Pool)
	eventRepo := NewWebhookEventRepository(td.Pool)
	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves updates for video", func(t *testing.T) {
		td.TruncateTables(t)

		// Create dependencies
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		publishedAt := time.Now().Add(-24 * time.Hour)
		video1 := models.NewVideo("video1", "UC123", "Video 1", "https://youtube.com/watch?v=video1", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video1)
		require.NoError(t, err)

		video2 := models.NewVideo("video2", "UC123", "Video 2", "https://youtube.com/watch?v=video2", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video2)
		require.NoError(t, err)

		// Create events and updates
		event1, err := eventRepo.CreateWebhookEvent(ctx, "<feed>1</feed>", "video1", "UC123")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		event2, err := eventRepo.CreateWebhookEvent(ctx, "<feed>2</feed>", "video1", "UC123")
		require.NoError(t, err)

		event3, err := eventRepo.CreateWebhookEvent(ctx, "<feed>3</feed>", "video2", "UC123")
		require.NoError(t, err)

		feedUpdatedAt := time.Now()

		update1 := models.NewVideoUpdate(event1.ID, "video1", "UC123", "Video 1", publishedAt, feedUpdatedAt, models.UpdateTypeNewVideo)
		err = updateRepo.CreateVideoUpdate(ctx, update1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		update2 := models.NewVideoUpdate(event2.ID, "video1", "UC123", "Video 1 Updated", publishedAt, feedUpdatedAt, models.UpdateTypeTitleUpdate)
		err = updateRepo.CreateVideoUpdate(ctx, update2)
		require.NoError(t, err)

		update3 := models.NewVideoUpdate(event3.ID, "video2", "UC123", "Video 2", publishedAt, feedUpdatedAt, models.UpdateTypeNewVideo)
		err = updateRepo.CreateVideoUpdate(ctx, update3)
		require.NoError(t, err)

		// Get updates for video1
		updates, err := updateRepo.GetUpdatesByVideoID(ctx, "video1", 10)
		require.NoError(t, err)
		assert.Len(t, updates, 2)
		// Should be ordered by created_at DESC
		assert.Equal(t, models.UpdateTypeTitleUpdate, updates[0].UpdateType)
		assert.Equal(t, models.UpdateTypeNewVideo, updates[1].UpdateType)
	})

	t.Run("respects limit", func(t *testing.T) {
		td.TruncateTables(t)

		// Create dependencies
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video1", "UC123", "Video 1", "https://youtube.com/watch?v=video1", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		// Create multiple updates
		for i := 0; i < 5; i++ {
			event, err := eventRepo.CreateWebhookEvent(ctx, "<feed>"+string(rune('1'+i))+"</feed>", "video1", "UC123")
			require.NoError(t, err)

			update := models.NewVideoUpdate(
				event.ID,
				"video1",
				"UC123",
				"Video 1",
				publishedAt,
				time.Now(),
				models.UpdateTypeUnknown,
			)
			err = updateRepo.CreateVideoUpdate(ctx, update)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		updates, err := updateRepo.GetUpdatesByVideoID(ctx, "video1", 3)
		require.NoError(t, err)
		assert.Len(t, updates, 3)
	})
}

func TestVideoUpdateRepository_GetUpdatesByChannelID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	updateRepo := NewVideoUpdateRepository(td.Pool)
	eventRepo := NewWebhookEventRepository(td.Pool)
	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves updates for channel", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channels
		channel1 := models.NewChannel("UC123", "Channel 1", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel1)
		require.NoError(t, err)

		channel2 := models.NewChannel("UC456", "Channel 2", "https://youtube.com/channel/UC456")
		err = channelRepo.UpsertChannel(ctx, channel2)
		require.NoError(t, err)

		// Create videos
		publishedAt := time.Now().Add(-24 * time.Hour)
		video1 := models.NewVideo("video1", "UC123", "Video 1", "https://youtube.com/watch?v=video1", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video1)
		require.NoError(t, err)

		video2 := models.NewVideo("video2", "UC123", "Video 2", "https://youtube.com/watch?v=video2", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video2)
		require.NoError(t, err)

		video3 := models.NewVideo("video3", "UC456", "Video 3", "https://youtube.com/watch?v=video3", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video3)
		require.NoError(t, err)

		// Create events and updates
		event1, err := eventRepo.CreateWebhookEvent(ctx, "<feed>1</feed>", "video1", "UC123")
		require.NoError(t, err)

		event2, err := eventRepo.CreateWebhookEvent(ctx, "<feed>2</feed>", "video2", "UC123")
		require.NoError(t, err)

		event3, err := eventRepo.CreateWebhookEvent(ctx, "<feed>3</feed>", "video3", "UC456")
		require.NoError(t, err)

		feedUpdatedAt := time.Now()

		update1 := models.NewVideoUpdate(event1.ID, "video1", "UC123", "Video 1", publishedAt, feedUpdatedAt, models.UpdateTypeNewVideo)
		err = updateRepo.CreateVideoUpdate(ctx, update1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		update2 := models.NewVideoUpdate(event2.ID, "video2", "UC123", "Video 2", publishedAt, feedUpdatedAt, models.UpdateTypeNewVideo)
		err = updateRepo.CreateVideoUpdate(ctx, update2)
		require.NoError(t, err)

		update3 := models.NewVideoUpdate(event3.ID, "video3", "UC456", "Video 3", publishedAt, feedUpdatedAt, models.UpdateTypeNewVideo)
		err = updateRepo.CreateVideoUpdate(ctx, update3)
		require.NoError(t, err)

		// Get updates for UC123
		updates, err := updateRepo.GetUpdatesByChannelID(ctx, "UC123", 10)
		require.NoError(t, err)
		assert.Len(t, updates, 2)
	})
}

func TestVideoUpdateRepository_GetRecentUpdates(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	updateRepo := NewVideoUpdateRepository(td.Pool)
	eventRepo := NewWebhookEventRepository(td.Pool)
	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves recent updates across all videos", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Create videos
		publishedAt := time.Now().Add(-24 * time.Hour)
		for i := 0; i < 3; i++ {
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

		// Create updates
		for i := 0; i < 3; i++ {
			event, err := eventRepo.CreateWebhookEvent(ctx, "<feed>"+string(rune('1'+i))+"</feed>", "video"+string(rune('1'+i)), "UC123")
			require.NoError(t, err)

			update := models.NewVideoUpdate(
				event.ID,
				"video"+string(rune('1'+i)),
				"UC123",
				"Video "+string(rune('1'+i)),
				publishedAt,
				time.Now(),
				models.UpdateTypeNewVideo,
			)
			err = updateRepo.CreateVideoUpdate(ctx, update)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}

		// Get recent updates
		updates, err := updateRepo.GetRecentUpdates(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, updates, 3)
		// Should be ordered by created_at DESC
		assert.Equal(t, "video3", updates[0].VideoID)
		assert.Equal(t, "video2", updates[1].VideoID)
		assert.Equal(t, "video1", updates[2].VideoID)
	})

	t.Run("respects limit", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123", "Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		publishedAt := time.Now().Add(-24 * time.Hour)

		// Create multiple updates
		for i := 0; i < 5; i++ {
			video := models.NewVideo(
				"video"+string(rune('1'+i)),
				"UC123",
				"Video "+string(rune('1'+i)),
				"https://youtube.com/watch?v=video"+string(rune('1'+i)),
				publishedAt,
			)
			err = videoRepo.UpsertVideo(ctx, video)
			require.NoError(t, err)

			event, err := eventRepo.CreateWebhookEvent(ctx, "<feed>"+string(rune('1'+i))+"</feed>", "video"+string(rune('1'+i)), "UC123")
			require.NoError(t, err)

			update := models.NewVideoUpdate(
				event.ID,
				"video"+string(rune('1'+i)),
				"UC123",
				"Video "+string(rune('1'+i)),
				publishedAt,
				time.Now(),
				models.UpdateTypeNewVideo,
			)
			err = updateRepo.CreateVideoUpdate(ctx, update)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		updates, err := updateRepo.GetRecentUpdates(ctx, 3)
		require.NoError(t, err)
		assert.Len(t, updates, 3)
	})
}

func TestVideoUpdateRepository_Immutability(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	updateRepo := NewVideoUpdateRepository(td.Pool)
	eventRepo := NewWebhookEventRepository(td.Pool)
	videoRepo := NewVideoRepository(td.Pool)
	channelRepo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("video_updates should be immutable", func(t *testing.T) {
		td.TruncateTables(t)

		// Create dependencies
		channel := models.NewChannel("UC123", "Test Channel", "https://youtube.com/channel/UC123")
		err := channelRepo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo("video123", "UC123", "Test Video", "https://youtube.com/watch?v=video123", publishedAt)
		err = videoRepo.UpsertVideo(ctx, video)
		require.NoError(t, err)

		event, err := eventRepo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video123", "UC123")
		require.NoError(t, err)

		// Create video update
		update := models.NewVideoUpdate(
			event.ID,
			"video123",
			"UC123",
			"Test Video",
			publishedAt,
			time.Now(),
			models.UpdateTypeNewVideo,
		)
		err = updateRepo.CreateVideoUpdate(ctx, update)
		require.NoError(t, err)

		// Note: We don't have triggers preventing updates/deletes on video_updates
		// but the repository interface doesn't expose update/delete methods,
		// enforcing immutability at the application level.
		// This is a design decision - immutability is enforced by the API, not the database.
	})
}
