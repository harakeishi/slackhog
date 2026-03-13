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

func (h *SlackHandler) HandleChatDelete(w http.ResponseWriter, r *http.Request) {
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

	ok := h.store.Delete(channel, ts)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "message_not_found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": channel,
		"ts":      ts,
	})
}

// buildChannelObject はSlack API互換のチャンネルオブジェクトを生成する。
func buildChannelObject(name string) map[string]any {
	return map[string]any{
		"id":              name,
		"name":            name,
		"is_channel":      true,
		"is_group":        false,
		"is_im":           false,
		"is_mpim":         false,
		"is_private":      false,
		"is_archived":     false,
		"is_general":      name == "general",
		"name_normalized": name,
		"num_members":     0,
		"topic":           map[string]any{"value": "", "creator": "", "last_set": 0},
		"purpose":         map[string]any{"value": "", "creator": "", "last_set": 0},
		"previous_names":  []string{},
	}
}

func (h *SlackHandler) HandleConversationsInfo(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "missing_argument",
		})
		return
	}

	found := false
	for _, ch := range h.store.Channels() {
		if ch == channel {
			found = true
			break
		}
	}

	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "channel_not_found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": buildChannelObject(channel),
	})
}

func (h *SlackHandler) HandleConversationsList(w http.ResponseWriter, r *http.Request) {
	channelNames := h.store.Channels()

	channels := make([]map[string]any, 0, len(channelNames))
	for _, name := range channelNames {
		ch := buildChannelObject(name)
		ch["is_member"] = true
		channels = append(channels, ch)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":       true,
		"channels": channels,
		"response_metadata": map[string]any{
			"next_cursor": "",
		},
	})
}

func (h *SlackHandler) HandleConversationsHistory(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "channel_not_found",
		})
		return
	}

	msgs := h.store.List(channel)

	slackMessages := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		slackMessages = append(slackMessages, map[string]any{
			"type":        "message",
			"user":        m.Username,
			"text":        m.Text,
			"ts":          fmt.Sprintf("%d.%06d", m.ReceivedAt.Unix(), m.ReceivedAt.Nanosecond()/1000),
			"blocks":      m.Blocks,
			"attachments": m.Attachments,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":       true,
		"messages": slackMessages,
		"has_more": false,
		"response_metadata": map[string]any{
			"next_cursor": "",
		},
	})
}

func (h *SlackHandler) HandleAuthTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                    true,
		"url":                   "https://slackhog.example.com/",
		"team":                  "SlackHog",
		"user":                  "slackhog",
		"team_id":               "T00000000",
		"user_id":               "U00000000",
		"bot_id":                "B00000000",
		"is_enterprise_install": false,
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
