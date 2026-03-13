package main

import (
	"fmt"
	"sync"
)

// MessageStore はメッセージの保存・取得インターフェース。
type MessageStore interface {
	Add(msg *Message)
	List(channel string) []Message
	Replies(threadTS string) []Message
	FindByTS(channel, ts string) (Message, bool)
	Update(channel, ts string, fn func(*Message)) bool
	Channels() []string
	Clear()
}

// MemoryStore はメッセージをメモリ上に保持する MessageStore の実装。
type MemoryStore struct {
	mu              sync.Mutex
	msgs            []Message
	maxSize         int
	initialChannels []string
}

// NewMemoryStore は指定した最大保持数で MemoryStore を生成する。
// maxSize が 0 以下の場合は無制限。
func NewMemoryStore(maxSize int) *MemoryStore {
	return &MemoryStore{
		msgs:    []Message{},
		maxSize: maxSize,
	}
}

// SetInitialChannels は起動時の初期チャンネルを設定する。
func (s *MemoryStore) SetInitialChannels(channels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialChannels = channels
}

// Add はメッセージを追加する。maxSize を超えた場合、最古のメッセージを削除する。
// スレッド返信の場合、ThreadTS（Slack形式のts値）を親メッセージのIDに変換して紐づける。
// ポインタを受け取り、呼び出し元のメッセージも更新する（Broadcast用）。
func (s *MemoryStore) Add(msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// スレッド返信の場合、thread_ts から親メッセージのIDを逆引きする
	if msg.ThreadTS != "" {
		for _, m := range s.msgs {
			if m.ThreadTS != "" {
				continue // 返信メッセージはスキップ
			}
			msgTS := fmt.Sprintf("%d.%06d", m.ReceivedAt.Unix(), m.ReceivedAt.Nanosecond()/1000)
			if msgTS == msg.ThreadTS && m.Channel == msg.Channel {
				msg.ThreadTS = m.ID
				break
			}
		}
	}

	s.msgs = append(s.msgs, *msg)
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

// Channels は保持しているメッセージのユニークなチャンネル名を返す。
// 初期チャンネルが設定されている場合、それらを先頭に含める。
func (s *MemoryStore) Channels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]struct{})
	var channels []string

	// 初期チャンネルを先に追加
	for _, ch := range s.initialChannels {
		if _, ok := seen[ch]; !ok {
			seen[ch] = struct{}{}
			channels = append(channels, ch)
		}
	}

	// メッセージ由来のチャンネルを追加
	for _, m := range s.msgs {
		if _, ok := seen[m.Channel]; !ok {
			seen[m.Channel] = struct{}{}
			channels = append(channels, m.Channel)
		}
	}
	return channels
}

// FindByTS はチャンネルとタイムスタンプでメッセージを検索する。
func (s *MemoryStore) FindByTS(channel, ts string) (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range s.msgs {
		msgTS := fmt.Sprintf("%d.%06d", m.ReceivedAt.Unix(), m.ReceivedAt.Nanosecond()/1000)
		if m.Channel == channel && msgTS == ts {
			return m, true
		}
	}
	return Message{}, false
}

// Update はチャンネルとタイムスタンプで一致するメッセージをコールバックで更新する。
func (s *MemoryStore) Update(channel, ts string, fn func(*Message)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.msgs {
		m := &s.msgs[i]
		msgTS := fmt.Sprintf("%d.%06d", m.ReceivedAt.Unix(), m.ReceivedAt.Nanosecond()/1000)
		if m.Channel == channel && msgTS == ts {
			fn(m)
			return true
		}
	}
	return false
}

// Clear は保持している全メッセージを削除する。
func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.msgs = []Message{}
}
