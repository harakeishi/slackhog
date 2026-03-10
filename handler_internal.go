package main

import (
	"encoding/json"
	"net/http"
)

type InternalHandler struct {
	store MessageStore
}

func NewInternalHandler(store MessageStore) *InternalHandler {
	return &InternalHandler{store: store}
}

func (h *InternalHandler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channel := r.URL.Query().Get("channel")
		messages := h.store.List(channel)
		channels := h.store.Channels()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"messages": messages,
			"channels": channels,
		})

	case http.MethodDelete:
		h.store.Clear()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
