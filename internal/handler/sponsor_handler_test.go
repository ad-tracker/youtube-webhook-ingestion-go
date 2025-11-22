package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db"
	"ad-tracker/youtube-webhook-ingestion/internal/db/models"
	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"

	"github.com/google/uuid"
)

// Mock sponsor detection repository for testing
type mockSponsorDetectionRepo struct {
	sponsors           map[uuid.UUID]*models.Sponsor
	videoSponsors      map[uuid.UUID]*models.VideoSponsor
	detectionJobs      map[uuid.UUID]*models.SponsorDetectionJob
	videoSponsorsByVid map[string][]*models.VideoSponsorDetail
	channelSponsors    map[string][]*models.Sponsor
}

func newMockSponsorDetectionRepo() *mockSponsorDetectionRepo {
	return &mockSponsorDetectionRepo{
		sponsors:           make(map[uuid.UUID]*models.Sponsor),
		videoSponsors:      make(map[uuid.UUID]*models.VideoSponsor),
		detectionJobs:      make(map[uuid.UUID]*models.SponsorDetectionJob),
		videoSponsorsByVid: make(map[string][]*models.VideoSponsorDetail),
		channelSponsors:    make(map[string][]*models.Sponsor),
	}
}

func (m *mockSponsorDetectionRepo) GetOrCreatePrompt(ctx context.Context, promptText, version, description string) (*models.SponsorDetectionPrompt, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) IncrementPromptUsageCount(ctx context.Context, promptID uuid.UUID) error {
	return nil
}

func (m *mockSponsorDetectionRepo) GetPromptByID(ctx context.Context, promptID uuid.UUID) (*models.SponsorDetectionPrompt, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) ListPrompts(ctx context.Context, limit, offset int) ([]*models.SponsorDetectionPrompt, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) GetSponsorByNormalizedName(ctx context.Context, normalizedName string) (*models.Sponsor, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) CreateSponsor(ctx context.Context, sponsor *models.Sponsor) error {
	return nil
}

func (m *mockSponsorDetectionRepo) UpdateSponsorLastSeen(ctx context.Context, sponsorID uuid.UUID, timestamp time.Time) error {
	return nil
}

func (m *mockSponsorDetectionRepo) IncrementSponsorVideoCount(ctx context.Context, sponsorID uuid.UUID) error {
	return nil
}

func (m *mockSponsorDetectionRepo) ListSponsors(ctx context.Context, sortBy string, order string, category string, limit, offset int) ([]*models.Sponsor, error) {
	var results []*models.Sponsor
	for _, sponsor := range m.sponsors {
		// Apply category filter if specified
		if category != "" {
			if sponsor.Category == nil || *sponsor.Category != category {
				continue
			}
		}
		results = append(results, sponsor)
	}

	// Simple pagination
	start := offset
	end := offset + limit
	if start > len(results) {
		return []*models.Sponsor{}, nil
	}
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], nil
}

func (m *mockSponsorDetectionRepo) GetSponsorByID(ctx context.Context, sponsorID uuid.UUID) (*models.Sponsor, error) {
	sponsor, ok := m.sponsors[sponsorID]
	if !ok {
		return nil, nil
	}
	return sponsor, nil
}

func (m *mockSponsorDetectionRepo) CreateDetectionJob(ctx context.Context, job *models.SponsorDetectionJob) error {
	return nil
}

func (m *mockSponsorDetectionRepo) UpdateDetectionJobStatus(ctx context.Context, jobID uuid.UUID, status string, errorMsg *string) error {
	return nil
}

func (m *mockSponsorDetectionRepo) CompleteDetectionJob(ctx context.Context, jobID uuid.UUID, promptID *uuid.UUID, llmResponse string, processingTimeMs, sponsorCount int) error {
	return nil
}

func (m *mockSponsorDetectionRepo) GetDetectionJobsByVideoID(ctx context.Context, videoID string) ([]*models.SponsorDetectionJob, error) {
	var results []*models.SponsorDetectionJob
	for _, job := range m.detectionJobs {
		if job.VideoID == videoID {
			results = append(results, job)
		}
	}
	return results, nil
}

