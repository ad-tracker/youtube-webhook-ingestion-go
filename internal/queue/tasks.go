package queue

import (
	"encoding/json"
	"fmt"
)

// Task types
const (
	TypeEnrichVideo   = "enrichment:video"
	TypeEnrichChannel = "enrichment:channel"
)

// EnrichVideoPayload is the payload for video enrichment tasks
type EnrichVideoPayload struct {
	VideoID   string                 `json:"video_id"`
	ChannelID string                 `json:"channel_id"`
	Priority  int                    `json:"priority"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// NewEnrichVideoTask creates a new video enrichment task payload
func NewEnrichVideoTask(videoID, channelID string, priority int, metadata map[string]interface{}) (*EnrichVideoPayload, error) {
	if videoID == "" {
		return nil, fmt.Errorf("video ID is required")
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &EnrichVideoPayload{
		VideoID:   videoID,
		ChannelID: channelID,
		Priority:  priority,
		Metadata:  metadata,
	}, nil
}

// Marshal serializes the payload to JSON
func (p *EnrichVideoPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalEnrichVideoPayload deserializes JSON to payload
func UnmarshalEnrichVideoPayload(data []byte) (*EnrichVideoPayload, error) {
	var payload EnrichVideoPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return &payload, nil
}

// EnrichChannelPayload is the payload for channel enrichment tasks
type EnrichChannelPayload struct {
	ChannelID string                 `json:"channel_id"`
	Priority  int                    `json:"priority"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// NewEnrichChannelTask creates a new channel enrichment task payload
func NewEnrichChannelTask(channelID string, priority int, metadata map[string]interface{}) (*EnrichChannelPayload, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel ID is required")
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &EnrichChannelPayload{
		ChannelID: channelID,
		Priority:  priority,
		Metadata:  metadata,
	}, nil
}

// Marshal serializes the payload to JSON
func (p *EnrichChannelPayload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalEnrichChannelPayload deserializes JSON to payload
func UnmarshalEnrichChannelPayload(data []byte) (*EnrichChannelPayload, error) {
	var payload EnrichChannelPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return &payload, nil
}
