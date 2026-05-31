package api

import (
	"sync"
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB *pgxpool.Pool
	Ctx context.Context
	Jobs    map[string]context.CancelFunc
	JobsMu  sync.Mutex
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
			r.Delete("/{job_id}", s.DeleteJob)
		})
	})
	return r
}