func (m *mockSponsorDetectionRepo) GetLatestDetectionJobForVideo(ctx context.Context, videoID string) (*models.SponsorDetectionJob, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) GetDetectionJobByID(ctx context.Context, jobID uuid.UUID) (*models.SponsorDetectionJob, error) {
	job, ok := m.detectionJobs[jobID]
	if !ok {
		return nil, nil
	}
	return job, nil
}

func (m *mockSponsorDetectionRepo) CreateVideoSponsor(ctx context.Context, videoSponsor *models.VideoSponsor) error {
	return nil
}

func (m *mockSponsorDetectionRepo) GetVideoSponsorsWithDetails(ctx context.Context, videoID string) ([]*models.VideoSponsorDetail, error) {
	details, ok := m.videoSponsorsByVid[videoID]
	if !ok {
		return []*models.VideoSponsorDetail{}, nil
	}
	return details, nil
}

func (m *mockSponsorDetectionRepo) GetSponsorVideos(ctx context.Context, sponsorID uuid.UUID, limit, offset int) ([]*models.VideoSponsor, error) {
	var results []*models.VideoSponsor
	for _, vs := range m.videoSponsors {
		if vs.SponsorID == sponsorID {
			results = append(results, vs)
		}
	}

	// Simple pagination
	start := offset
	end := offset + limit
	if start > len(results) {
		return []*models.VideoSponsor{}, nil
	}
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], nil
}

func (m *mockSponsorDetectionRepo) GetVideoSponsorsByJobID(ctx context.Context, jobID uuid.UUID) ([]*models.VideoSponsor, error) {
	return nil, nil
}

func (m *mockSponsorDetectionRepo) GetSponsorsByChannelID(ctx context.Context, channelID string, limit, offset int) ([]*models.Sponsor, error) {
	sponsors, ok := m.channelSponsors[channelID]
	if !ok {
		return []*models.Sponsor{}, nil
	}

	// Simple pagination
	start := offset
	end := offset + limit
	if start > len(sponsors) {
		return []*models.Sponsor{}, nil
	}
	if end > len(sponsors) {
		end = len(sponsors)
	}

	return sponsors[start:end], nil
}

func (m *mockSponsorDetectionRepo) SaveDetectionResults(ctx context.Context, jobID uuid.UUID, videoID string, promptID *uuid.UUID, llmResults []models.LLMSponsorResult, llmRawResponse string, processingTimeMs int) error {
	return nil
}

// Mock video repository for testing
type mockVideoRepo struct {
	videos map[string]*models.Video
}

func newMockVideoRepo() *mockVideoRepo {
	return &mockVideoRepo{
		videos: make(map[string]*models.Video),
	}
}

func (m *mockVideoRepo) Create(ctx context.Context, video *models.Video) error {
	return nil
}

func (m *mockVideoRepo) GetVideoByID(ctx context.Context, videoID string) (*models.Video, error) {
	video, ok := m.videos[videoID]
	if !ok {
		return nil, db.ErrNotFound
	}
	return video, nil
}

func (m *mockVideoRepo) List(ctx context.Context, filters *repository.VideoFilters) ([]*models.Video, int, error) {
	return nil, 0, nil
}

func (m *mockVideoRepo) Update(ctx context.Context, video *models.Video) error {
	return nil
}

func (m *mockVideoRepo) Delete(ctx context.Context, videoID string) error {
	return nil
}

func (m *mockVideoRepo) UpsertVideo(ctx context.Context, video *models.Video) error {
	return nil
}

func (m *mockVideoRepo) GetVideosByChannelID(ctx context.Context, channelID string, limit int) ([]*models.Video, error) {
	return nil, nil
}

func (m *mockVideoRepo) GetVideosByPublishedDate(ctx context.Context, since time.Time, limit int) ([]*models.Video, error) {
	return nil, nil
}

func (m *mockVideoRepo) ListVideos(ctx context.Context, limit, offset int) ([]*models.Video, error) {
	return nil, nil
}

// Tests for SponsorHandler

