package main

import "sync"

// MessageStore はメッセージの保存・取得インターフェース。
type MessageStore interface {
	Add(msg Message)
	List(channel string) []Message
	Replies(threadTS string) []Message
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

// List はトップレベルメッセージ一覧を返す（スレッド返信は除外）。
// channel が空の場合は全件、指定した場合はそのチャンネルのみ。
// 各トップレベルメッセージの ReplyCount には返信数がセットされる。
func (s *MemoryStore) List(channel string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 返信数をカウントするマップを構築
	replyCounts := make(map[string]int)
	for _, m := range s.msgs {
		if m.ThreadTS != "" {
			replyCounts[m.ThreadTS]++
		}
	}

	result := make([]Message, 0)
	for _, m := range s.msgs {
		if m.ThreadTS != "" {
			continue // 返信メッセージは除外
		}
		if channel != "" && m.Channel != channel {
			continue
		}
		m.ReplyCount = replyCounts[m.ID]
		result = append(result, m)
	}
	return result
}

// Replies は指定した threadTS を持つ返信メッセージ一覧を返す。
func (s *MemoryStore) Replies(threadTS string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Message, 0)
	for _, m := range s.msgs {
		if m.ThreadTS == threadTS {
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
