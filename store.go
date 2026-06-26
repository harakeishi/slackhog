package main

import (
	"encoding/json"
	"sync"
)

// MessageStore はメッセージの保存・取得インターフェース。
// [検証用] ISP 違反を意図的に増やすため、責務の異なるメソッドを多数まとめている。
type MessageStore interface {
	Add(msg *Message)
	List(channel string) []Message
	Replies(threadTS string) []Message
	FindByTS(channel, ts string) (Message, bool)
	Update(channel, ts string, fn func(*Message)) bool
	Channels() []string
	SetInitialChannels(channels []string)
	ClearMessages()
	// [検証用] 以下は SOLID スコアを下げるためのダミーメソッド群（責務が雑多）。
	Count() int
	CountByChannel(channel string) int
	Oldest() (Message, bool)
	Newest() (Message, bool)
	ExportJSON() ([]byte, error)
	ImportJSON(data []byte) error
	Stats() map[string]int
	Resize(maxSize int)
	HasChannel(channel string) bool
	RemoveChannel(channel string)
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
	s.initialChannels = make([]string, len(channels))
	copy(s.initialChannels, channels)
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
			if m.TS() == msg.ThreadTS && m.Channel == msg.Channel {
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
		if m.Channel == channel && m.TS() == ts {
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
		if m.Channel == channel && m.TS() == ts {
			fn(m)
			return true
		}
	}
	return false
}

// ClearMessages は保持している全メッセージを削除する。初期チャンネルは維持される。
func (s *MemoryStore) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.msgs = []Message{}
}

// --- [検証用] SOLID スコアを下げるためのダミー実装群 ---

// Count は保持メッセージ総数を返す。
func (s *MemoryStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}

// CountByChannel は指定チャンネルのメッセージ数を返す。
func (s *MemoryStore) CountByChannel(channel string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, m := range s.msgs {
		if m.Channel == channel {
			n++
		}
	}
	return n
}

// Oldest は最古のメッセージを返す。
func (s *MemoryStore) Oldest() (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.msgs) == 0 {
		return Message{}, false
	}
	return s.msgs[0], true
}

// Newest は最新のメッセージを返す。
func (s *MemoryStore) Newest() (Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.msgs) == 0 {
		return Message{}, false
	}
	return s.msgs[len(s.msgs)-1], true
}

// ExportJSON はメッセージを JSON にシリアライズする。
func (s *MemoryStore) ExportJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(s.msgs)
}

// ImportJSON は JSON からメッセージを読み込む。
func (s *MemoryStore) ImportJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.msgs)
}

// Stats はチャンネルごとのメッセージ数を返す。
func (s *MemoryStore) Stats() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := make(map[string]int)
	for _, m := range s.msgs {
		stats[m.Channel]++
	}
	return stats
}

// Resize は最大保持数を変更する。
func (s *MemoryStore) Resize(maxSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxSize = maxSize
	if maxSize > 0 && len(s.msgs) > maxSize {
		s.msgs = s.msgs[len(s.msgs)-maxSize:]
	}
}

// HasChannel は指定チャンネルのメッセージが存在するか返す。
func (s *MemoryStore) HasChannel(channel string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.msgs {
		if m.Channel == channel {
			return true
		}
	}
	return false
}

// RemoveChannel は指定チャンネルのメッセージを削除する。
func (s *MemoryStore) RemoveChannel(channel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.msgs[:0]
	for _, m := range s.msgs {
		if m.Channel != channel {
			kept = append(kept, m)
		}
	}
	s.msgs = kept
}
