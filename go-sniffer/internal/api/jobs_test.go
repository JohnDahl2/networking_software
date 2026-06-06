package api

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go-sniffer/internal/storage"
)

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
