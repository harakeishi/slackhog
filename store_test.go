package main

import (
	"fmt"
	"testing"
	"time"
)

func newTestMessage(id, channel, text string) *Message {
	return &Message{
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

func TestMemoryStore_ClearMessages(t *testing.T) {
	store := NewMemoryStore(10)
	store.Add(newTestMessage("1", "general", "hello"))
	store.Add(newTestMessage("2", "random", "world"))

	store.ClearMessages()

	msgs := store.List("")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after ClearMessages, got %d", len(msgs))
	}

	channels := store.Channels()
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after ClearMessages, got %d", len(channels))
	}
}

func TestMemoryStore_ClearMessages_PreservesInitialChannels(t *testing.T) {
	store := NewMemoryStore(10)
	store.SetInitialChannels([]string{"general", "random"})
	store.Add(newTestMessage("1", "general", "hello"))
	store.Add(newTestMessage("2", "alerts", "world"))

	store.ClearMessages()

	msgs := store.List("")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after ClearMessages, got %d", len(msgs))
	}

	// 初期チャンネルは維持される
	channels := store.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 initial channels after ClearMessages, got %d", len(channels))
	}
	if channels[0] != "general" || channels[1] != "random" {
		t.Errorf("expected [general random], got %v", channels)
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

func TestMemoryStore_ThreadReplyCount(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(&Message{ID: "parent1", Channel: "general", Text: "parent"})
	s.Add(&Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1"})
	s.Add(&Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1"})

	msgs := s.List("")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 top-level message, got %d", len(msgs))
	}
	if msgs[0].ReplyCount != 2 {
		t.Fatalf("expected ReplyCount=2, got %d", msgs[0].ReplyCount)
	}
}

func TestMemoryStore_Replies(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(&Message{ID: "parent1", Channel: "general", Text: "parent"})
	s.Add(&Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1"})
	s.Add(&Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1"})
	s.Add(&Message{ID: "reply3", Channel: "general", Text: "other", ThreadTS: "parent2"})

	replies := s.Replies("parent1")
	if len(replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(replies))
	}
	if replies[0].ID != "reply1" {
		t.Errorf("expected first reply ID=reply1, got %s", replies[0].ID)
	}
}

func TestMemoryStore_ListExcludesReplies(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(&Message{ID: "msg1", Channel: "general", Text: "normal"})
	s.Add(&Message{ID: "msg2", Channel: "general", Text: "parent"})
	s.Add(&Message{ID: "reply1", Channel: "general", Text: "reply", ThreadTS: "msg2"})

	msgs := s.List("general")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 top-level messages, got %d", len(msgs))
	}
}

func TestMemoryStore_FindByTS(t *testing.T) {
	s := NewMemoryStore(100)

	msg := Message{ID: "abc", Channel: "general", Text: "hello", ReceivedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s.Add(&msg)

	ts := fmt.Sprintf("%d.%06d", msg.ReceivedAt.Unix(), msg.ReceivedAt.Nanosecond()/1000)

	found, ok := s.FindByTS("general", ts)
	if !ok {
		t.Fatal("expected to find message by ts")
	}
	if found.ID != "abc" {
		t.Fatalf("expected ID 'abc', got %q", found.ID)
	}

	_, ok = s.FindByTS("general", "9999999999.000000")
	if ok {
		t.Fatal("expected not found for non-existent ts")
	}

	_, ok = s.FindByTS("other", ts)
	if ok {
		t.Fatal("expected not found for wrong channel")
	}
}

func TestMemoryStore_InitialChannels(t *testing.T) {
	store := NewMemoryStore(100)
	store.SetInitialChannels([]string{"general", "random"})

	channels := store.Channels()
	if len(channels) != 2 {
		t.Fatalf("Channels length = %d, want 2", len(channels))
	}
	if channels[0] != "general" || channels[1] != "random" {
		t.Errorf("Channels = %v, want [general random]", channels)
	}
}

func TestMemoryStore_InitialChannels_MergedWithMessages(t *testing.T) {
	store := NewMemoryStore(100)
	store.SetInitialChannels([]string{"general", "random"})

	store.Add(&Message{ID: "1", Channel: "alerts", Text: "test"})
	store.Add(&Message{ID: "2", Channel: "general", Text: "hello"})

	channels := store.Channels()
	if len(channels) != 3 {
		t.Fatalf("Channels length = %d, want 3", len(channels))
	}
	if channels[0] != "general" {
		t.Errorf("Channels[0] = %q, want general", channels[0])
	}
	if channels[1] != "random" {
		t.Errorf("Channels[1] = %q, want random", channels[1])
	}
	if channels[2] != "alerts" {
		t.Errorf("Channels[2] = %q, want alerts", channels[2])
	}
}

func TestMemoryStore_Update(t *testing.T) {
	s := NewMemoryStore(100)

	msg := Message{ID: "abc", Channel: "general", Text: "original", ReceivedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s.Add(&msg)

	ts := fmt.Sprintf("%d.%06d", msg.ReceivedAt.Unix(), msg.ReceivedAt.Nanosecond()/1000)

	ok := s.Update("general", ts, func(m *Message) {
		m.Text = "updated"
	})
	if !ok {
		t.Fatal("expected update to succeed")
	}

	found, _ := s.FindByTS("general", ts)
	if found.Text != "updated" {
		t.Fatalf("expected text 'updated', got %q", found.Text)
	}

	ok = s.Update("general", "9999999999.000000", func(m *Message) {
		m.Text = "nope"
	})
	if ok {
		t.Fatal("expected update to fail for non-existent ts")
	}
}
