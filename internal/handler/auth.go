package handler

import (
	"encoding/json"
	"net/http"
)

type loginInput struct {
	Email string
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var in loginInput

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
