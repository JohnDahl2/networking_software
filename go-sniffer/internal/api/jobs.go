package api

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go-sniffer/internal/storage"
	"go-sniffer/internal/worker"

	"github.com/go-chi/chi/v5"
)

type JobResponse struct {
	JobID       string     `json:"job_id"`
	Status      string     `json:"status"`
	ProgressPct int        `json:"progress_pct"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	SourceDir   string     `json:"source_dir"`
	TotalFiles  int        `json:"total_files"`
	FilesRead   int        `json:"files_read"`
}

func jobRowToResponse(row storage.JobRow) JobResponse {
	var pct int
	if row.TotalFiles > 0 {
		pct = (row.FilesDone * 100) / row.TotalFiles
	}
	return JobResponse{
		JobID:       row.JobID.String(),
		Status:      row.Status,
		ProgressPct: pct,
		StartedAt:   row.StartedAt,
		CompletedAt: row.CompletedAt,
		SourceDir:   row.SourceDir,
		TotalFiles:  row.TotalFiles,
		FilesRead:   row.FilesDone,
	}
}

// HandleListJobs returns all jobs ordered by most recent first.
func (s *Server) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.GetAllJobs(r.Context())
	if err != nil {
		http.Error(w, "error while getting jobs", http.StatusInternalServerError)
		return
	}

	response := make([]JobResponse, len(rows))
    for i, row := range rows {
        response[i] = jobRowToResponse(row)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// HandleGetJob returns a single job by ID.
func (s *Server) HandleGetJob(w http.ResponseWriter, r *http.Request) {
    jobIDStr := chi.URLParam(r, "job_id")

    row, err := s.Store.GetHandleJob(r.Context(), jobIDStr)
    if err != nil {
        switch {
        case errors.Is(err, storage.ErrInvalidUUID):
            http.Error(w, "invalid job id", http.StatusBadRequest)
        case errors.Is(err, storage.ErrJobNotFound):
            http.Error(w, "job not found", http.StatusNotFound)
        default:
            http.Error(w, "internal server error", http.StatusInternalServerError)
        }
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(jobRowToResponse(*row))
}

// HandleCreateJob starts a new extraction job in the background and returns the job_id immediately.
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	pcapDir := r.URL.Query().Get("source_dir")
	if pcapDir == "" {
		pcapDir = os.Getenv("FILE_FOLDER")
	}

	// Count files so we can create the job record before starting the pipeline.
	filePaths, _ := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))	

	// Create the job synchronously so we have an ID to return immediately.
	jobID, err := storage.CreateJob(s.Ctx, s.DB, pcapDir, len(filePaths))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create job: %v", err), http.StatusInternalServerError)
		return
	}

	jobCtx, cancel := context.WithCancel(s.Ctx)

	s.JobsMu.Lock()
	s.Jobs[jobID.String()] = cancel
	s.JobsMu.Unlock()

	go func() {
		worker.ProcessWithPool(jobCtx, s.DB, jobID, filePaths, 2, 2, 2)
		s.JobsMu.Lock()
		delete(s.Jobs, jobID.String())
		s.JobsMu.Unlock()
	}()

	// Return the job immediately as 202 Accepted.
	row := storage.JobRow{
		JobID:      jobID,
		Status:     "PROCESSING",
		StartedAt:  time.Now(),
		SourceDir:  pcapDir,
		TotalFiles: len(filePaths),
		FilesDone:  0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(jobRowToResponse(row))
}

// HandleCDeleteJob to delete job.
func (s *Server) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobIDStr := chi.URLParam(r, "job_id")

	// If the job is still running, cancel it.
	s.JobsMu.Lock()
	if cancel, ok := s.Jobs[jobIDStr]; ok {
		cancel()
		delete(s.Jobs, jobIDStr)
	}
	s.JobsMu.Unlock()

	_, err := s.DB.Exec(r.Context(), `DELETE FROM packet_logs WHERE job_id = $1`, jobIDStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("DB error: %v", err), http.StatusBadGateway)
		return
	}
	_, err = s.DB.Exec(r.Context(), `DELETE FROM job_tracking WHERE job_id = $1`, jobIDStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("DB error: %v", err), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
