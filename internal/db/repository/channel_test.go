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

func TestChannelRepository_UpsertChannel(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("creates new channel", func(t *testing.T) {
		td.TruncateTables(t)

		channel := models.NewChannel("UC123456789", "Test Channel", "https://youtube.com/channel/UC123456789")
		err := repo.UpsertChannel(ctx, channel)

		require.NoError(t, err)
		assert.NotZero(t, channel.FirstSeenAt)
		assert.NotZero(t, channel.LastUpdatedAt)
		assert.NotZero(t, channel.CreatedAt)
		assert.NotZero(t, channel.UpdatedAt)
	})

	t.Run("updates existing channel", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel
		channel := models.NewChannel("UC123456789", "Test Channel", "https://youtube.com/channel/UC123456789")
		err := repo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		firstSeenAt := channel.FirstSeenAt
		createdAt := channel.CreatedAt

		time.Sleep(10 * time.Millisecond)

		// Update channel
		channel.Update("Updated Channel Name", "https://youtube.com/channel/UC123456789")
		err = repo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		// Verify first_seen_at and created_at didn't change
		assert.Equal(t, firstSeenAt.Unix(), channel.FirstSeenAt.Unix())
		assert.Equal(t, createdAt.Unix(), channel.CreatedAt.Unix())

		// Verify last_updated_at and updated_at changed
		assert.True(t, channel.LastUpdatedAt.After(firstSeenAt))
		assert.True(t, channel.UpdatedAt.After(createdAt))

		// Verify title was updated
		retrieved, err := repo.GetChannelByID(ctx, "UC123456789")
		require.NoError(t, err)
		assert.Equal(t, "Updated Channel Name", retrieved.Title)
	})
}

func TestChannelRepository_GetChannelByID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves channel successfully", func(t *testing.T) {
		td.TruncateTables(t)

		channel := models.NewChannel("UC123456789", "Test Channel", "https://youtube.com/channel/UC123456789")
		err := repo.UpsertChannel(ctx, channel)
		require.NoError(t, err)

		retrieved, err := repo.GetChannelByID(ctx, "UC123456789")
		require.NoError(t, err)
		assert.Equal(t, channel.ChannelID, retrieved.ChannelID)
		assert.Equal(t, channel.Title, retrieved.Title)
		assert.Equal(t, channel.ChannelURL, retrieved.ChannelURL)
	})

	t.Run("returns error for non-existent channel", func(t *testing.T) {
		td.TruncateTables(t)

		_, err := repo.GetChannelByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestChannelRepository_ListChannels(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("lists channels with pagination", func(t *testing.T) {
		td.TruncateTables(t)

		// Create multiple channels
		for i := 0; i < 5; i++ {
			channel := models.NewChannel(
				"UC"+string(rune('1'+i)),
				"Channel "+string(rune('1'+i)),
				"https://youtube.com/channel/UC"+string(rune('1'+i)),
			)
			err := repo.UpsertChannel(ctx, channel)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		// Get first page
		channels, err := repo.ListChannels(ctx, 3, 0)
		require.NoError(t, err)
		assert.Len(t, channels, 3)

		// Get second page
		channels, err = repo.ListChannels(ctx, 3, 3)
		require.NoError(t, err)
		assert.Len(t, channels, 2)
	})

	t.Run("orders by last_updated_at DESC", func(t *testing.T) {
		td.TruncateTables(t)

		channel1 := models.NewChannel("UC1", "Channel 1", "https://youtube.com/channel/UC1")
		err := repo.UpsertChannel(ctx, channel1)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		channel2 := models.NewChannel("UC2", "Channel 2", "https://youtube.com/channel/UC2")
		err = repo.UpsertChannel(ctx, channel2)
		require.NoError(t, err)

		channels, err := repo.ListChannels(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, channels, 2)
		assert.Equal(t, "UC2", channels[0].ChannelID)
		assert.Equal(t, "UC1", channels[1].ChannelID)
	})
}

func TestChannelRepository_GetChannelsByLastUpdated(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewChannelRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves channels updated since timestamp", func(t *testing.T) {
		td.TruncateTables(t)

		// Create channel 1
		channel1 := models.NewChannel("UC1", "Channel 1", "https://youtube.com/channel/UC1")
		err := repo.UpsertChannel(ctx, channel1)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		since := time.Now()
		time.Sleep(100 * time.Millisecond)

		// Create channel 2 after 'since' timestamp
		channel2 := models.NewChannel("UC2", "Channel 2", "https://youtube.com/channel/UC2")
		err = repo.UpsertChannel(ctx, channel2)
		require.NoError(t, err)

		// Update channel 1 after 'since' timestamp
		channel1.Update("Updated Channel 1", "https://youtube.com/channel/UC1")
		err = repo.UpsertChannel(ctx, channel1)
		require.NoError(t, err)

		channels, err := repo.GetChannelsByLastUpdated(ctx, since, 10)
		require.NoError(t, err)
		assert.Len(t, channels, 2)
	})

	t.Run("respects limit", func(t *testing.T) {
		td.TruncateTables(t)

		for i := 0; i < 5; i++ {
			channel := models.NewChannel(
				"UC"+string(rune('1'+i)),
				"Channel "+string(rune('1'+i)),
				"https://youtube.com/channel/UC"+string(rune('1'+i)),
			)
			err := repo.UpsertChannel(ctx, channel)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		channels, err := repo.GetChannelsByLastUpdated(ctx, time.Now().Add(-1*time.Hour), 3)
		require.NoError(t, err)
		assert.Len(t, channels, 3)
	})
}
