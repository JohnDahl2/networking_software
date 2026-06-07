package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidUUID = errors.New("invalid job UUID")
var ErrJobNotFound = errors.New("job not found")
var ErrDatabase = errors.New("database connection error")

type JobRow struct {
    JobID       pgtype.UUID `db:"job_id"`
    Status      string      `db:"status"`
    StartedAt   time.Time   `db:"started_at"`
    CompletedAt *time.Time  `db:"completed_at"`
    SourceDir   string      `db:"source_dir"`
    TotalFiles  int         `db:"total_files"`
    FilesDone   int         `db:"files_read"`
}

type Store struct {
	DB DBStore
}

func (s *Store) GetAllJobs(ctx context.Context) ([]JobRow, error) {
	query := `
		SELECT job_id, status, started_at, completed_at, source_dir, total_files, files_read
		FROM job_tracking
		ORDER BY started_at DESC
	`
    rows, err := s.DB.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("getting jobs: %w", err)
    }
    defer rows.Close()

    var jobs []JobRow
    for rows.Next() {
        var row JobRow
        if err := rows.Scan(
            &row.JobID,
            &row.Status,
            &row.StartedAt,
            &row.CompletedAt,
            &row.SourceDir,
            &row.TotalFiles,
            &row.FilesDone,
        ); err != nil {
            return nil, fmt.Errorf("scanning job row: %w", err)
        }
        jobs = append(jobs, row)
    }
    return jobs, nil
}

// CreateJob inserts a new PENDING job into job_tracking and returns the generated job_id.
func (s *Store) CreateJob(ctx context.Context, sourceDir string, totalFiles int) (pgtype.UUID, error) {
	query := `
		INSERT INTO job_tracking (
			status, started_at, source_dir, total_files, files_read
		) VALUES (
			$1, $2, $3, $4, $5
		) RETURNING job_id
	`

	var jobID pgtype.UUID
	err := s.DB.QueryRow(ctx, query,
		"PROCESSING",
		time.Now(),
		sourceDir,
		totalFiles,
		0,
	).Scan(&jobID)
	if err != nil {
		slog.Error("failed to create job record", "error", err)
		return pgtype.UUID{}, err
	}
	return jobID, nil
}

// UpdateJobProgress increments files_read and recalculates progress_pct.
func UpdateJobProgress(ctx context.Context, DB DBStore, jobID pgtype.UUID, filesRead int) error {
	query := `
		UPDATE job_tracking
		SET files_read = files_read + $1
		WHERE job_id = $2
	`
	_, err := DB.Exec(ctx, query, filesRead, jobID)
	if err != nil {
		slog.Error("failed to update job progress", "error", err)
		return err
	}
	return nil
}

// UpdateJobStatus updates the job status and optionally sets completed_at.
// Pass nil for completedAt when transitioning to PROCESSING.
func UpdateJobStatus(ctx context.Context, DB DBStore, jobID pgtype.UUID, status string, completedAt *time.Time) error {
	query := `
		UPDATE job_tracking
		SET status       = $1,
		    completed_at = $2
		WHERE job_id = $3
	`
	_, err := DB.Exec(ctx, query, status, completedAt, jobID)
	if err != nil {
		slog.Error("failed to update job status", "error", err)
		return err
	}
	return nil
}

func (s *Store) GetHandleJob(ctx context.Context, jobIDStr string) (*JobRow, error) {
	var jobID pgtype.UUID
	if err := jobID.Scan(jobIDStr); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUUID, err)
	}

	query := `
		SELECT job_id, status, started_at, completed_at, source_dir, total_files, files_read
		FROM job_tracking
		WHERE job_id = $1
	`
	var row JobRow
	err := s.DB.QueryRow(ctx, query, jobID).Scan(
		&row.JobID,
		&row.Status,
		&row.StartedAt,
		&row.CompletedAt,
		&row.SourceDir,
		&row.TotalFiles,
		&row.FilesDone,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJobNotFound, err)
	}

	return &row, nil
}

func (s *Store) DeleteJob(ctx context.Context, jobIDStr string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM packet_logs WHERE job_id = $1`, jobIDStr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabase, err)
	}
	_, err = s.DB.Exec(ctx, `DELETE FROM job_tracking WHERE job_id = $1`, jobIDStr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabase, err)
	}
	return nil
}
