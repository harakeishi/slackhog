package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SlackHandler struct {
	store       MessageStore
	broadcaster Broadcaster
}

func NewSlackHandler(store MessageStore, broadcaster Broadcaster) *SlackHandler {
	return &SlackHandler{store: store, broadcaster: broadcaster}
}

func (h *SlackHandler) HandleChatPostMessage(w http.ResponseWriter, r *http.Request) {
	var channel, text, username, iconEmoji, iconURL string
	var blocks, attachments any
	var rawPayload any

	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		rawPayload = payload
		channel, _ = payload["channel"].(string)
		text, _ = payload["text"].(string)
		username, _ = payload["username"].(string)
		iconEmoji, _ = payload["icon_emoji"].(string)
		iconURL, _ = payload["icon_url"].(string)
		blocks = payload["blocks"]
		attachments = payload["attachments"]
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		channel = r.FormValue("channel")
		text = r.FormValue("text")
		username = r.FormValue("username")
		iconEmoji = r.FormValue("icon_emoji")
		iconURL = r.FormValue("icon_url")
		rawPayload = r.Form
	}

	msg := Message{
		ID:          uuid.New().String(),
		Channel:     channel,
		Username:    username,
		Text:        text,
		IconEmoji:   iconEmoji,
		IconURL:     iconURL,
		Blocks:      blocks,
		Attachments: attachments,
		ReceivedAt:  time.Now(),
		RawPayload:  rawPayload,
	}

	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": channel,
		"ts":      fmt.Sprintf("%d.%06d", msg.ReceivedAt.Unix(), msg.ReceivedAt.Nanosecond()/1000),
	})
}

func (h *SlackHandler) HandleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	text, _ := payload["text"].(string)
	channel, _ := payload["channel"].(string)
	if channel == "" {
		channel = "webhook"
	}
	username, _ := payload["username"].(string)
	if username == "" {
		username = "incoming-webhook"
	}
	iconEmoji, _ := payload["icon_emoji"].(string)
	iconURL, _ := payload["icon_url"].(string)

	msg := Message{
		ID:          uuid.New().String(),
		Channel:     channel,
		Username:    username,
		Text:        text,
		IconEmoji:   iconEmoji,
		IconURL:     iconURL,
		Blocks:      payload["blocks"],
		Attachments: payload["attachments"],
		ReceivedAt:  time.Now(),
		RawPayload:  payload,
	}

	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
