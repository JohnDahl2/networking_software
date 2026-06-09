package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go-sniffer/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

// JobStore is the interface the API layer uses to read and write job data.
// In production, storage.Store satisfies this. In tests, a fake does.
type JobStore interface {
	GetAllJobs(ctx context.Context) ([]storage.JobRow, error)
	GetJob(ctx context.Context, jobIDStr string) (*storage.JobRow, error)
	CreateJob(ctx context.Context, sourceDir string, totalFiles int) (pgtype.UUID, error)
	DeleteJob(ctx context.Context, jobIDStr string) error
}

// PipelineLauncher runs the pcap processing pipeline for a job.
// In production, worker.Launcher satisfies this. In tests, a fake does.
type PipelineLauncher interface {
	Launch(ctx context.Context, jobID pgtype.UUID, paths []string)
}

type Server struct {
	DB              storage.DBStore // used by HandleListPackets for dynamic query building
	Store           JobStore
	Launcher        PipelineLauncher
	Jobs            map[string]context.CancelFunc
	JobsMu          sync.Mutex
	GlobFn          func(pattern string) ([]string, error)
	DefaultSourceDir string
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/packets", func(r chi.Router) {
			r.Get("/", s.HandleListPackets)
		})
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/", s.HandleListJobs)
			r.Post("/", s.HandleCreateJob)
			r.Get("/{job_id}", s.HandleGetJob)
			r.Delete("/{job_id}", s.HandleDeleteJob)
		})
	})
	return r
}
