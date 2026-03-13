package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type mockBroadcaster struct {
	messages []Message
}

func (m *mockBroadcaster) Broadcast(msg Message) {
	m.messages = append(m.messages, msg)
}

func TestHandleChatPostMessage(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	form := url.Values{}
	form.Set("channel", "general")
	form.Set("text", "hello world")
	form.Set("username", "testbot")

	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleChatPostMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msgs := store.List("")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(msgs))
	}
	if msgs[0].Channel != "general" {
		t.Fatalf("expected channel 'general', got %q", msgs[0].Channel)
	}
	if msgs[0].Text != "hello world" {
		t.Fatalf("expected text 'hello world', got %q", msgs[0].Text)
	}
	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}
}

func TestHandleChatPostMessage_JSON(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"alerts","text":"server down","username":"monitor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleChatPostMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	msgs := store.List("alerts")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in alerts, got %d", len(msgs))
	}
}

func TestHandleIncomingWebhook(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"text":"webhook message"}`
	req := httptest.NewRequest(http.MethodPost, "/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleIncomingWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	msgs := store.List("")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(msgs))
	}
	if msgs[0].Text != "webhook message" {
		t.Fatalf("expected 'webhook message', got %q", msgs[0].Text)
	}
}

func TestHandleChatPostMessage_FormWithAttachments(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	form := url.Values{}
	form.Set("channel", "#alerts")
	form.Set("text", "test notification")
	form.Set("attachments", `[{"color":"danger","title":"Error","fields":[{"title":"ID","value":"123","short":true}],"footer":"please check"}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.HandleChatPostMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msgs := store.List("#alerts")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	// attachments should be parsed into []any, not remain as string
	att, ok := msgs[0].Attachments.([]any)
	if !ok {
		t.Fatalf("expected attachments to be []any, got %T", msgs[0].Attachments)
	}
	if len(att) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(att))
	}
	attMap, ok := att[0].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment to be map[string]any, got %T", att[0])
	}
	if attMap["color"] != "danger" {
		t.Fatalf("expected color 'danger', got %v", attMap["color"])
	}
}

func TestHandleChatPostMessage_JSONWithAttachments(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"general","text":"alert","attachments":[{"color":"good","title":"OK","fields":[{"title":"Status","value":"healthy","short":true}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleChatPostMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msgs := store.List("general")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	// JSON request: attachments should already be []any
	att, ok := msgs[0].Attachments.([]any)
	if !ok {
		t.Fatalf("expected attachments to be []any, got %T", msgs[0].Attachments)
	}
	if len(att) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(att))
	}
}

func TestTryParseJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantType string
	}{
		{"array string", `[{"color":"good"}]`, "[]any"},
		{"object string", `{"key":"val"}`, "map"},
		{"plain string", "hello", "string"},
		{"empty string", "", "string"},
		{"nil", nil, "nil"},
		{"already array", []any{"a"}, "[]any"},
		{"invalid json", `[broken`, "string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tryParseJSON(tt.input)
			switch tt.wantType {
			case "[]any":
				if _, ok := result.([]any); !ok {
					t.Errorf("expected []any, got %T", result)
				}
			case "map":
				if _, ok := result.(map[string]any); !ok {
					t.Errorf("expected map[string]any, got %T", result)
				}
			case "string":
				if _, ok := result.(string); !ok {
					t.Errorf("expected string, got %T", result)
				}
			case "nil":
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			}
		})
	}
}

func TestHandleChatPostMessage_WithThreadTS(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"general","text":"thread reply","thread_ts":"parent-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleChatPostMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}
	if bc.messages[0].ThreadTS != "parent-id" {
		t.Fatalf("expected ThreadTS='parent-id', got %q", bc.messages[0].ThreadTS)
	}
}

