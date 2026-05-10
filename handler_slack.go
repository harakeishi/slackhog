package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	h.store.Add(&msg)
	h.broadcaster.Broadcast(msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": msg.Channel,
		"ts":      msg.TS(),
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
	h.store.Add(&msg)
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	limit := 100
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	msgs := h.store.List(channel)

	q := r.URL.Query()
	inclusive := q.Get("inclusive") == "1"

	// cursor はページ境界の ts を base64 エンコードしたもの。latest の上限として使う
	latestParam := q.Get("latest")
	if cursor := q.Get("cursor"); cursor != "" {
		if decoded, err := base64.StdEncoding.DecodeString(cursor); err == nil {
			latestParam = string(decoded)
		}
	}

	if oldest := q.Get("oldest"); oldest != "" {
		oldestF, err := strconv.ParseFloat(oldest, 64)
		if err == nil {
			filtered := make([]Message, 0, len(msgs))
			for _, m := range msgs {
				tsF, _ := strconv.ParseFloat(m.TS(), 64)
				if (inclusive && tsF >= oldestF) || (!inclusive && tsF > oldestF) {
					filtered = append(filtered, m)
				}
			}
			msgs = filtered
		}
	}

	if latestParam != "" {
		latestF, err := strconv.ParseFloat(latestParam, 64)
		if err == nil {
			filtered := make([]Message, 0, len(msgs))
			for _, m := range msgs {
				tsF, _ := strconv.ParseFloat(m.TS(), 64)
				if (inclusive && tsF <= latestF) || (!inclusive && tsF < latestF) {
					filtered = append(filtered, m)
				}
			}
			msgs = filtered
		}
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[len(msgs)-limit:]
	}

	// Slack API は newest-first（降順）で返す
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	result := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		entry := map[string]any{
			"type": "message",
			"text": m.Text,
			"ts":   m.TS(),
			"user": m.Username,
		}
		if m.ReplyCount > 0 {
			entry["thread_ts"] = m.TS()
			entry["reply_count"] = m.ReplyCount
		}
		result = append(result, entry)
	}

	// has_more=true のとき現在ページの末尾（oldest）ts を base64 エンコードして cursor にする
	nextCursor := ""
	if hasMore && len(msgs) > 0 {
		nextCursor = base64.StdEncoding.EncodeToString([]byte(msgs[len(msgs)-1].TS()))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                true,
		"messages":          result,
		"has_more":          hasMore,
		"response_metadata": map[string]any{"next_cursor": nextCursor},
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
