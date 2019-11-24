package handler

import (
	"net/http"

	"github.com/go-chi/chi/middleware"

	"github.com/go-chi/chi"

	"github.com/leogsouza/linnet/internal/service"
)

type handler struct {
	*service.Service
}

// New create a http.Handler with predefined routing
func New(s *service.Service) http.Handler {

	h := &handler{s}

	r := chi.NewRouter()
	r.Use(middleware.StripSlashes)
	r.Route("/api", func(r chi.Router) {
		r.Post("/login", h.login)
		r.Post("/users", h.createUser)

	})

	return r
}
