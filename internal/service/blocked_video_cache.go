package service

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

const (
	blockedVideosSetKey = "blocked_videos:set"
)

// BlockedVideoCache provides efficient caching of blocked video IDs using Redis.
type BlockedVideoCache struct {
	redisClient *redis.Client
	repo        repository.BlockedVideoRepository
}

// NewBlockedVideoCache creates a new BlockedVideoCache.
func NewBlockedVideoCache(redisClient *redis.Client, repo repository.BlockedVideoRepository) *BlockedVideoCache {
	return &BlockedVideoCache{
		redisClient: redisClient,
		repo:        repo,
	}
}

// LoadFromDB loads all blocked video IDs from the database into the Redis cache.
// This should be called on application startup.
func (c *BlockedVideoCache) LoadFromDB(ctx context.Context) error {
	videoIDs, err := c.repo.GetAllBlockedVideoIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to load blocked video IDs from database: %w", err)
	}

	if len(videoIDs) == 0 {
		log.Println("No blocked videos found in database")
		return nil
	}

	// Clear existing set and add all video IDs
	pipe := c.redisClient.Pipeline()
	pipe.Del(ctx, blockedVideosSetKey)

	// Convert []string to []interface{} for Redis SADD
	members := make([]interface{}, len(videoIDs))
	for i, id := range videoIDs {
		members[i] = id
	}
	pipe.SAdd(ctx, blockedVideosSetKey, members...)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to load blocked videos into Redis: %w", err)
	}

	log.Printf("Loaded %d blocked video IDs into cache", len(videoIDs))
	return nil
}

// IsBlocked checks if a video ID is in the blocked list.
// This is an O(1) operation using Redis SISMEMBER.
func (c *BlockedVideoCache) IsBlocked(ctx context.Context, videoID string) (bool, error) {
	result, err := c.redisClient.SIsMember(ctx, blockedVideosSetKey, videoID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if video is blocked: %w", err)
	}
	return result, nil
}

// Add adds a video ID to the blocked list cache.
// This should be called after successfully adding to the database.
func (c *BlockedVideoCache) Add(ctx context.Context, videoID string) error {
	err := c.redisClient.SAdd(ctx, blockedVideosSetKey, videoID).Err()
	if err != nil {
		return fmt.Errorf("failed to add video to blocked cache: %w", err)
	}
	return nil
}

// Remove removes a video ID from the blocked list cache.
// This should be called after successfully removing from the database.
func (c *BlockedVideoCache) Remove(ctx context.Context, videoID string) error {
	err := c.redisClient.SRem(ctx, blockedVideosSetKey, videoID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove video from blocked cache: %w", err)
	}
	return nil
}

// Sync reloads all blocked video IDs from the database, ensuring cache consistency.
// This can be called periodically or when cache inconsistency is suspected.
func (c *BlockedVideoCache) Sync(ctx context.Context) error {
	return c.LoadFromDB(ctx)
}

// GetCount returns the number of blocked videos in the cache.
func (c *BlockedVideoCache) GetCount(ctx context.Context) (int64, error) {
	count, err := c.redisClient.SCard(ctx, blockedVideosSetKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get blocked videos count: %w", err)
	}
	return count, nil
}
