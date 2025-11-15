package repository

import (
	"context"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookEventRepository_CreateWebhookEvent(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewWebhookEventRepository(td.Pool)
	ctx := context.Background()

	t.Run("creates event successfully", func(t *testing.T) {
		td.TruncateTables(t)

		rawXML := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Test Video</title></entry></feed>`
		event, err := repo.CreateWebhookEvent(ctx, rawXML, "testVideoID", "testChannelID")

		require.NoError(t, err)
		assert.NotZero(t, event.ID)
		assert.Equal(t, rawXML, event.RawXML)
		assert.NotEmpty(t, event.ContentHash)
		assert.False(t, event.Processed)
		assert.Equal(t, "testVideoID", event.VideoID.String)
		assert.Equal(t, "testChannelID", event.ChannelID.String)
	})

	t.Run("prevents duplicate content hash", func(t *testing.T) {
		td.TruncateTables(t)

		rawXML := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Test Video</title></entry></feed>`

		// Create first event
		_, err := repo.CreateWebhookEvent(ctx, rawXML, "testVideoID", "testChannelID")
		require.NoError(t, err)

		// Try to create duplicate
		_, err = repo.CreateWebhookEvent(ctx, rawXML, "testVideoID", "testChannelID")
		require.Error(t, err)
		assert.True(t, db.IsDuplicateKey(err))
	})
}

func TestWebhookEventRepository_GetUnprocessedEvents(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewWebhookEventRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves unprocessed events in order", func(t *testing.T) {
		td.TruncateTables(t)

		// Create multiple events
		_, err := repo.CreateWebhookEvent(ctx, "<feed>1</feed>", "video1", "channel1")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		event2, err := repo.CreateWebhookEvent(ctx, "<feed>2</feed>", "video2", "channel1")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		_, err = repo.CreateWebhookEvent(ctx, "<feed>3</feed>", "video3", "channel1")
		require.NoError(t, err)

		// Mark second event as processed
		err = repo.MarkEventProcessed(ctx, event2.ID, "")
		require.NoError(t, err)

		// Get unprocessed events
		events, err := repo.GetUnprocessedEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, events, 2)
		assert.Equal(t, "<feed>1</feed>", events[0].RawXML)
		assert.Equal(t, "<feed>3</feed>", events[1].RawXML)
	})

	t.Run("respects limit", func(t *testing.T) {
		td.TruncateTables(t)

		for i := 0; i < 5; i++ {
			_, err := repo.CreateWebhookEvent(ctx, "<feed>test"+string(rune('0'+i))+"</feed>", "video", "channel")
			require.NoError(t, err)
			time.Sleep(5 * time.Millisecond)
		}

		events, err := repo.GetUnprocessedEvents(ctx, 3)
		require.NoError(t, err)
		assert.Len(t, events, 3)
	})
}

func TestWebhookEventRepository_MarkEventProcessed(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewWebhookEventRepository(td.Pool)
	ctx := context.Background()

	t.Run("marks event as processed without error", func(t *testing.T) {
		td.TruncateTables(t)

		event, err := repo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video1", "channel1")
		require.NoError(t, err)
		assert.False(t, event.Processed)

		err = repo.MarkEventProcessed(ctx, event.ID, "")
		require.NoError(t, err)

		// Verify event is marked as processed
		retrieved, err := repo.GetEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.True(t, retrieved.Processed)
		assert.True(t, retrieved.ProcessedAt.Valid)
		assert.False(t, retrieved.ProcessingError.Valid)
	})

	t.Run("marks event as processed with error", func(t *testing.T) {
		td.TruncateTables(t)

		event, err := repo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video1", "channel1")
		require.NoError(t, err)

		processingError := "failed to parse XML"
		err = repo.MarkEventProcessed(ctx, event.ID, processingError)
		require.NoError(t, err)

		// Verify error is recorded
		retrieved, err := repo.GetEventByID(ctx, event.ID)
		require.NoError(t, err)
		assert.True(t, retrieved.Processed)
		assert.True(t, retrieved.ProcessingError.Valid)
		assert.Equal(t, processingError, retrieved.ProcessingError.String)
	})

	t.Run("returns error for non-existent event", func(t *testing.T) {
		td.TruncateTables(t)

		err := repo.MarkEventProcessed(ctx, 99999, "")
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestWebhookEventRepository_GetEventByID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewWebhookEventRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves event successfully", func(t *testing.T) {
		td.TruncateTables(t)

		rawXML := "<feed>test</feed>"
		created, err := repo.CreateWebhookEvent(ctx, rawXML, "video1", "channel1")
		require.NoError(t, err)

		retrieved, err := repo.GetEventByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, rawXML, retrieved.RawXML)
	})

	t.Run("returns error for non-existent event", func(t *testing.T) {
		td.TruncateTables(t)

		_, err := repo.GetEventByID(ctx, 99999)
		require.Error(t, err)
		assert.True(t, db.IsNotFound(err))
	})
}

