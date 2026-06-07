package api

import (
	"context"
	"sync"
	"testing"
	"time"
	"encoding/json"
	"fmt"

	"net/http"
	"net/http/httptest"

	"go.uber.org/goleak"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go-sniffer/internal/storage"
)

type fakeJobStore struct {
	jobs         []storage.JobRow
	job          *storage.JobRow
	err          error
	createdJobID pgtype.UUID
	createJobErr error
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

func (f *fakeJobStore) CreateJob(ctx context.Context, sourceDir string, totalFiles int) (pgtype.UUID, error) {
	return f.createdJobID, f.createJobErr
}

func (f *fakeJobStore) DeleteJob(ctx context.Context, jobIDStr string) error {
	return f.err
}

type fakeLauncher struct {
	fn func(ctx context.Context, jobID pgtype.UUID, paths []string)
}

func (f *fakeLauncher) Launch(ctx context.Context, jobID pgtype.UUID, paths []string) {
	if f.fn != nil {
		f.fn(ctx, jobID, paths)
	}
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
	jobID := pgtype.UUID{Bytes: [16]byte{0xe6, 0x47, 0xdf, 0x1f, 0xd8, 0xc0, 0x40, 0xe0, 0xa7, 0x07, 0x93, 0xed, 0x08, 0x79, 0xb0, 0xbe}, Valid: true}
	jobIDString := jobID.String()
	urlString := fmt.Sprintf("/api/v1/jobs/%v", jobIDString)
	fakeJob := storage.JobRow{
		JobID:       jobID,
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
		s.HandleGetJob(rr, chiCtx(jobIDString))

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
		s.HandleGetJob(rr, chiCtx(jobIDString))

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

func TestHandleCreateJob(t *testing.T) {
    defer goleak.VerifyNone(t)

    fakeJobID := pgtype.UUID{
        Bytes: [16]byte{0xe6, 0x47, 0xdf, 0x1f, 0xd8, 0xc0, 0x40, 0xe0, 0xa7, 0x07, 0x93, 0xed, 0x08, 0x79, 0xb0, 0xbe},
        Valid: true,
    }

    t.Run("glob error returns 400", func(t *testing.T) {
        s := &Server{
            Jobs:   make(map[string]context.CancelFunc),
            GlobFn: func(pattern string) ([]string, error) {
                return nil, fmt.Errorf("bad pattern")
            },
        }
        req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs?source_dir=/data", nil)
        rr := httptest.NewRecorder()
        s.HandleCreateJob(rr, req)
        if rr.Code != http.StatusBadRequest {
            t.Errorf("got %d, want %d", rr.Code, http.StatusBadRequest)
        }
    })

    t.Run("no pcap files found returns 400", func(t *testing.T) {
        s := &Server{
            Jobs:   make(map[string]context.CancelFunc),
            GlobFn: func(pattern string) ([]string, error) {
                return []string{}, nil
            },
        }
        req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs?source_dir=/data", nil)
        rr := httptest.NewRecorder()
        s.HandleCreateJob(rr, req)
        if rr.Code != http.StatusBadRequest {
            t.Errorf("got %d, want %d", rr.Code, http.StatusBadRequest)
        }
    })

    t.Run("db create fails returns 500", func(t *testing.T) {
        s := &Server{
            Jobs: make(map[string]context.CancelFunc),
            GlobFn: func(pattern string) ([]string, error) {
                return []string{"file1.pcap"}, nil
            },
            Store: &fakeJobStore{
                createJobErr: fmt.Errorf("connection refused"),
            },
        }
        req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs?source_dir=/data", nil)
        rr := httptest.NewRecorder()
        s.HandleCreateJob(rr, req)
        if rr.Code != http.StatusInternalServerError {
            t.Errorf("got %d, want %d", rr.Code, http.StatusInternalServerError)
        }
    })

    t.Run("happy path returns 202 with correct body", func(t *testing.T) {
        var wg sync.WaitGroup
        wg.Add(1)

        s := &Server{
            Jobs: make(map[string]context.CancelFunc),
            GlobFn: func(pattern string) ([]string, error) {
                return []string{"file1.pcap", "file2.pcap"}, nil
            },
            Store: &fakeJobStore{
                createdJobID: fakeJobID,
            },
            Launcher: &fakeLauncher{
                fn: func(ctx context.Context, jobID pgtype.UUID, paths []string) {
                    defer wg.Done()
                },
            },
        }

        req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs?source_dir=/data", nil)
        rr := httptest.NewRecorder()
        s.HandleCreateJob(rr, req)
        wg.Wait()

        if rr.Code != http.StatusAccepted {
            t.Errorf("got %d, want %d", rr.Code, http.StatusAccepted)
        }

        var response JobResponse
        if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
            t.Fatalf("failed to decode response: %v", err)
        }
        if response.Status != "PROCESSING" {
            t.Errorf("got status %q, want %q", response.Status, "PROCESSING")
        }
        if response.TotalFiles != 2 {
            t.Errorf("got total_files %d, want 2", response.TotalFiles)
        }
        if response.JobID != fakeJobID.String() {
            t.Errorf("got job_id %q, want %q", response.JobID, fakeJobID.String())
        }
    })
}

func TestDeleteJob(t *testing.T) {
    jobID := pgtype.UUID{Bytes: [16]byte{0xe6, 0x47, 0xdf, 0x1f, 0xd8, 0xc0, 0x40, 0xe0, 0xa7, 0x07, 0x93, 0xed, 0x08, 0x79, 0xb0, 0xbe}, Valid: true}
    jobIDStr := jobID.String()

    chiCtx := func(id string) *http.Request {
        req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+id, nil)
        rctx := chi.NewRouteContext()
        rctx.URLParams.Add("job_id", id)
        return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
    }

    t.Run("job not running returns 204", func(t *testing.T) {
        s := &Server{
            Jobs:  make(map[string]context.CancelFunc),
            Store: &fakeJobStore{},
        }
        rr := httptest.NewRecorder()
        s.DeleteJob(rr, chiCtx(jobIDStr))
        if rr.Code != http.StatusNoContent {
            t.Errorf("got %d, want %d", rr.Code, http.StatusNoContent)
        }
    })

    t.Run("cancels running job and removes it from map", func(t *testing.T) {
        cancelled := false
        s := &Server{
            Jobs: map[string]context.CancelFunc{
                jobIDStr: func() { cancelled = true },
            },
            Store: &fakeJobStore{},
        }
        rr := httptest.NewRecorder()
        s.DeleteJob(rr, chiCtx(jobIDStr))

        if rr.Code != http.StatusNoContent {
            t.Errorf("got %d, want %d", rr.Code, http.StatusNoContent)
        }
        if !cancelled {
            t.Error("expected cancel to be called for running job")
        }
        s.JobsMu.Lock()
        _, stillPresent := s.Jobs[jobIDStr]
        s.JobsMu.Unlock()
        if stillPresent {
            t.Error("expected job to be removed from Jobs map after deletion")
        }
    })

    t.Run("db error returns 502", func(t *testing.T) {
        s := &Server{
            Jobs:  make(map[string]context.CancelFunc),
            Store: &fakeJobStore{err: fmt.Errorf("connection refused")},
        }
        rr := httptest.NewRecorder()
        s.DeleteJob(rr, chiCtx(jobIDStr))
        if rr.Code != http.StatusBadGateway {
            t.Errorf("got %d, want %d", rr.Code, http.StatusBadGateway)
        }
    })
}