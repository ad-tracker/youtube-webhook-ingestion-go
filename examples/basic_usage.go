package main

import (
	"context"
	"log"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

// This example demonstrates basic usage of the database layer for
// YouTube PubSubHubbub webhook ingestion.
func main() {
	ctx := context.Background()

	// Create database connection pool
	cfg := &db.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "youtube_webhooks",
		SSLMode:  "disable",
		MaxConns: 25,
		MinConns: 5,
	}

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer db.Close(pool)

	log.Println("Connected to database successfully")

	// Create repositories
	webhookRepo := repository.NewWebhookEventRepository(pool)
	channelRepo := repository.NewChannelRepository(pool)
	videoRepo := repository.NewVideoRepository(pool)
	updateRepo := repository.NewVideoUpdateRepository(pool)

	// Example 1: Ingest a webhook event
	log.Println("\n=== Example 1: Ingest Webhook Event ===")
	rawXML := `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
		<entry>
			<id>yt:video:dQw4w9WgXcQ</id>
			<yt:videoId>dQw4w9WgXcQ</yt:videoId>
			<yt:channelId>UCuAXFkgsw1L7xaCfnd5JJOw</yt:channelId>
			<title>Example Video Title</title>
			<link rel="alternate" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ"/>
			<author>
				<name>Example Channel</name>
				<uri>https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw</uri>
			</author>
			<published>2024-01-01T12:00:00+00:00</published>
			<updated>2024-01-01T12:00:00+00:00</updated>
		</entry>
	</feed>`

	event, err := webhookRepo.CreateWebhookEvent(ctx, rawXML, "dQw4w9WgXcQ", "UCuAXFkgsw1L7xaCfnd5JJOw")
	if err != nil {
		if db.IsDuplicateKey(err) {
			log.Println("Event already exists (duplicate content hash)")
		} else {
			log.Fatalf("Failed to create webhook event: %v", err)
		}
	} else {
		log.Printf("Created webhook event with ID: %d", event.ID)
	}

	// Example 2: Process webhook events
	log.Println("\n=== Example 2: Process Unprocessed Events ===")
	unprocessed, err := webhookRepo.GetUnprocessedEvents(ctx, 10)
	if err != nil {
		log.Fatalf("Failed to get unprocessed events: %v", err)
	}

	log.Printf("Found %d unprocessed events", len(unprocessed))

	for _, evt := range unprocessed {
		log.Printf("Processing event ID %d...", evt.ID)

		// In a real application, you would parse the XML here and extract data
		// For this example, we'll just create dummy data

		// Upsert channel
		channel := models.NewChannel(
			"UCuAXFkgsw1L7xaCfnd5JJOw",
			"Example Channel",
			"https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw",
		)
		if err := channelRepo.UpsertChannel(ctx, channel); err != nil {
			log.Printf("Failed to upsert channel: %v", err)
			continue
		}

		// Upsert video
		publishedAt := time.Now().Add(-24 * time.Hour)
		video := models.NewVideo(
			"dQw4w9WgXcQ",
			"UCuAXFkgsw1L7xaCfnd5JJOw",
			"Example Video Title",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			publishedAt,
		)
		if err := videoRepo.UpsertVideo(ctx, video); err != nil {
			log.Printf("Failed to upsert video: %v", err)
			continue
		}

		// Create video update record
		update := models.NewVideoUpdate(
			evt.ID,
			"dQw4w9WgXcQ",
			"UCuAXFkgsw1L7xaCfnd5JJOw",
			"Example Video Title",
			publishedAt,
			time.Now(),
			models.UpdateTypeNewVideo,
		)
		if err := updateRepo.CreateVideoUpdate(ctx, update); err != nil {
			log.Printf("Failed to create video update: %v", err)
			continue
		}

		// Mark event as processed
		if err := webhookRepo.MarkEventProcessed(ctx, evt.ID, ""); err != nil {
			log.Printf("Failed to mark event as processed: %v", err)
			continue
		}

		log.Printf("Successfully processed event ID %d", evt.ID)
	}

	// Example 3: Query data
	log.Println("\n=== Example 3: Query Data ===")

	// Get recent videos
	videos, err := videoRepo.ListVideos(ctx, 10, 0)
	if err != nil {
		log.Fatalf("Failed to list videos: %v", err)
	}
	log.Printf("Found %d videos:", len(videos))
	for _, v := range videos {
		log.Printf("  - %s: %s (published: %s)", v.VideoID, v.Title, v.PublishedAt.Format(time.RFC3339))
	}

	// Get update history for a video
	if len(videos) > 0 {
		updates, err := updateRepo.GetUpdatesByVideoID(ctx, videos[0].VideoID, 10)
		if err != nil {
			log.Fatalf("Failed to get update history: %v", err)
		}
		log.Printf("\nUpdate history for video %s:", videos[0].VideoID)
		for _, u := range updates {
			log.Printf("  - %s: %s (type: %s)", u.CreatedAt.Format(time.RFC3339), u.Title, u.UpdateType)
		}
	}

	// Get channels
	channels, err := channelRepo.ListChannels(ctx, 10, 0)
	if err != nil {
		log.Fatalf("Failed to list channels: %v", err)
	}
	log.Printf("\nFound %d channels:", len(channels))
	for _, c := range channels {
		log.Printf("  - %s: %s (last updated: %s)", c.ChannelID, c.Title, c.LastUpdatedAt.Format(time.RFC3339))
	}

	log.Println("\n=== Examples Complete ===")
}