func TestWebhookEventRepository_GetEventsByVideoID(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	repo := NewWebhookEventRepository(td.Pool)
	ctx := context.Background()

	t.Run("retrieves events for video", func(t *testing.T) {
		td.TruncateTables(t)

		// Create events for same video
		_, err := repo.CreateWebhookEvent(ctx, "<feed>1</feed>", "video1", "channel1")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		_, err = repo.CreateWebhookEvent(ctx, "<feed>2</feed>", "video1", "channel1")
		require.NoError(t, err)

		// Create event for different video
		_, err = repo.CreateWebhookEvent(ctx, "<feed>3</feed>", "video2", "channel1")
		require.NoError(t, err)

		events, err := repo.GetEventsByVideoID(ctx, "video1")
		require.NoError(t, err)
		assert.Len(t, events, 2)
		// Should be ordered by received_at DESC
		assert.Equal(t, "<feed>2</feed>", events[0].RawXML)
		assert.Equal(t, "<feed>1</feed>", events[1].RawXML)
	})
}

func TestWebhookEventRepository_ImmutabilityProtection(t *testing.T) {
	td := testutil.SetupTestDatabase(t)
	defer td.Cleanup(t)

	ctx := context.Background()

	t.Run("prevents deletion", func(t *testing.T) {
		td.TruncateTables(t)

		repo := NewWebhookEventRepository(td.Pool)
		event, err := repo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video1", "channel1")
		require.NoError(t, err)

		// Try to delete
		_, err = td.Pool.Exec(ctx, "DELETE FROM webhook_events WHERE id = $1", event.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Deleting webhook events is not allowed")
	})

	t.Run("prevents modification of immutable fields", func(t *testing.T) {
		td.TruncateTables(t)

		repo := NewWebhookEventRepository(td.Pool)
		event, err := repo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video1", "channel1")
		require.NoError(t, err)

		// Try to modify raw_xml
		_, err = td.Pool.Exec(ctx, "UPDATE webhook_events SET raw_xml = $1 WHERE id = $2", "<feed>modified</feed>", event.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Modifying immutable fields")
	})

	t.Run("allows updating processed fields", func(t *testing.T) {
		td.TruncateTables(t)

		repo := NewWebhookEventRepository(td.Pool)
		event, err := repo.CreateWebhookEvent(ctx, "<feed>test</feed>", "video1", "channel1")
		require.NoError(t, err)

		// This should succeed
		err = repo.MarkEventProcessed(ctx, event.ID, "")
		require.NoError(t, err)
	})
}

func TestWebhookEventRepository_ContentHashGeneration(t *testing.T) {
	rawXML1 := "<feed>test1</feed>"
	rawXML2 := "<feed>test2</feed>"

	hash1 := db.GenerateContentHash(rawXML1)
	hash2 := db.GenerateContentHash(rawXML2)
	hash3 := db.GenerateContentHash(rawXML1)

	assert.NotEqual(t, hash1, hash2, "different content should have different hashes")
	assert.Equal(t, hash1, hash3, "same content should have same hash")
	assert.Len(t, hash1, 64, "SHA-256 hash should be 64 characters")
}
