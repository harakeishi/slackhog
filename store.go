package main

import "sync"

// MessageStore はメッセージの保存・取得インターフェース。
type MessageStore interface {
	Add(msg Message)
	List(channel string) []Message
	Channels() []string
	Clear()
}

// MemoryStore はメッセージをメモリ上に保持する MessageStore の実装。
type MemoryStore struct {
	mu      sync.Mutex
	msgs    []Message
	maxSize int
}

// NewMemoryStore は指定した最大保持数で MemoryStore を生成する。
// maxSize が 0 以下の場合は無制限。
func NewMemoryStore(maxSize int) *MemoryStore {
	return &MemoryStore{
		msgs:    []Message{},
		maxSize: maxSize,
	}
}

// Add はメッセージを追加する。maxSize を超えた場合、最古のメッセージを削除する。
func (s *MemoryStore) Add(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.msgs = append(s.msgs, msg)
	if s.maxSize > 0 && len(s.msgs) > s.maxSize {
		s.msgs = s.msgs[len(s.msgs)-s.maxSize:]
	}
}

// List はメッセージ一覧を返す。channel が空の場合は全件、指定した場合はそのチャンネルのみ。
func (s *MemoryStore) List(channel string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	if channel == "" {
		result := make([]Message, len(s.msgs))
		copy(result, s.msgs)
		return result
	}

	var result []Message
	for _, m := range s.msgs {
		if m.Channel == channel {
			result = append(result, m)
		}
	}
	return result
}

// Channels は保持しているメッセージのユニークなチャンネル名を挿入順で返す。
func (s *MemoryStore) Channels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]struct{})
	var channels []string
	for _, m := range s.msgs {
		if _, ok := seen[m.Channel]; !ok {
			seen[m.Channel] = struct{}{}
			channels = append(channels, m.Channel)
		}
	}
	return channels
}

// Clear は保持している全メッセージを削除する。
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.msgs = []Message{}
}
