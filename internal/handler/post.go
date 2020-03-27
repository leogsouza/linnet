package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/leogsouza/linnet/internal/service"
)

type createPostInput struct {
	Content   string
	SpoilerOf *string `json:"spoiler_of"`
	NSFW      bool
}

func (h *handler) createPost(w http.ResponseWriter, r *http.Request) {
	var in createPostInput
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ti, err := h.CreatePost(r.Context(), in.Content, in.SpoilerOf, in.NSFW)
	if err == service.ErrUnauthenticated {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err == service.ErrInvalidContent || err == service.ErrInvalidSpoiler {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, ti, http.StatusCreated)
}

func (h *handler) togglePostLike(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, _ := strconv.ParseInt(chi.URLParamFromCtx(ctx, "post_id"), 10, 64)
	out, err := h.TogglePostLike(ctx, postID)
	if err == service.ErrUnauthenticated {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err == service.ErrPostNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, out, http.StatusOK)
}

func (h *handler) posts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	last, _ := strconv.Atoi(q.Get("last"))
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	pp, err := h.Posts(ctx, chi.URLParamFromCtx(ctx, "username"), last, before)
	if err == service.ErrInvalidUsername {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, pp, http.StatusOK)
}

func (h *handler) post(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID, _ := strconv.ParseInt(chi.URLParamFromCtx(ctx, "post_id"), 10, 64)
	p, err := h.Post(ctx, postID)
	if err == service.ErrPostNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	respond(w, p, http.StatusOK)
}
