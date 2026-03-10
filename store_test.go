package main

import (
	"testing"
	"time"
)

func newTestMessage(id, channel, text string) Message {
	return Message{
		ID:         id,
		Channel:    channel,
		Username:   "testuser",
		Text:       text,
		ReceivedAt: time.Now(),
	}
}

func TestMemoryStore_AddAndList(t *testing.T) {
	store := NewMemoryStore(10)
	msg := newTestMessage("1", "general", "hello")
	store.Add(msg)

	msgs := store.List("")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ID != "1" {
		t.Errorf("expected ID=1, got %s", msgs[0].ID)
	}
}

func TestMemoryStore_ListByChannel(t *testing.T) {
	store := NewMemoryStore(10)
	store.Add(newTestMessage("1", "general", "hello"))
	store.Add(newTestMessage("2", "random", "world"))
	store.Add(newTestMessage("3", "general", "foo"))

	generalMsgs := store.List("general")
	if len(generalMsgs) != 2 {
		t.Fatalf("expected 2 messages in general, got %d", len(generalMsgs))
	}

	randomMsgs := store.List("random")
	if len(randomMsgs) != 1 {
		t.Fatalf("expected 1 message in random, got %d", len(randomMsgs))
	}
}

func TestMemoryStore_Channels(t *testing.T) {
	store := NewMemoryStore(10)
	store.Add(newTestMessage("1", "general", "hello"))
	store.Add(newTestMessage("2", "random", "world"))
	store.Add(newTestMessage("3", "general", "foo"))

	channels := store.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 unique channels, got %d", len(channels))
	}

	// insertion order: general first, then random
	if channels[0] != "general" {
		t.Errorf("expected first channel=general, got %s", channels[0])
	}
	if channels[1] != "random" {
		t.Errorf("expected second channel=random, got %s", channels[1])
	}
}

func TestMemoryStore_Clear(t *testing.T) {
	store := NewMemoryStore(10)
	store.Add(newTestMessage("1", "general", "hello"))
	store.Add(newTestMessage("2", "random", "world"))

	store.Clear()

	msgs := store.List("")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after Clear, got %d", len(msgs))
	}

	channels := store.Channels()
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after Clear, got %d", len(channels))
	}
}

func TestMemoryStore_MaxMessages(t *testing.T) {
	store := NewMemoryStore(3)
	store.Add(newTestMessage("1", "general", "msg1"))
	store.Add(newTestMessage("2", "general", "msg2"))
	store.Add(newTestMessage("3", "general", "msg3"))
	store.Add(newTestMessage("4", "general", "msg4"))
	store.Add(newTestMessage("5", "general", "msg5"))

	msgs := store.List("")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (max), got %d", len(msgs))
	}

	// should retain the last 3 messages
	if msgs[0].ID != "3" {
		t.Errorf("expected first message ID=3, got %s", msgs[0].ID)
	}
	if msgs[1].ID != "4" {
		t.Errorf("expected second message ID=4, got %s", msgs[1].ID)
	}
	if msgs[2].ID != "5" {
		t.Errorf("expected third message ID=5, got %s", msgs[2].ID)
	}
}
