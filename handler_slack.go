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
	payload, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg := buildMessage(payload)
	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": msg.Channel,
		"ts":      fmt.Sprintf("%d.%06d", msg.ReceivedAt.Unix(), msg.ReceivedAt.Nanosecond()/1000),
	})
}

func (h *SlackHandler) HandleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Webhook defaults
	if payload["channel"] == nil || payload["channel"] == "" {
		payload["channel"] = "webhook"
	}
	if payload["username"] == nil || payload["username"] == "" {
		payload["username"] = "incoming-webhook"
	}

	msg := buildMessage(payload)
	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// parseRequest はJSON/form両対応でリクエストボディをmap[string]anyに変換する。
func (h *SlackHandler) parseRequest(r *http.Request) (map[string]any, error) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("invalid json")
		}
		return payload, nil
	}

	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form")
	}
	payload := make(map[string]any)
	for key, values := range r.Form {
		if len(values) == 1 {
			payload[key] = values[0]
		} else {
			payload[key] = values
		}
	}
	return payload, nil
}

// buildMessage はpayloadからMessageを組み立てる。
func buildMessage(payload map[string]any) Message {
	str := func(key string) string {
		v, _ := payload[key].(string)
		return v
	}
	return Message{
		ID:          uuid.New().String(),
		Channel:     str("channel"),
		Username:    str("username"),
		Text:        str("text"),
		ThreadTS:    str("thread_ts"),
		IconEmoji:   str("icon_emoji"),
		IconURL:     str("icon_url"),
		Blocks:      payload["blocks"],
		Attachments: payload["attachments"],
		ReceivedAt:  time.Now(),
		RawPayload:  payload,
	}
}
