package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
)

// Mock webhook event repository
type mockWebhookEventRepo struct {
	events map[int64]*models.WebhookEvent
	nextID int64
}

func newMockWebhookEventRepo() *mockWebhookEventRepo {
	return &mockWebhookEventRepo{
		events: make(map[int64]*models.WebhookEvent),
		nextID: 1,
	}
}

func (m *mockWebhookEventRepo) Create(ctx context.Context, event *models.WebhookEvent) error {
	for _, e := range m.events {
		if e.ContentHash == event.ContentHash {
			return db.ErrDuplicateKey
		}
	}

	event.ID = m.nextID
	m.nextID++
	event.ReceivedAt = time.Now()
	event.CreatedAt = time.Now()
	m.events[event.ID] = event
	return nil
}

func (m *mockWebhookEventRepo) GetEventByID(ctx context.Context, eventID int64) (*models.WebhookEvent, error) {
	event, ok := m.events[eventID]
	if !ok {
		return nil, db.ErrNotFound
	}
	return event, nil
}

func (m *mockWebhookEventRepo) List(ctx context.Context, filters *repository.WebhookEventFilters) ([]*models.WebhookEvent, int, error) {
	var results []*models.WebhookEvent
	for _, event := range m.events {
		include := true

		if filters.Processed != nil && event.Processed != *filters.Processed {
			include = false
		}

		if filters.VideoID != "" && (!event.VideoID.Valid || event.VideoID.String != filters.VideoID) {
			include = false
		}

		if filters.ChannelID != "" && (!event.ChannelID.Valid || event.ChannelID.String != filters.ChannelID) {
			include = false
		}

		if include {
			results = append(results, event)
		}
	}

	start := filters.Offset
	end := filters.Offset + filters.Limit
	if start > len(results) {
		return []*models.WebhookEvent{}, len(results), nil
	}
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], len(results), nil
}

