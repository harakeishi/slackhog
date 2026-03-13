package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleGetMessages(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(&Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
	store.Add(&Message{ID: "2", Channel: "alerts", Text: "alert", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/_api/messages", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Messages []Message `json:"messages"`
		Channels []string  `json:"channels"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}
	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}
}

func TestHandleGetMessages_FilterByChannel(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(&Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
	store.Add(&Message{ID: "2", Channel: "alerts", Text: "alert", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/_api/messages?channel=general", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)

	var resp struct {
		Messages []Message `json:"messages"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message for general, got %d", len(resp.Messages))
	}
}

func TestHandleReplies(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(&Message{ID: "parent1", Channel: "general", Text: "parent", ReceivedAt: time.Now()})
	store.Add(&Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1", ReceivedAt: time.Now()})
	store.Add(&Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/_api/messages/parent1/replies", nil)
	w := httptest.NewRecorder()
	h.HandleReplies(w, req, "parent1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		ParentID string    `json:"parent_id"`
		Replies  []Message `json:"replies"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(resp.Replies))
	}
	if resp.ParentID != "parent1" {
		t.Fatalf("expected parent_id=parent1, got %s", resp.ParentID)
	}
}

func TestHandleReplies_NonExistentParent(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(&Message{ID: "parent1", Channel: "general", Text: "parent", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/_api/messages/nonexistent/replies", nil)
	w := httptest.NewRecorder()
	h.HandleReplies(w, req, "nonexistent")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		ParentID string    `json:"parent_id"`
		Replies  []Message `json:"replies"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Replies) != 0 {
		t.Fatalf("expected 0 replies for nonexistent parent, got %d", len(resp.Replies))
	}
	if resp.Replies == nil {
		t.Fatal("expected empty array, got null")
	}
}

func TestHandleDeleteMessages(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(&Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/_api/messages", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msgs := store.List("")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after delete, got %d", len(msgs))
	}
}
