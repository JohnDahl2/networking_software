package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go-sniffer/internal/storage"
	"go-sniffer/internal/worker"
)

// JobResponse is the API-friendly representation of a job_tracking row.
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
	query := `
		SELECT job_id, status, started_at, completed_at, source_dir, total_files, files_read
		FROM job_tracking
		ORDER BY started_at DESC
	`
	rows, err := s.DB.Query(r.Context(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("DB error: %v", err), http.StatusBadGateway)
		return
	}
	defer rows.Close()

	jobs := []JobResponse{}
	for rows.Next() {
		var row storage.JobRow
		if err := rows.Scan(
			&row.JobID,
			&row.Status,
			&row.StartedAt,
			&row.CompletedAt,
			&row.SourceDir,
			&row.TotalFiles,
			&row.FilesDone,
		); err != nil {
			http.Error(w, "error scanning job row", http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, jobRowToResponse(row))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// HandleGetJob returns a single job by ID.
func (s *Server) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	jobIDStr := chi.URLParam(r, "job_id")

	var jobID pgtype.UUID
	if err := jobID.Scan(jobIDStr); err != nil {
		http.Error(w, "invalid job_id", http.StatusBadRequest)
		return
	}

	query := `
		SELECT job_id, status, started_at, completed_at, source_dir, total_files, files_read
		FROM job_tracking
		WHERE job_id = $1
	`
	var row storage.JobRow
	err := s.DB.QueryRow(r.Context(), query, jobID).Scan(
		&row.JobID,
		&row.Status,
		&row.StartedAt,
		&row.CompletedAt,
		&row.SourceDir,
		&row.TotalFiles,
		&row.FilesDone,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("job not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobRowToResponse(row))
}

// HandleCreateJob starts a new extraction job in the background and returns the job_id immediately.
func (s *Server) HandleCreateJob(w http.ResponseWriter, r *http.Request) {
	pcapDir := r.URL.Query().Get("source_dir")
	if pcapDir == "" {
		pcapDir = "data/dumb_data"
	}

	// Count files so we can create the job record before starting the pipeline.
	filePaths, _ := filepath.Glob(filepath.Join(pcapDir, "*.pcap"))

	// Create the job synchronously so we have an ID to return immediately.
	jobID, err := storage.CreateJob(context.Background(), s.DB, pcapDir, len(filePaths))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create job: %v", err), http.StatusInternalServerError)
		return
	}

	// Start the pipeline in the background with the pre-created job ID.
	go worker.ProcessWithPool(context.Background(), s.DB, jobID, 2, 2)

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
