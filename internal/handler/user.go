package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"

	"github.com/leogsouza/linnet/internal/service"
)

type createUserInput struct {
	Email, Username string
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in createUserInput

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.CreateUser(r.Context(), in.Email, in.Username)
	if err == service.ErrInvalidEmail || err == service.ErrInvalidUsername {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err == service.ErrEmailTaken || err == service.ErrUsernameTaken {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) user(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := chi.URLParamFromCtx(ctx, "username")

	u, err := h.User(ctx, username)
	if err == service.ErrInvalidUsername {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err == service.ErrUserNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, u, http.StatusOK)
}

func (h *handler) toggleFollow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := chi.URLParamFromCtx(ctx, "username")
	out, err := h.ToggleFollow(ctx, username)
	if err == service.ErrUnauthenticated {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err == service.ErrInvalidUsername {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err == service.ErrUserNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err == service.ErrForbiddenFollow {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, out, http.StatusOK)

}
