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
		r.Use(h.withAuth)
		r.Post("/login", h.login)
		r.Get("/auth_user", h.authUser)
		r.Post("/users", h.createUser)
		r.Get("/users", h.users)
		r.Get("/timeline", h.timeline)
		r.Get("/users/{username}", h.user)
		r.Put("/auth_user/avatar", h.updateAvatar)
		r.Post("/users/{username}/toggle_follow", h.toggleFollow)
		r.Get("/users/{username}/followers", h.followers)
		r.Get("/users/{username}/followees", h.followees)
		r.Post("/posts", h.createPost)
		r.Get("/users/{username}/posts", h.posts)
		r.Post("/posts/{post_id}/toggle_like", h.togglePostLike)
		r.Get("/posts/{post_id}", h.post)
		r.Post("/posts/{post_id}/comments", h.createComment)
		r.Get("/posts/{post_id}/comments", h.comments)
		r.Get("/comments/{comment_id}/toggle_like", h.toggleCommentLike)
		r.Get("/notifications", h.notifications)
		r.Post("/notifications/{notification_id}/mark_as_read", h.markNotificationAsRead)
		r.Post("/mark_notifications_as_read", h.markNotificationsAsRead)
	})

	return r
}