func TestHandleChatUpdate_JSON(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	postBody := `{"channel":"general","text":"original","username":"bot"}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	h.HandleChatPostMessage(postW, postReq)

	msgs := store.List("general")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	origMsg := msgs[0]
	ts := fmt.Sprintf("%d.%06d", origMsg.ReceivedAt.Unix(), origMsg.ReceivedAt.Nanosecond()/1000)

	updateBody := fmt.Sprintf(`{"channel":"general","ts":"%s","text":"updated text"}`, ts)
	updateReq := httptest.NewRequest(http.MethodPost, "/api/chat.update", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	h.HandleChatUpdate(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateW.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(updateW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}
	if resp["ts"] != ts {
		t.Fatalf("expected ts=%q, got %v", ts, resp["ts"])
	}

	updated, ok := store.FindByTS("general", ts)
	if !ok {
		t.Fatal("expected to find updated message")
	}
	if updated.Text != "updated text" {
		t.Fatalf("expected text 'updated text', got %q", updated.Text)
	}
}

func TestHandleChatUpdate_Form(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	postBody := `{"channel":"general","text":"original","username":"bot"}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	h.HandleChatPostMessage(httptest.NewRecorder(), postReq)

	msgs := store.List("general")
	origMsg := msgs[0]
	ts := fmt.Sprintf("%d.%06d", origMsg.ReceivedAt.Unix(), origMsg.ReceivedAt.Nanosecond()/1000)

	form := url.Values{}
	form.Set("channel", "general")
	form.Set("ts", ts)
	form.Set("text", "form updated")

	updateReq := httptest.NewRequest(http.MethodPost, "/api/chat.update", strings.NewReader(form.Encode()))
	updateReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateW := httptest.NewRecorder()
	h.HandleChatUpdate(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateW.Code)
	}

	updated, ok := store.FindByTS("general", ts)
	if !ok {
		t.Fatal("expected to find updated message")
	}
	if updated.Text != "form updated" {
		t.Fatalf("expected text 'form updated', got %q", updated.Text)
	}
}

func TestHandleChatUpdate_MessageNotFound(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"general","ts":"9999999999.000000","text":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleChatUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "message_not_found" {
		t.Fatalf("expected error 'message_not_found', got %v", resp["error"])
	}
}

