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

func TestSubscriptionRepository_Create(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("creates new subscription successfully", func(t *testing.T) {
		td.TruncateTables(t)

		sub := models.NewSubscription("UCtest123", 432000)

		err := repo.Create(ctx, sub)
		require.NoError(t, err)
		assert.NotZero(t, sub.ID)
		assert.NotZero(t, sub.CreatedAt)
		assert.NotZero(t, sub.UpdatedAt)
		assert.Equal(t, "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCtest123", sub.TopicURL)
		assert.Equal(t, models.StatusPending, sub.Status)
	})

	t.Run("returns error on duplicate channel_id and callback_url", func(t *testing.T) {
		td.TruncateTables(t)

		sub1 := models.NewSubscription("UCtest789", 432000)
		err := repo.Create(ctx, sub1)
		require.NoError(t, err)

		// Try to create duplicate
		sub2 := models.NewSubscription("UCtest789", 432000)
		err = repo.Create(ctx, sub2)
		require.Error(t, err)
		assert.True(t, db.IsDuplicateKey(err))
	})
}

func TestSubscriptionRepository_GetByID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves subscription successfully", func(t *testing.T) {
		td.TruncateTables(t)

		sub := models.NewSubscription("UCtest123", 432000)
		err := repo.Create(ctx, sub)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, sub.ID, retrieved.ID)
		assert.Equal(t, sub.ChannelID, retrieved.ChannelID)
		assert.Equal(t, sub.TopicURL, retrieved.TopicURL)
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		td.TruncateTables(t)

		_, err := repo.GetByID(ctx, 99999)
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestSubscriptionRepository_GetByChannelID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves subscription for channel", func(t *testing.T) {
		td.TruncateTables(t)

		channelID := "UCtest123"

		// Create subscription for the channel
		sub1 := models.NewSubscription(channelID, 432000)
		err := repo.Create(ctx, sub1)
		require.NoError(t, err)

		// Create subscription for different channel
		sub2 := models.NewSubscription("UCother456", 432000)
		err = repo.Create(ctx, sub2)
		require.NoError(t, err)

		subscriptions, err := repo.GetByChannelID(ctx, channelID)
		require.NoError(t, err)
		assert.Len(t, subscriptions, 1)
		assert.Equal(t, sub1.ID, subscriptions[0].ID)
	})

	t.Run("returns empty slice for channel with no subscriptions", func(t *testing.T) {
		td.TruncateTables(t)

		subscriptions, err := repo.GetByChannelID(ctx, "UCnonexistent")
		require.NoError(t, err)
		assert.Empty(t, subscriptions)
	})
}

func TestSubscriptionRepository_Update(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("updates subscription successfully", func(t *testing.T) {
		td.TruncateTables(t)

		sub := models.NewSubscription("UCtest123", 432000)
		err := repo.Create(ctx, sub)
		require.NoError(t, err)

		originalUpdatedAt := sub.UpdatedAt
		time.Sleep(10 * time.Millisecond)

		// Mark as active
		sub.MarkActive()
		err = repo.Update(ctx, sub)
		require.NoError(t, err)

		assert.True(t, sub.UpdatedAt.After(originalUpdatedAt))
		assert.Equal(t, models.StatusActive, sub.Status)
		assert.NotNil(t, sub.LastVerifiedAt)

		// Verify update persisted
		retrieved, err := repo.GetByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, retrieved.Status)
		assert.NotNil(t, retrieved.LastVerifiedAt)
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		td.TruncateTables(t)

		sub := &models.Subscription{ID: 99999}
		err := repo.Update(ctx, sub)
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestSubscriptionRepository_Delete(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("deletes subscription successfully", func(t *testing.T) {
		td.TruncateTables(t)

		sub := models.NewSubscription("UCtest123", 432000)
		err := repo.Create(ctx, sub)
		require.NoError(t, err)

		err = repo.Delete(ctx, sub.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = repo.GetByID(ctx, sub.ID)
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		td.TruncateTables(t)

		err := repo.Delete(ctx, 99999)
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestSubscriptionRepository_GetExpiringSoon(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves subscriptions expiring within 24 hours", func(t *testing.T) {
		td.TruncateTables(t)

		// Create subscription expiring in 1 hour
		sub1 := models.NewSubscription("UCtest1", 3600)
		sub1.Status = models.StatusActive
		err := repo.Create(ctx, sub1)
		require.NoError(t, err)

		// Create subscription expiring in 2 days (should not be returned)
		sub2 := models.NewSubscription("UCtest2", 172800)
		sub2.Status = models.StatusActive
		err = repo.Create(ctx, sub2)
		require.NoError(t, err)

		// Create pending subscription expiring soon (should not be returned)
		sub3 := models.NewSubscription("UCtest3", 3600)
		err = repo.Create(ctx, sub3)
		require.NoError(t, err)

		subscriptions, err := repo.GetExpiringSoon(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, subscriptions, 1)
		assert.Equal(t, sub1.ID, subscriptions[0].ID)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		td.TruncateTables(t)

		// Create multiple subscriptions expiring soon
		for i := 0; i < 5; i++ {
			sub := models.NewSubscription("UCtest"+string(rune('1'+i)), 3600)
			sub.Status = models.StatusActive
			err := repo.Create(ctx, sub)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		subscriptions, err := repo.GetExpiringSoon(ctx, 3)
		require.NoError(t, err)
		assert.Len(t, subscriptions, 3)
	})
}

func TestSubscriptionRepository_GetByStatus(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewSubscriptionRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves subscriptions by status", func(t *testing.T) {
		td.TruncateTables(t)

		// Create subscriptions with different statuses
		sub1 := models.NewSubscription("UCtest1", 432000)
		err := repo.Create(ctx, sub1)
		require.NoError(t, err)

		sub2 := models.NewSubscription("UCtest2", 432000)
		sub2.MarkActive()
		err = repo.Create(ctx, sub2)
		require.NoError(t, err)

		sub3 := models.NewSubscription("UCtest3", 432000)
		sub3.MarkFailed()
		err = repo.Create(ctx, sub3)
		require.NoError(t, err)

		// Get pending subscriptions
		pending, err := repo.GetByStatus(ctx, models.StatusPending, 10)
		require.NoError(t, err)
		assert.Len(t, pending, 1)
		assert.Equal(t, sub1.ID, pending[0].ID)

		// Get active subscriptions
		active, err := repo.GetByStatus(ctx, models.StatusActive, 10)
		require.NoError(t, err)
		assert.Len(t, active, 1)
		assert.Equal(t, sub2.ID, active[0].ID)

		// Get failed subscriptions
		failed, err := repo.GetByStatus(ctx, models.StatusFailed, 10)
		require.NoError(t, err)
		assert.Len(t, failed, 1)
		assert.Equal(t, sub3.ID, failed[0].ID)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		td.TruncateTables(t)

		// Create multiple pending subscriptions
		for i := 0; i < 5; i++ {
			sub := models.NewSubscription("UCtest"+string(rune('1'+i)), 432000)
			err := repo.Create(ctx, sub)
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		subscriptions, err := repo.GetByStatus(ctx, models.StatusPending, 3)
		require.NoError(t, err)
		assert.Len(t, subscriptions, 3)
	})
}