func (m *mockWebhookEventRepo) UpdateProcessingStatus(ctx context.Context, eventID int64, processed bool, processingError string) error {
	event, ok := m.events[eventID]
	if !ok {
		return db.ErrNotFound
	}

	event.Processed = processed
	if processed {
		event.ProcessedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	if processingError != "" {
		event.ProcessingError = sql.NullString{String: processingError, Valid: true}
	}

	return nil
}

func (m *mockWebhookEventRepo) CreateWebhookEvent(ctx context.Context, rawXML, videoID, channelID string) (*models.WebhookEvent, error) {
	return nil, nil
}

func (m *mockWebhookEventRepo) GetUnprocessedEvents(ctx context.Context, limit int) ([]*models.WebhookEvent, error) {
	return nil, nil
}

func (m *mockWebhookEventRepo) MarkEventProcessed(ctx context.Context, eventID int64, processingError string) error {
	return nil
}

func (m *mockWebhookEventRepo) GetEventsByVideoID(ctx context.Context, videoID string) ([]*models.WebhookEvent, error) {
	return nil, nil
}

func TestWebhookEventHandler_Create(t *testing.T) {
	repo := newMockWebhookEventRepo()
	handler := NewWebhookEventHandler(repo, nil)

	tests := []struct {
		name           string
		body           CreateWebhookEventRequest
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "valid request",
			body: CreateWebhookEventRequest{
				RawXML:      "<feed>test</feed>",
				ContentHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				VideoID:     "test-video",
				ChannelID:   "UCtest123456789012345678",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var event models.WebhookEvent
				if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if event.ID == 0 {
					t.Error("expected event ID to be set")
				}

				if event.RawXML != "<feed>test</feed>" {
					t.Errorf("expected raw_xml '<feed>test</feed>', got '%s'", event.RawXML)
				}
			},
		},
		{
			name: "missing raw_xml",
			body: CreateWebhookEventRequest{
				ContentHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing content_hash",
			body: CreateWebhookEventRequest{
				RawXML: "<feed>test</feed>",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid content_hash length",
			body: CreateWebhookEventRequest{
				RawXML:      "<feed>test</feed>",
				ContentHash: "short",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate content_hash",
			body: CreateWebhookEventRequest{
				RawXML:      "<feed>duplicate</feed>",
				ContentHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook-events", bytes.NewReader(body))
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestWebhookEventHandler_Get(t *testing.T) {
	repo := newMockWebhookEventRepo()
	handler := NewWebhookEventHandler(repo, nil)

	event := &models.WebhookEvent{
		RawXML:      "<feed>test</feed>",
		ContentHash: "testhash0123456789abcdef0123456789abcdef0123456789abcdef01234567",
		ReceivedAt:  time.Now(),
		Processed:   false,
		VideoID:     sql.NullString{String: "test-video", Valid: true},
		ChannelID:   sql.NullString{String: "UCtest123456789012345678", Valid: true},
		CreatedAt:   time.Now(),
	}
	repo.Create(context.Background(), event)

	tests := []struct {
		name           string
		eventID        string
		expectedStatus int
	}{
		{
			name:           "existing event",
			eventID:        "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent event",
			eventID:        "999",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid event ID",
			eventID:        "invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/webhook-events/"+tt.eventID, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}
		})
	}
}

func TestWebhookEventHandler_List(t *testing.T) {
	repo := newMockWebhookEventRepo()
	handler := NewWebhookEventHandler(repo, nil)

	for i := 0; i < 5; i++ {
		event := &models.WebhookEvent{
			RawXML:      "<feed>test</feed>",
			ContentHash: string(rune(i)) + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
			ReceivedAt:  time.Now(),
			Processed:   i%2 == 0,
			VideoID:     sql.NullString{String: "test-video", Valid: true},
			ChannelID:   sql.NullString{String: "UCtest123456789012345678", Valid: true},
			CreatedAt:   time.Now(),
		}
		repo.Create(context.Background(), event)
	}

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "list all",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
		},
		{
			name:           "filter by processed=true",
			query:          "?processed=true",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "filter by processed=false",
			query:          "?processed=false",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "with pagination",
			query:          "?limit=2&offset=0",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/webhook-events"+tt.query, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			events := result["webhook_events"].([]interface{})
			if len(events) != tt.expectedCount {
				t.Errorf("expected %d events, got %d", tt.expectedCount, len(events))
			}
		})
	}
}

func TestWebhookEventHandler_Update(t *testing.T) {
	repo := newMockWebhookEventRepo()
	handler := NewWebhookEventHandler(repo, nil)

	event := &models.WebhookEvent{
		RawXML:      "<feed>test</feed>",
		ContentHash: "testhash0123456789abcdef0123456789abcdef0123456789abcdef01234567",
		ReceivedAt:  time.Now(),
		Processed:   false,
		VideoID:     sql.NullString{String: "test-video", Valid: true},
		ChannelID:   sql.NullString{String: "UCtest123456789012345678", Valid: true},
		CreatedAt:   time.Now(),
	}
	repo.Create(context.Background(), event)

	processed := true
	updateReq := UpdateWebhookEventRequest{
		Processed: &processed,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/webhook-events/1", bytes.NewReader(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var updatedEvent models.WebhookEvent
	if err := json.NewDecoder(resp.Body).Decode(&updatedEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !updatedEvent.Processed {
		t.Error("expected event to be marked as processed")
	}
}

func TestWebhookEventHandler_Delete(t *testing.T) {
	repo := newMockWebhookEventRepo()
	handler := NewWebhookEventHandler(repo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhook-events/1", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Errorf("expected status %d (forbidden), got %d", http.StatusForbidden, resp.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errorResp.Error != "Forbidden" {
		t.Errorf("expected error 'Forbidden', got '%s'", errorResp.Error)
	}
}

// Mock channel repository for testing
type mockChannelRepo struct {
	channels map[string]*models.Channel
}

func newMockChannelRepo() *mockChannelRepo {
	return &mockChannelRepo{
		channels: make(map[string]*models.Channel),
	}
}

func (m *mockChannelRepo) Create(ctx context.Context, channel *models.Channel) error {
	if _, exists := m.channels[channel.ChannelID]; exists {
		return db.ErrDuplicateKey
	}
	m.channels[channel.ChannelID] = channel
	return nil
}

func (m *mockChannelRepo) GetChannelByID(ctx context.Context, channelID string) (*models.Channel, error) {
	channel, ok := m.channels[channelID]
	if !ok {
		return nil, db.ErrNotFound
	}
	return channel, nil
}

func (m *mockChannelRepo) Update(ctx context.Context, channel *models.Channel) error {
	if _, ok := m.channels[channel.ChannelID]; !ok {
		return db.ErrNotFound
	}
	m.channels[channel.ChannelID] = channel
	return nil
}

func (m *mockChannelRepo) Delete(ctx context.Context, channelID string) error {
	if _, ok := m.channels[channelID]; !ok {
		return db.ErrNotFound
	}
	delete(m.channels, channelID)
	return nil
}

func (m *mockChannelRepo) List(ctx context.Context, filters *repository.ChannelFilters) ([]*models.Channel, int, error) {
	var results []*models.Channel
	for _, channel := range m.channels {
		results = append(results, channel)
	}

	start := filters.Offset
	end := filters.Offset + filters.Limit
	if start > len(results) {
		return []*models.Channel{}, len(results), nil
	}
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], len(results), nil
}

func (m *mockChannelRepo) UpsertChannel(ctx context.Context, channel *models.Channel) error {
	return nil
}

func (m *mockChannelRepo) ListChannels(ctx context.Context, limit, offset int) ([]*models.Channel, error) {
	return nil, nil
}

func (m *mockChannelRepo) GetChannelsByLastUpdated(ctx context.Context, since time.Time, limit int) ([]*models.Channel, error) {
	return nil, nil
}

func TestChannelHandler_Create(t *testing.T) {
	repo := newMockChannelRepo()
	handler := NewChannelHandler(repo, nil)

	tests := []struct {
		name           string
		body           CreateChannelRequest
		expectedStatus int
	}{
		{
			name: "valid request",
			body: CreateChannelRequest{
				ChannelID:  "UCtest123456789012345678",
				Title:      "Test Channel",
				ChannelURL: "https://www.youtube.com/channel/UCtest123456789012345678",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing channel_id",
			body: CreateChannelRequest{
				Title:      "Test Channel",
				ChannelURL: "https://www.youtube.com/channel/UCtest123456789012345678",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid channel_id format",
			body: CreateChannelRequest{
				ChannelID:  "invalid",
				Title:      "Test Channel",
				ChannelURL: "https://www.youtube.com/channel/invalid",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate channel",
			body: CreateChannelRequest{
				ChannelID:  "UCtest123456789012345678",
				Title:      "Duplicate Channel",
				ChannelURL: "https://www.youtube.com/channel/UCtest123456789012345678",
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/channels", bytes.NewReader(body))
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}
		})
	}
}
