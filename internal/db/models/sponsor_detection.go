package models

import (
	"time"

	"github.com/google/uuid"
)

// SponsorDetectionPrompt represents a unique LLM prompt used for sponsor detection.
// Prompts are deduplicated by hash to avoid storing the same text multiple times.
type SponsorDetectionPrompt struct {
	ID          uuid.UUID `db:"id" json:"id"`
	PromptText  string    `db:"prompt_text" json:"prompt_text"`
	PromptHash  string    `db:"prompt_hash" json:"prompt_hash"`
	Version     *string   `db:"version" json:"version,omitempty"`
	Description *string   `db:"description" json:"description,omitempty"`
	UsageCount  int       `db:"usage_count" json:"usage_count"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// Sponsor represents a brand or sponsor detected in video content.
// Sponsors are normalized to prevent duplicates (e.g., "NordVPN" vs "nordvpn").
type Sponsor struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	NormalizedName string    `db:"normalized_name" json:"normalized_name"`
	Category       *string   `db:"category" json:"category,omitempty"`
	WebsiteURL     *string   `db:"website_url" json:"website_url,omitempty"`
	Description    *string   `db:"description" json:"description,omitempty"`
	FirstSeenAt    time.Time `db:"first_seen_at" json:"first_seen_at"`
	LastSeenAt     time.Time `db:"last_seen_at" json:"last_seen_at"`
	VideoCount     int       `db:"video_count" json:"video_count"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// SponsorDetectionJob tracks a single LLM analysis run for a video.
type SponsorDetectionJob struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	VideoID               string     `db:"video_id" json:"video_id"`
	PromptID              *uuid.UUID `db:"prompt_id" json:"prompt_id,omitempty"`
	LLMModel              string     `db:"llm_model" json:"llm_model"`
	LLMResponseRaw        *string    `db:"llm_response_raw" json:"llm_response_raw,omitempty"`
	SponsorsDetectedCount int        `db:"sponsors_detected_count" json:"sponsors_detected_count"`
	ProcessingTimeMs      *int       `db:"processing_time_ms" json:"processing_time_ms,omitempty"`
	Status                string     `db:"status" json:"status"` // 'pending', 'completed', 'failed', 'skipped'
	ErrorMessage          *string    `db:"error_message" json:"error_message,omitempty"`
	DetectedAt            *time.Time `db:"detected_at" json:"detected_at,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

// VideoSponsor represents the many-to-many relationship between videos and sponsors.
type VideoSponsor struct {
	ID             uuid.UUID `db:"id" json:"id"`
	VideoID        string    `db:"video_id" json:"video_id"`
	SponsorID      uuid.UUID `db:"sponsor_id" json:"sponsor_id"`
	DetectionJobID uuid.UUID `db:"detection_job_id" json:"detection_job_id"`
	Confidence     float64   `db:"confidence" json:"confidence"`
	Evidence       string    `db:"evidence" json:"evidence"`
	DetectedAt     time.Time `db:"detected_at" json:"detected_at"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// VideoSponsorDetail is a JOIN view that includes sponsor information with the relationship.
type VideoSponsorDetail struct {
	VideoSponsor
	SponsorName     string  `db:"sponsor_name" json:"sponsor_name"`
	SponsorCategory *string `db:"sponsor_category" json:"sponsor_category,omitempty"`
}

// LLMSponsorResult represents a single sponsor detection result from the LLM.
// This is used for parsing the JSON response from Ollama.
type LLMSponsorResult struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Evidence   string  `json:"evidence"`
}

// LLMAnalysisResponse represents the complete JSON response from the LLM.
type LLMAnalysisResponse struct {
	Sponsors []LLMSponsorResult `json:"sponsors"`
}
