package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	
)

type Server struct{
	DB *pgxpool.Pool	
}

func NewServer() *Server{
	return &Server{}
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
	})
	return r
}