func TestSponsorHandler_ListSponsors(t *testing.T) {
	repo := newMockSponsorDetectionRepo()
	videoRepo := newMockVideoRepo()

	// Add test data
	sponsor1 := &models.Sponsor{
		ID:             uuid.New(),
		Name:           "NordVPN",
		NormalizedName: "nordvpn",
		VideoCount:     10,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	sponsor2 := &models.Sponsor{
		ID:             uuid.New(),
		Name:           "Brilliant",
		NormalizedName: "brilliant",
		VideoCount:     5,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	repo.sponsors[sponsor1.ID] = sponsor1
	repo.sponsors[sponsor2.ID] = sponsor2

	handler := NewSponsorHandler(repo, videoRepo, nil)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "list sponsors default params",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				items, ok := response["items"].([]interface{})
				if !ok {
					t.Fatal("items field missing or invalid")
				}

				if len(items) != 2 {
					t.Errorf("expected 2 sponsors, got %d", len(items))
				}
			},
		},
		{
			name:           "list sponsors with limit",
			queryParams:    "?limit=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				items, ok := response["items"].([]interface{})
				if !ok {
					t.Fatal("items field missing or invalid")
				}

				if len(items) != 1 {
					t.Errorf("expected 1 sponsor, got %d", len(items))
				}
			},
		},
		{
			name:           "invalid sort_by parameter",
			queryParams:    "?sort_by=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid order parameter",
			queryParams:    "?order=invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsors"+tt.queryParams, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestSponsorHandler_GetSponsor(t *testing.T) {
	repo := newMockSponsorDetectionRepo()
	videoRepo := newMockVideoRepo()

	sponsorID := uuid.New()
	sponsor := &models.Sponsor{
		ID:             sponsorID,
		Name:           "NordVPN",
		NormalizedName: "nordvpn",
		VideoCount:     10,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.sponsors[sponsorID] = sponsor

	handler := NewSponsorHandler(repo, videoRepo, nil)

	tests := []struct {
		name           string
		sponsorID      string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get existing sponsor",
			sponsorID:      sponsorID.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result models.Sponsor
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if result.ID != sponsorID {
					t.Errorf("expected sponsor ID %s, got %s", sponsorID, result.ID)
				}

				if result.Name != "NordVPN" {
					t.Errorf("expected sponsor name 'NordVPN', got '%s'", result.Name)
				}
			},
		},
		{
			name:           "sponsor not found",
			sponsorID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid sponsor ID format",
			sponsorID:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsors/"+tt.sponsorID, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestSponsorHandler_GetSponsorVideos(t *testing.T) {
	repo := newMockSponsorDetectionRepo()
	videoRepo := newMockVideoRepo()

	sponsorID := uuid.New()
	sponsor := &models.Sponsor{
		ID:             sponsorID,
		Name:           "NordVPN",
		NormalizedName: "nordvpn",
		VideoCount:     2,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.sponsors[sponsorID] = sponsor

	// Add video sponsor relationships
	vs1ID := uuid.New()
	videoSponsor1 := &models.VideoSponsor{
		ID:             vs1ID,
		VideoID:        "video1",
		SponsorID:      sponsorID,
		DetectionJobID: uuid.New(),
		Confidence:     0.95,
		Evidence:       "Sponsored segment detected",
		DetectedAt:     time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.videoSponsors[vs1ID] = videoSponsor1

	// Add corresponding video
	video1 := &models.Video{
		VideoID:     "video1",
		ChannelID:   "UCtest123",
		Title:       "Test Video 1",
		VideoURL:    "https://youtube.com/watch?v=video1",
		PublishedAt: time.Now(),
	}
	videoRepo.videos["video1"] = video1

	handler := NewSponsorHandler(repo, videoRepo, nil)

	tests := []struct {
		name           string
		sponsorID      string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get sponsor videos",
			sponsorID:      sponsorID.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				items, ok := response["items"].([]interface{})
				if !ok {
					t.Fatal("items field missing or invalid")
				}

				if len(items) != 1 {
					t.Errorf("expected 1 video sponsor, got %d", len(items))
				}

				firstItem := items[0].(map[string]interface{})
				if firstItem["video_id"] != "video1" {
					t.Errorf("expected video_id 'video1', got '%v'", firstItem["video_id"])
				}
			},
		},
		{
			name:           "sponsor not found",
			sponsorID:      uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsors/"+tt.sponsorID+"/videos", nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestVideoSponsorHandler_GetVideoSponsors(t *testing.T) {
	repo := newMockSponsorDetectionRepo()

	videoID := "test-video-1"
	sponsorName := "NordVPN"
	category := "VPN"

	// Add video sponsor details
	detail := &models.VideoSponsorDetail{
		VideoSponsor: models.VideoSponsor{
			ID:             uuid.New(),
			VideoID:        videoID,
			SponsorID:      uuid.New(),
			DetectionJobID: uuid.New(),
			Confidence:     0.95,
			Evidence:       "Sponsored segment detected",
			DetectedAt:     time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		SponsorName:     sponsorName,
		SponsorCategory: &category,
	}
	repo.videoSponsorsByVid[videoID] = []*models.VideoSponsorDetail{detail}

	handler := NewVideoSponsorHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+videoID+"/sponsors", nil)
	resp := httptest.NewRecorder()

	handler.HandleGetVideoSponsors(resp, req, videoID)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	items, ok := response["items"].([]interface{})
	if !ok {
		t.Fatal("items field missing or invalid")
	}

	if len(items) != 1 {
		t.Errorf("expected 1 video sponsor, got %d", len(items))
	}
}

func TestChannelSponsorHandler_GetChannelSponsors(t *testing.T) {
	repo := newMockSponsorDetectionRepo()
	videoRepo := newMockVideoRepo()

	channelID := "UCtest123"
	sponsor := &models.Sponsor{
		ID:             uuid.New(),
		Name:           "NordVPN",
		NormalizedName: "nordvpn",
		VideoCount:     5,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.channelSponsors[channelID] = []*models.Sponsor{sponsor}

	handler := NewChannelSponsorHandler(repo, videoRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/channels/"+channelID+"/sponsors", nil)
	resp := httptest.NewRecorder()

	handler.HandleGetChannelSponsors(resp, req, channelID)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	items, ok := response["items"].([]interface{})
	if !ok {
		t.Fatal("items field missing or invalid")
	}

	if len(items) != 1 {
		t.Errorf("expected 1 sponsor, got %d", len(items))
	}
}

func TestSponsorDetectionJobHandler_GetJob(t *testing.T) {
	repo := newMockSponsorDetectionRepo()

	jobID := uuid.New()
	job := &models.SponsorDetectionJob{
		ID:                    jobID,
		VideoID:               "test-video",
		LLMModel:              "ollama:llama3.2",
		Status:                "completed",
		SponsorsDetectedCount: 2,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	repo.detectionJobs[jobID] = job

	handler := NewSponsorDetectionJobHandler(repo, nil)

	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get existing job",
			jobID:          jobID.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result models.SponsorDetectionJob
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if result.ID != jobID {
					t.Errorf("expected job ID %s, got %s", jobID, result.ID)
				}

				if result.Status != "completed" {
					t.Errorf("expected status 'completed', got '%s'", result.Status)
				}
			},
		},
		{
			name:           "job not found",
			jobID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid job ID format",
			jobID:          "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsor-detection-jobs/"+tt.jobID, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestSponsorDetectionJobHandler_ListJobs(t *testing.T) {
	repo := newMockSponsorDetectionRepo()

	jobID := uuid.New()
	job := &models.SponsorDetectionJob{
		ID:                    jobID,
		VideoID:               "test-video",
		LLMModel:              "ollama:llama3.2",
		Status:                "completed",
		SponsorsDetectedCount: 2,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	repo.detectionJobs[jobID] = job

	handler := NewSponsorDetectionJobHandler(repo, nil)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "list jobs by video_id",
			queryParams:    "?video_id=test-video",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				items, ok := response["items"].([]interface{})
				if !ok {
					t.Fatal("items field missing or invalid")
				}

				if len(items) != 1 {
					t.Errorf("expected 1 job, got %d", len(items))
				}
			},
		},
		{
			name:           "list jobs with status filter",
			queryParams:    "?video_id=test-video&status=completed",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				items, ok := response["items"].([]interface{})
				if !ok {
					t.Fatal("items field missing or invalid")
				}

				if len(items) != 1 {
					t.Errorf("expected 1 job, got %d", len(items))
				}
			},
		},
		{
			name:           "invalid status parameter",
			queryParams:    "?video_id=test-video&status=invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsor-detection-jobs"+tt.queryParams, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}
