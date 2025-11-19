package quota

import (
	"context"
	"fmt"
	"log"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/model"
)

// Manager handles YouTube API quota management
type Manager struct {
	repo             repository.QuotaRepository
	dailyLimit       int
	thresholdPercent int // Stop processing when this % of quota is used
}

// NewManager creates a new quota manager
func NewManager(repo repository.QuotaRepository, dailyLimit int, thresholdPercent int) *Manager {
	if dailyLimit <= 0 {
		dailyLimit = 10000 // YouTube API v3 default
	}
	if thresholdPercent <= 0 || thresholdPercent > 100 {
		thresholdPercent = 90 // Stop at 90% by default
	}

	return &Manager{
		repo:             repo,
		dailyLimit:       dailyLimit,
		thresholdPercent: thresholdPercent,
	}
}

// CheckQuotaAvailable checks if there's enough quota to proceed
// Returns true if quota is available, false otherwise
func (m *Manager) CheckQuotaAvailable(ctx context.Context, requiredQuota int) (bool, *model.QuotaInfo, error) {
	info, err := m.repo.GetTodaysQuota(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get quota info: %w", err)
	}

	// Check against threshold
	thresholdQuota := (m.dailyLimit * m.thresholdPercent) / 100

	if info.QuotaUsed >= thresholdQuota {
		log.Printf("[Quota] Threshold reached: %d/%d (%.1f%%)", info.QuotaUsed, m.dailyLimit,
			float64(info.QuotaUsed)/float64(m.dailyLimit)*100)
		return false, info, nil
	}

	// Check if we have enough for this operation
	if info.QuotaUsed+requiredQuota > thresholdQuota {
		log.Printf("[Quota] Not enough quota for operation: need %d, have %d remaining (threshold %d)",
			requiredQuota, info.QuotaRemaining, thresholdQuota)
		return false, info, nil
	}

	return true, info, nil
}

// RecordQuotaUsage records API quota usage
func (m *Manager) RecordQuotaUsage(ctx context.Context, quotaCost int, operationType string) error {
	if err := m.repo.IncrementQuota(ctx, quotaCost, operationType); err != nil {
		return fmt.Errorf("failed to record quota usage: %w", err)
	}

	// Log the usage
	info, _ := m.repo.GetTodaysQuota(ctx)
	if info != nil {
		percentage := float64(info.QuotaUsed) / float64(m.dailyLimit) * 100
		log.Printf("[Quota] Used: %d/%d (%.1f%%) - Cost: %d (%s)",
			info.QuotaUsed, m.dailyLimit, percentage, quotaCost, operationType)
	}

	return nil
}

// GetQuotaInfo returns current quota information
func (m *Manager) GetQuotaInfo(ctx context.Context) (*model.QuotaInfo, error) {
	return m.repo.GetTodaysQuota(ctx)
}

// GetQuotaUsagePercentage returns the percentage of daily quota used
func (m *Manager) GetQuotaUsagePercentage(ctx context.Context) (float64, error) {
	info, err := m.repo.GetTodaysQuota(ctx)
	if err != nil {
		return 0, err
	}

	return float64(info.QuotaUsed) / float64(m.dailyLimit) * 100, nil
}

// IsQuotaExhausted checks if quota threshold has been reached
func (m *Manager) IsQuotaExhausted(ctx context.Context) (bool, error) {
	info, err := m.repo.GetTodaysQuota(ctx)
	if err != nil {
		return false, err
	}

	thresholdQuota := (m.dailyLimit * m.thresholdPercent) / 100
	return info.QuotaUsed >= thresholdQuota, nil
}

// GetRemainingQuota returns how much quota is remaining before threshold
func (m *Manager) GetRemainingQuota(ctx context.Context) (int, error) {
	info, err := m.repo.GetTodaysQuota(ctx)
	if err != nil {
		return 0, err
	}

	thresholdQuota := (m.dailyLimit * m.thresholdPercent) / 100
	remaining := thresholdQuota - info.QuotaUsed

	if remaining < 0 {
		return 0, nil
	}

	return remaining, nil
}
