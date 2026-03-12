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
