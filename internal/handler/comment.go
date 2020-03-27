package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/leogsouza/linnet/internal/service"
)

type createCommentInput struct {
	Content string
}

func (h *handler) createComment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var in createCommentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	postID, _ := strconv.ParseInt(chi.URLParamFromCtx(ctx, "post_id"), 10, 64)
	log.Println(postID)
	c, err := h.CreateComment(ctx, postID, in.Content)
	if err == service.ErrUnauthenticated {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err == service.ErrInvalidContent {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
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

	respond(w, c, http.StatusOK)
}