func TestHandleChatUpdate_MissingArgs(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"text":"no channel or ts"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleChatUpdate(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "missing_argument" {
		t.Fatalf("expected error 'missing_argument', got %v", resp["error"])
	}
}

func TestHandleChatDelete_JSON(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	postBody := `{"channel":"general","text":"to delete","username":"bot"}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	h.HandleChatPostMessage(httptest.NewRecorder(), postReq)

	msgs := store.List("general")
	origMsg := msgs[0]
	ts := fmt.Sprintf("%d.%06d", origMsg.ReceivedAt.Unix(), origMsg.ReceivedAt.Nanosecond()/1000)

	deleteBody := fmt.Sprintf(`{"channel":"general","ts":"%s"}`, ts)
	deleteReq := httptest.NewRequest(http.MethodPost, "/api/chat.delete", strings.NewReader(deleteBody))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteW := httptest.NewRecorder()
	h.HandleChatDelete(deleteW, deleteReq)

	if deleteW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteW.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(deleteW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	remaining := store.List("general")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 messages after delete, got %d", len(remaining))
	}
}

func TestHandleChatDelete_MessageNotFound(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"general","ts":"9999999999.000000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleChatDelete(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "message_not_found" {
		t.Fatalf("expected error 'message_not_found', got %v", resp["error"])
	}
}

func TestHandleChatDelete_MissingArgs(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	body := `{"channel":"general"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleChatDelete(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "missing_argument" {
		t.Fatalf("expected error 'missing_argument', got %v", resp["error"])
	}
}

func TestHandleConversationsInfo(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	postBody := `{"channel":"general","text":"hello","username":"bot"}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	h.HandleChatPostMessage(httptest.NewRecorder(), postReq)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.info?channel=general", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	ch, ok := resp["channel"].(map[string]any)
	if !ok {
		t.Fatalf("expected channel object, got %T", resp["channel"])
	}
	if ch["name"] != "general" {
		t.Fatalf("expected name 'general', got %v", ch["name"])
	}
	if ch["is_channel"] != true {
		t.Fatalf("expected is_channel=true")
	}
	if ch["is_general"] != true {
		t.Fatalf("expected is_general=true for 'general' channel")
	}
}

func TestHandleConversationsInfo_NotFound(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.info?channel=nonexistent", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsInfo(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "channel_not_found" {
		t.Fatalf("expected error 'channel_not_found', got %v", resp["error"])
	}
}

func TestHandleConversationsInfo_MissingChannel(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.info", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsInfo(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "missing_argument" {
		t.Fatalf("expected error 'missing_argument', got %v", resp["error"])
	}
}

func TestHandleConversationsList(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	for _, ch := range []string{"general", "random", "alerts"} {
		body := fmt.Sprintf(`{"channel":"%s","text":"hi","username":"bot"}`, ch)
		req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.HandleChatPostMessage(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.list", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	channels, ok := resp["channels"].([]any)
	if !ok {
		t.Fatalf("expected channels array, got %T", resp["channels"])
	}
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}

	first, ok := channels[0].(map[string]any)
	if !ok {
		t.Fatalf("expected channel object, got %T", channels[0])
	}
	if first["name"] != "general" {
		t.Fatalf("expected first channel name 'general', got %v", first["name"])
	}
	if first["is_member"] != true {
		t.Fatalf("expected is_member=true")
	}

	meta, ok := resp["response_metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected response_metadata object")
	}
	if meta["next_cursor"] != "" {
		t.Fatalf("expected empty next_cursor, got %v", meta["next_cursor"])
	}
}

func TestHandleAuthTest(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth.test", nil)
	w := httptest.NewRecorder()
	h.HandleAuthTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}
	if resp["user_id"] == nil || resp["user_id"] == "" {
		t.Fatal("expected user_id to be set")
	}
	if resp["team_id"] == nil || resp["team_id"] == "" {
		t.Fatal("expected team_id to be set")
	}
}

func TestHandleConversationsHistory(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	// Post messages
	for _, text := range []string{"msg1", "msg2", "msg3"} {
		body := fmt.Sprintf(`{"channel":"general","text":"%s","username":"bot"}`, text)
		req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.HandleChatPostMessage(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.history?channel=general", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	messages, ok := resp["messages"].([]any)
	if !ok {
		t.Fatalf("expected messages array, got %T", resp["messages"])
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// Check first message has ts field
	first, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected message object, got %T", messages[0])
	}
	if first["ts"] == nil || first["ts"] == "" {
		t.Fatal("expected ts to be set")
	}
	if first["type"] != "message" {
		t.Fatalf("expected type 'message', got %v", first["type"])
	}

	if resp["has_more"] != false {
		t.Fatalf("expected has_more=false, got %v", resp["has_more"])
	}
}

func TestHandleConversationsHistory_MissingChannel(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.history", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsHistory(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != false {
		t.Fatalf("expected ok=false, got %v", resp["ok"])
	}
	if resp["error"] != "channel_not_found" {
		t.Fatalf("expected error 'channel_not_found', got %v", resp["error"])
	}
}

func TestHandleConversationsHistory_Empty(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	// Create channel by posting then check history
	body := `{"channel":"empty-ch","text":"hi","username":"bot"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat.postMessage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.HandleChatPostMessage(httptest.NewRecorder(), req)

	// Delete the message to make channel exist but empty
	msgs := store.List("empty-ch")
	ts := fmt.Sprintf("%d.%06d", msgs[0].ReceivedAt.Unix(), msgs[0].ReceivedAt.Nanosecond()/1000)
	store.Delete("empty-ch", ts)

	req = httptest.NewRequest(http.MethodGet, "/api/conversations.history?channel=empty-ch", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsHistory(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}
	messages := resp["messages"].([]any)
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(messages))
	}
}

func TestHandleConversationsList_Empty(t *testing.T) {
	store := NewMemoryStore(100)
	bc := &mockBroadcaster{}
	h := NewSlackHandler(store, bc)

	req := httptest.NewRequest(http.MethodGet, "/api/conversations.list", nil)
	w := httptest.NewRecorder()
	h.HandleConversationsList(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != true {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	channels, ok := resp["channels"].([]any)
	if !ok {
		t.Fatalf("expected channels array, got %T", resp["channels"])
	}
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(channels))
	}
}
