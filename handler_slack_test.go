package main

import (
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
