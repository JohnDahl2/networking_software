package api

import (
	"context"
	"testing"
	"time"
	"encoding/json"
	"fmt"

	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go-sniffer/internal/storage"
)

type fakeJobStore struct {
	jobs []storage.JobRow
	job *storage.JobRow
	err  error
}

func (f *fakeJobStore) GetAllJobs(ctx context.Context) ([]storage.JobRow, error) {
	return f.jobs, f.err
}

func (f *fakeJobStore) GetHandleJob(ctx context.Context, jobIDStr string) (*storage.JobRow, error) {
	var id pgtype.UUID
	if err := id.Scan(jobIDStr); err != nil {
		return nil, fmt.Errorf("%w: %w", storage.ErrInvalidUUID, err)
	}
	return f.job, f.err
}

func TestJobRowToResponse(t *testing.T) {
	tests := []struct {
		name        string
		filesDone   int
		totalFiles  int
		expectedPct int
	}{
		{"zero total files returns zero pct", 0, 0, 0},
		{"half done returns 50 pct", 5, 10, 50},
		{"all done returns 100 pct", 10, 10, 100},
		{"one file done of three", 1, 3, 33},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := storage.JobRow{
				JobID:      pgtype.UUID{},
				Status:     "PROCESSING",
				StartedAt:  time.Now(),
				TotalFiles: tc.totalFiles,
				FilesDone:  tc.filesDone,
			}
			response := jobRowToResponse(row)
			if response.ProgressPct != tc.expectedPct {
				t.Errorf("got %d%%, want %d%%", response.ProgressPct, tc.expectedPct)
			}
		})
	}
}


func TestHandleListJobs(t *testing.T) {
	completedAt := time.Date(2026, 6, 6, 13, 47, 35, 0, time.UTC)
	fakeJob := storage.JobRow{
		JobID:       pgtype.UUID{Bytes: [16]byte{0xe6, 0x47, 0xdf, 0x1f, 0xd8, 0xc0, 0x40, 0xe0, 0xa7, 0x07, 0x93, 0xed, 0x08, 0x79, 0xb0, 0xbe}, Valid: true},
		Status:      "COMPLETED",
		StartedAt:   time.Date(2026, 6, 6, 13, 47, 34, 0, time.UTC),
		CompletedAt: &completedAt,
		SourceDir:   "data/dumb_data",
		TotalFiles:  5,
		FilesDone:   5,
	}

	t.Run("Succesul Run", func(t *testing.T) {
		var response []JobResponse
		store := &fakeJobStore{
			jobs: []storage.JobRow{fakeJob},
			err: nil,
		}
		s := &Server{
			Store: store,
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		rr  := httptest.NewRecorder()

		s.HandleListJobs(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		
		if len(response) != 1 {
			t.Fatalf("got %d jobs, want 1", len(response))
		}
		
		if response[0].Status != "COMPLETED" {
			t.Errorf("got status %q, want %q", response[0].Status, "COMPLETED")
		}
	})

	t.Run("db error returns 500", func(t *testing.T) {
		store := &fakeJobStore{
			jobs: nil,
			err:  fmt.Errorf("connection refused"),
		}
		s := &Server{Store: store}
	
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		rr  := httptest.NewRecorder()
	
		s.HandleListJobs(rr, req)
	
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("No data returns", func(t *testing.T) {
		var response []JobResponse
		store := &fakeJobStore{
			jobs: nil,
		}
		s := &Server{Store: store}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		rr  := httptest.NewRecorder()
	
		s.HandleListJobs(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		
		if len(response) != 0 {
			t.Fatalf("got %d jobs, wanted 0", len(response))
		}
	})
}


func TestHandleGetJob(t *testing.T) {
	completedAt := time.Date(2026, 6, 6, 13, 47, 35, 0, time.UTC)
	jobId := pgtype.UUID{Bytes: [16]byte{0xe6, 0x47, 0xdf, 0x1f, 0xd8, 0xc0, 0x40, 0xe0, 0xa7, 0x07, 0x93, 0xed, 0x08, 0x79, 0xb0, 0xbe}, Valid: true}
	jobIdString := jobId.String()
	urlString := fmt.Sprintf("/api/v1/jobs/%v", jobIdString)
	fakeJob := storage.JobRow{
		JobID:       jobId,
		Status:      "COMPLETED",
		StartedAt:   time.Date(2026, 6, 6, 13, 47, 34, 0, time.UTC),
		CompletedAt: &completedAt,
		SourceDir:   "data/dumb_data",
		TotalFiles:  5,
		FilesDone:   5,
	}

	chiCtx := func(jobID string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, urlString, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("job_id", jobID)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	t.Run("Succesul Run", func(t *testing.T) {
		var response JobResponse
		store := &fakeJobStore{
			job: &fakeJob,
			err: nil,
		}
		s := &Server{Store: store}

		rr := httptest.NewRecorder()
		s.HandleGetJob(rr, chiCtx(jobIdString))

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if response.Status != "COMPLETED" {
			t.Errorf("got status %q, want %q", response.Status, "COMPLETED")
		}
	})

	t.Run("db error returns 500", func(t *testing.T) {
		store := &fakeJobStore{
			job: nil,
			err: fmt.Errorf("connection refused"),
		}
		s := &Server{Store: store}

		rr := httptest.NewRecorder()
		s.HandleGetJob(rr, chiCtx(jobIdString))

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("invalid job_id returns bad request", func(t *testing.T) {
		store := &fakeJobStore{}
		s := &Server{Store: store}

		rr := httptest.NewRecorder()
		s.HandleGetJob(rr, chiCtx("not-a-uuid"))

		if rr.Code != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}
