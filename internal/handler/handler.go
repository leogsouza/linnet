package handler

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/leogsouza/linnet/internal/service"
)

type handler struct {
	*service.Service
}

// New create a http.Handler with predefined routing
func New(s *service.Service) http.Handler {

	h := &handler{s}

	api := chi.NewRouter()
	api.Post("/login", h.login)
	api.Post("/users", h.createUser)

	r := chi.NewRouter()
	r.Handle("/api", api)

	return r
}
