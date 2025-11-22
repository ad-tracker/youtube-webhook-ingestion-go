package queue

import (
	"context"
	"log"
	"sync"

	"ad-tracker/youtube-webhook-ingestion/internal/model"
)

// EnrichmentCallback is a function that gets called after successful video enrichment
type EnrichmentCallback func(ctx context.Context, videoID, channelID string, enrichment *model.VideoEnrichment) error

// CallbackManager manages enrichment callbacks
type CallbackManager struct {
	callbacks []EnrichmentCallback
	mu        sync.RWMutex
}

// NewCallbackManager creates a new callback manager
func NewCallbackManager() *CallbackManager {
	return &CallbackManager{
		callbacks: make([]EnrichmentCallback, 0),
	}
}

// RegisterCallback registers a new callback to be called after enrichment
func (m *CallbackManager) RegisterCallback(cb EnrichmentCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

// TriggerCallbacks executes all registered callbacks after enrichment completion
// Callbacks are executed sequentially. If a callback fails, it's logged but doesn't stop other callbacks.
func (m *CallbackManager) TriggerCallbacks(ctx context.Context, videoID, channelID string, enrichment *model.VideoEnrichment) {
	m.mu.RLock()
	callbacks := make([]EnrichmentCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	for i, cb := range callbacks {
		if err := cb(ctx, videoID, channelID, enrichment); err != nil {
			log.Printf("[Callbacks] Callback %d failed for video %s: %v", i, videoID, err)
			// Continue executing other callbacks even if one fails
		}
	}
}

// CallbackCount returns the number of registered callbacks
func (m *CallbackManager) CallbackCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.callbacks)
}
