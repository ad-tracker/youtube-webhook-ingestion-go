package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ad-tracker/youtube-webhook-ingestion/internal/db/repository"
	"ad-tracker/youtube-webhook-ingestion/internal/model"
)

// Mock enrichment job repository
type mockEnrichmentJobRepo struct {
	jobs   []*model.EnrichmentJob
	nextID int64
}

func newMockEnrichmentJobRepo() *mockEnrichmentJobRepo {
	return &mockEnrichmentJobRepo{
		jobs:   make([]*model.EnrichmentJob, 0),
		nextID: 1,
	}
}

func (m *mockEnrichmentJobRepo) CreateJob(ctx context.Context, job *model.EnrichmentJob) error {
	job.ID = m.nextID
	m.nextID++
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockEnrichmentJobRepo) GetJobByID(ctx context.Context, id int64) (*model.EnrichmentJob, error) {
	return nil, nil
}

func (m *mockEnrichmentJobRepo) GetJobByAsynqID(ctx context.Context, asynqTaskID string) (*model.EnrichmentJob, error) {
	return nil, nil
}

func (m *mockEnrichmentJobRepo) UpdateJobStatus(ctx context.Context, id int64, status string, errorMsg *string) error {
	return nil
}

func (m *mockEnrichmentJobRepo) MarkJobProcessing(ctx context.Context, id int64) error {
	return nil
}

func (m *mockEnrichmentJobRepo) MarkJobCompleted(ctx context.Context, id int64) error {
	return nil
}

func (m *mockEnrichmentJobRepo) MarkJobFailed(ctx context.Context, id int64, errorMsg string, stackTrace *string) error {
	return nil
}

func (m *mockEnrichmentJobRepo) IncrementAttempts(ctx context.Context, id int64) error {
	return nil
}

func (m *mockEnrichmentJobRepo) GetPendingJobs(ctx context.Context, limit int) ([]*model.EnrichmentJob, error) {
	return nil, nil
}

func (m *mockEnrichmentJobRepo) GetJobsByStatus(ctx context.Context, status string, limit int) ([]*model.EnrichmentJob, error) {
	return nil, nil
}

