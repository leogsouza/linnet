package handler

import (
	"fmt"
	"net/http"
	"strings"

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

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		fmt.Printf("%s %s\n", method, route)
		return nil
	}

	if err := chi.Walk(r, walkFunc); err != nil {
		fmt.Printf("Logging err: %s\n", err.Error())
	}

	return r
}
