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

func (h *SlackHandler) HandleChatUpdate(w http.ResponseWriter, r *http.Request) {
	payload, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	channel, _ := payload["channel"].(string)
	ts, _ := payload["ts"].(string)

	if channel == "" || ts == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "missing_argument",
		})
		return
	}

	ok := h.store.Update(channel, ts, func(m *Message) {
		if text, exists := payload["text"]; exists {
			m.Text, _ = text.(string)
		}
		if blocks, exists := payload["blocks"]; exists {
			m.Blocks = tryParseJSON(blocks)
		}
		if attachments, exists := payload["attachments"]; exists {
			m.Attachments = tryParseJSON(attachments)
		}
		m.RawPayload = payload
	})

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "message_not_found",
		})
		return
	}

	updated, _ := h.store.FindByTS(channel, ts)
	h.broadcaster.Broadcast(updated)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": channel,
		"ts":      ts,
		"text":    updated.Text,
	})
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

// tryParseJSON はJSON文字列を受け取った場合にパースして返す。
// 文字列でない場合やパースに失敗した場合はそのまま返す。
func tryParseJSON(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return v
	}
	if s[0] != '[' && s[0] != '{' {
		return v
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return v
	}
	return parsed
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
		Blocks:      tryParseJSON(payload["blocks"]),
		Attachments: tryParseJSON(payload["attachments"]),
		ReceivedAt:  time.Now(),
		RawPayload:  payload,
	}
}