func (m *mockEnrichmentJobRepo) ListJobs(ctx context.Context, filters repository.JobFilters) ([]*model.EnrichmentJob, int, error) {
	results := make([]*model.EnrichmentJob, 0)

	// Filter by status if specified
	for _, job := range m.jobs {
		if filters.Status == "" || job.Status == filters.Status {
			results = append(results, job)
		}
	}

	total := len(results)

	// Apply pagination
	start := filters.Offset
	end := filters.Offset + filters.Limit

	if start >= len(results) {
		return []*model.EnrichmentJob{}, total, nil
	}

	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

func (m *mockEnrichmentJobRepo) GetJobStats(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

func TestEnrichmentJobHandler_List(t *testing.T) {
	repo := newMockEnrichmentJobRepo()
	handler := NewEnrichmentJobHandler(repo, nil)

	// Create test jobs with different statuses
	now := time.Now()
	testJobs := []*model.EnrichmentJob{
		{
			JobType:     "video_enrichment",
			VideoID:     "video1",
			Status:      "pending",
			Priority:    1,
			ScheduledAt: now,
			Attempts:    0,
			MaxAttempts: 3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			JobType:     "video_enrichment",
			VideoID:     "video2",
			Status:      "processing",
			Priority:    2,
			ScheduledAt: now,
			Attempts:    1,
			MaxAttempts: 3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			JobType:     "video_enrichment",
			VideoID:     "video3",
			Status:      "completed",
			Priority:    1,
			ScheduledAt: now,
			Attempts:    1,
			MaxAttempts: 3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			JobType:     "video_enrichment",
			VideoID:     "video4",
			Status:      "pending",
			Priority:    1,
			ScheduledAt: now,
			Attempts:    0,
			MaxAttempts: 3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			JobType:     "video_enrichment",
			VideoID:     "video5",
			Status:      "failed",
			Priority:    1,
			ScheduledAt: now,
			Attempts:    3,
			MaxAttempts: 3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	for _, job := range testJobs {
		repo.CreateJob(context.Background(), job)
	}

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
		checkResponse  func(t *testing.T, jobs []*model.EnrichmentJob)
	}{
		{
			name:           "list all jobs with default limit",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 5 {
					t.Errorf("expected 5 jobs, got %d", len(jobs))
				}
			},
		},
		{
			name:           "filter by pending status",
			query:          "?status=pending",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 2 {
					t.Errorf("expected 2 pending jobs, got %d", len(jobs))
				}
				for _, job := range jobs {
					if job.Status != "pending" {
						t.Errorf("expected status 'pending', got '%s'", job.Status)
					}
				}
			},
		},
		{
			name:           "filter by completed status",
			query:          "?status=completed",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 1 {
					t.Errorf("expected 1 completed job, got %d", len(jobs))
				}
				if len(jobs) > 0 && jobs[0].Status != "completed" {
					t.Errorf("expected status 'completed', got '%s'", jobs[0].Status)
				}
			},
		},
		{
			name:           "with custom limit",
			query:          "?limit=3",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 3 {
					t.Errorf("expected 3 jobs, got %d", len(jobs))
				}
			},
		},
		{
			name:           "with offset",
			query:          "?limit=2&offset=2",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 2 {
					t.Errorf("expected 2 jobs, got %d", len(jobs))
				}
			},
		},
		{
			name:           "offset beyond results",
			query:          "?offset=10",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				if len(jobs) != 0 {
					t.Errorf("expected 0 jobs, got %d", len(jobs))
				}
			},
		},
		{
			name:           "max limit enforcement (501 requested, should cap at 500)",
			query:          "?limit=501",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				// All 5 jobs should be returned since 500 > 5
				if len(jobs) != 5 {
					t.Errorf("expected 5 jobs, got %d", len(jobs))
				}
			},
		},
		{
			name:           "invalid limit (negative)",
			query:          "?limit=-1",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				// Should default to 100, returning all 5 jobs
				if len(jobs) != 5 {
					t.Errorf("expected 5 jobs with default limit, got %d", len(jobs))
				}
			},
		},
		{
			name:           "invalid limit (non-numeric)",
			query:          "?limit=invalid",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			checkResponse: func(t *testing.T, jobs []*model.EnrichmentJob) {
				// Should default to 100, returning all 5 jobs
				if len(jobs) != 5 {
					t.Errorf("expected 5 jobs with default limit, got %d", len(jobs))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs"+tt.query, nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				// Convert items from interface{} to []*model.EnrichmentJob
				itemsInterface := response["items"].([]interface{})
				jobs := make([]*model.EnrichmentJob, len(itemsInterface))
				for i, item := range itemsInterface {
					itemBytes, _ := json.Marshal(item)
					job := &model.EnrichmentJob{}
					json.Unmarshal(itemBytes, job)
					jobs[i] = job
				}

				if tt.checkResponse != nil {
					tt.checkResponse(t, jobs)
				}
			}
		})
	}
}

func TestEnrichmentJobHandler_MethodNotAllowed(t *testing.T) {
	repo := newMockEnrichmentJobRepo()
	handler := NewEnrichmentJobHandler(repo, nil)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/jobs", nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			if resp.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
			}
		})
	}
}

func TestEnrichmentJobHandler_EmptyResults(t *testing.T) {
	repo := newMockEnrichmentJobRepo()
	handler := NewEnrichmentJobHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	items := response["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(items))
	}
}

func TestEnrichmentJobHandler_StatusFilterNoMatch(t *testing.T) {
	repo := newMockEnrichmentJobRepo()
	handler := NewEnrichmentJobHandler(repo, nil)

	// Create a job with 'pending' status
	now := time.Now()
	job := &model.EnrichmentJob{
		JobType:     "video_enrichment",
		VideoID:     "video1",
		Status:      "pending",
		Priority:    1,
		ScheduledAt: now,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.CreateJob(context.Background(), job)

	// Query for 'archived' status (which doesn't exist)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?status=archived", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	items := response["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 jobs for non-matching status, got %d", len(items))
	}
}
