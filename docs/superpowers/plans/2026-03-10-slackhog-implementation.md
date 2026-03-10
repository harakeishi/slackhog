# SlackHog Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** MailHogのSlack版 — ローカルでSlack APIリクエストをキャッチし、Slack風Web UIでリアルタイム確認できる開発用ツールを実装する。

**Architecture:** 単一バイナリ・単一ポート(4112)でSlack API互換エンドポイントとWeb UIを提供。メッセージはインメモリ保持。ハンドラーは `MessageStore` / `Broadcaster` インターフェースに依存し、`main.go` でDI。Go `embed` でフロントエンド同梱。

**Tech Stack:** Go (標準ライブラリ + gorilla/websocket), 素のHTML/CSS/JS

---

## File Structure

| File | Responsibility |
|------|---------------|
| `main.go` | エントリーポイント、CLIフラグ解析、DI |
| `message.go` | `Message` データモデル |
| `store.go` | `MessageStore` インターフェース + `MemoryStore` 実装 |
| `store_test.go` | MemoryStore のテスト |
| `websocket.go` | `Broadcaster` インターフェース + `WebSocketHub` 実装 |
| `websocket_test.go` | WebSocketHub のテスト |
| `handler_slack.go` | Slack API互換エンドポイント (`/api/chat.postMessage`, `/services/:id`) |
| `handler_slack_test.go` | Slack API ハンドラーのテスト |
| `handler_internal.go` | 内部API (`/_api/messages`) |
| `handler_internal_test.go` | 内部API ハンドラーのテスト |
| `server.go` | HTTPサーバー、ルーティング |
| `server_test.go` | サーバー統合テスト |
| `ui/index.html` | Web UI HTML |
| `ui/style.css` | Slackダークテーマ風CSS |
| `ui/app.js` | フロントエンドJS (WebSocket, レンダリング) |
| `go.mod` | Goモジュール定義 |

---

## Chunk 1: Core Domain (message, store)

### Task 1: プロジェクト初期化

**Files:**
- Create: `go.mod`
- Create: `message.go`

- [ ] **Step 1: Go module 初期化**

Run: `cd /Users/keishi.hara/src/github.com/harakeishi/slackhog && go mod init github.com/harakeishi/slackhog`

- [ ] **Step 2: message.go を作成**

```go
package main

import "time"

// Message はSlackから受信したメッセージを表す。
type Message struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Username    string    `json:"username"`
	Text        string    `json:"text"`
	IconEmoji   string    `json:"icon_emoji,omitempty"`
	IconURL     string    `json:"icon_url,omitempty"`
	Blocks      any       `json:"blocks,omitempty"`
	Attachments any       `json:"attachments,omitempty"`
	ReceivedAt  time.Time `json:"received_at"`
	RawPayload  any       `json:"raw_payload"`
}
```

- [ ] **Step 3: コンパイル確認**

Run: `go build ./...`
Expected: エラーなし（main関数がないのでバイナリは生成されないが構文チェックは通る）

注: `go build ./...` は main 関数がなくてもパッケージのコンパイルチェックを行う。ただし `package main` でmain関数がない場合エラーになるため、 `go vet ./...` で構文チェックする。

- [ ] **Step 4: Commit**

```bash
git add go.mod message.go
git commit -m "feat: initialize go module and add Message data model"
```

---

### Task 2: MessageStore インターフェース + MemoryStore テスト

**Files:**
- Create: `store.go`
- Create: `store_test.go`

- [ ] **Step 1: store_test.go にテストを書く**

```go
package main

import (
	"testing"
	"time"
)

func TestMemoryStore_AddAndList(t *testing.T) {
	s := NewMemoryStore(100)
	msg := Message{
		ID:         "1",
		Channel:    "general",
		Username:   "bot",
		Text:       "hello",
		ReceivedAt: time.Now(),
	}
	s.Add(msg)

	msgs := s.List("")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Text != "hello" {
		t.Fatalf("expected 'hello', got %q", msgs[0].Text)
	}
}

func TestMemoryStore_ListByChannel(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(Message{ID: "1", Channel: "general", Text: "a", ReceivedAt: time.Now()})
	s.Add(Message{ID: "2", Channel: "alerts", Text: "b", ReceivedAt: time.Now()})
	s.Add(Message{ID: "3", Channel: "general", Text: "c", ReceivedAt: time.Now()})

	msgs := s.List("general")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages for general, got %d", len(msgs))
	}
}

func TestMemoryStore_Channels(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(Message{ID: "1", Channel: "general", ReceivedAt: time.Now()})
	s.Add(Message{ID: "2", Channel: "alerts", ReceivedAt: time.Now()})
	s.Add(Message{ID: "3", Channel: "general", ReceivedAt: time.Now()})

	channels := s.Channels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
}

func TestMemoryStore_Clear(t *testing.T) {
	s := NewMemoryStore(100)
	s.Add(Message{ID: "1", Channel: "general", ReceivedAt: time.Now()})
	s.Clear()

	msgs := s.List("")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
	channels := s.Channels()
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after clear, got %d", len(channels))
	}
}

func TestMemoryStore_MaxMessages(t *testing.T) {
	s := NewMemoryStore(3)
	for i := range 5 {
		s.Add(Message{ID: string(rune('1' + i)), Channel: "general", Text: string(rune('a' + i)), ReceivedAt: time.Now()})
	}

	msgs := s.List("")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (max), got %d", len(msgs))
	}
	// 最も古いメッセージが削除されている
	if msgs[0].Text != "c" {
		t.Fatalf("expected oldest remaining to be 'c', got %q", msgs[0].Text)
	}
}
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run TestMemoryStore`
Expected: FAIL (NewMemoryStore が未定義)

- [ ] **Step 3: store.go を実装**

```go
package main

import "sync"

// MessageStore はメッセージの保存・取得・削除を抽象化するインターフェース。
type MessageStore interface {
	Add(msg Message)
	List(channel string) []Message
	Channels() []string
	Clear()
}

// MemoryStore はインメモリのMessageStore実装。
type MemoryStore struct {
	mu       sync.RWMutex
	messages []Message
	maxSize  int
}

// NewMemoryStore は指定した最大件数のMemoryStoreを生成する。
func NewMemoryStore(maxSize int) *MemoryStore {
	return &MemoryStore{
		messages: make([]Message, 0),
		maxSize:  maxSize,
	}
}

func (s *MemoryStore) Add(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	if len(s.messages) > s.maxSize {
		s.messages = s.messages[len(s.messages)-s.maxSize:]
	}
}

func (s *MemoryStore) List(channel string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if channel == "" {
		result := make([]Message, len(s.messages))
		copy(result, s.messages)
		return result
	}
	var result []Message
	for _, m := range s.messages {
		if m.Channel == channel {
			result = append(result, m)
		}
	}
	return result
}

func (s *MemoryStore) Channels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var channels []string
	for _, m := range s.messages {
		if !seen[m.Channel] {
			seen[m.Channel] = true
			channels = append(channels, m.Channel)
		}
	}
	return channels
}

func (s *MemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
}
```

- [ ] **Step 4: テストがパスすることを確認**

Run: `go test ./... -v -run TestMemoryStore`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add store.go store_test.go
git commit -m "feat: add MessageStore interface and MemoryStore implementation"
```

---

## Chunk 2: WebSocket + Handlers

### Task 3: Broadcaster インターフェース + WebSocketHub

**Files:**
- Create: `websocket.go`

- [ ] **Step 1: websocket.go を実装**

```go
package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Broadcaster はWebSocket経由のメッセージ配信を抽象化するインターフェース。
type Broadcaster interface {
	Broadcast(msg Message)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebSocketHub はWebSocket接続を管理し、メッセージをブロードキャストする。
type WebSocketHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

// NewWebSocketHub は新しいWebSocketHubを生成する。
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients: make(map[*websocket.Conn]bool),
	}
}

// HandleWS はWebSocket接続のHTTPハンドラー。
func (h *WebSocketHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	// 切断検知のためにreadを待つ
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// Broadcast は接続中の全クライアントにメッセージを送信する。
func (h *WebSocketHub) Broadcast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}
```

- [ ] **Step 2: gorilla/websocket 依存を追加**

Run: `go get github.com/gorilla/websocket`

- [ ] **Step 3: コンパイル確認**

Run: `go vet ./...`
Expected: エラーなし

- [ ] **Step 4: Commit**

```bash
git add websocket.go go.mod go.sum
git commit -m "feat: add Broadcaster interface and WebSocketHub implementation"
```

---

### Task 4: Slack API互換ハンドラー

**Files:**
- Create: `handler_slack.go`
- Create: `handler_slack_test.go`

- [ ] **Step 1: handler_slack_test.go にテストを書く**

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// mockBroadcaster はテスト用のBroadcaster実装。
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
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run "TestHandle(Chat|Incoming)"`
Expected: FAIL (NewSlackHandler が未定義)

- [ ] **Step 3: handler_slack.go を実装**

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// SlackHandler はSlack API互換エンドポイントを処理する。
type SlackHandler struct {
	store       MessageStore
	broadcaster Broadcaster
}

// NewSlackHandler は新しいSlackHandlerを生成する。
func NewSlackHandler(store MessageStore, broadcaster Broadcaster) *SlackHandler {
	return &SlackHandler{store: store, broadcaster: broadcaster}
}

// HandleChatPostMessage は /api/chat.postMessage を処理する。
func (h *SlackHandler) HandleChatPostMessage(w http.ResponseWriter, r *http.Request) {
	var channel, text, username, iconEmoji, iconURL string
	var blocks, attachments any
	var rawPayload any

	contentType := r.Header.Get("Content-Type")

	if contentType == "application/json" || contentType == "application/json; charset=utf-8" {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		rawPayload = payload
		channel, _ = payload["channel"].(string)
		text, _ = payload["text"].(string)
		username, _ = payload["username"].(string)
		iconEmoji, _ = payload["icon_emoji"].(string)
		iconURL, _ = payload["icon_url"].(string)
		blocks = payload["blocks"]
		attachments = payload["attachments"]
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		channel = r.FormValue("channel")
		text = r.FormValue("text")
		username = r.FormValue("username")
		iconEmoji = r.FormValue("icon_emoji")
		iconURL = r.FormValue("icon_url")
		rawPayload = r.Form
	}

	msg := Message{
		ID:          uuid.New().String(),
		Channel:     channel,
		Username:    username,
		Text:        text,
		IconEmoji:   iconEmoji,
		IconURL:     iconURL,
		Blocks:      blocks,
		Attachments: attachments,
		ReceivedAt:  time.Now(),
		RawPayload:  rawPayload,
	}

	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"channel": channel,
		"ts":      fmt.Sprintf("%d.%06d", msg.ReceivedAt.Unix(), msg.ReceivedAt.Nanosecond()/1000),
	})
}

// HandleIncomingWebhook は /services/:id を処理する。
func (h *SlackHandler) HandleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	text, _ := payload["text"].(string)
	channel, _ := payload["channel"].(string)
	if channel == "" {
		channel = "webhook"
	}
	username, _ := payload["username"].(string)
	if username == "" {
		username = "incoming-webhook"
	}
	iconEmoji, _ := payload["icon_emoji"].(string)
	iconURL, _ := payload["icon_url"].(string)

	msg := Message{
		ID:          uuid.New().String(),
		Channel:     channel,
		Username:    username,
		Text:        text,
		IconEmoji:   iconEmoji,
		IconURL:     iconURL,
		Blocks:      payload["blocks"],
		Attachments: payload["attachments"],
		ReceivedAt:  time.Now(),
		RawPayload:  payload,
	}

	h.store.Add(msg)
	h.broadcaster.Broadcast(msg)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
```

- [ ] **Step 4: uuid依存を追加してテストがパスすることを確認**

Run: `go get github.com/google/uuid && go test ./... -v -run "TestHandle(Chat|Incoming)"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add handler_slack.go handler_slack_test.go go.mod go.sum
git commit -m "feat: add Slack API compatible handlers (chat.postMessage, incoming webhook)"
```

---

### Task 5: 内部APIハンドラー

**Files:**
- Create: `handler_internal.go`
- Create: `handler_internal_test.go`

- [ ] **Step 1: handler_internal_test.go にテストを書く**

```go
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
	store.Add(Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
	store.Add(Message{ID: "2", Channel: "alerts", Text: "alert", ReceivedAt: time.Now()})
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
	store.Add(Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
	store.Add(Message{ID: "2", Channel: "alerts", Text: "alert", ReceivedAt: time.Now()})
	h := NewInternalHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/_api/messages?channel=general", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)

	var resp struct {
		Messages []Message `json:"messages"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message for general, got %d", len(resp.Messages))
	}
}

func TestHandleDeleteMessages(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(Message{ID: "1", Channel: "general", Text: "hello", ReceivedAt: time.Now()})
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
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run "TestHandle(Get|Delete)Messages"`
Expected: FAIL (NewInternalHandler が未定義)

- [ ] **Step 3: handler_internal.go を実装**

```go
package main

import (
	"encoding/json"
	"net/http"
)

// InternalHandler は内部API（/_api/*）を処理する。
type InternalHandler struct {
	store MessageStore
}

// NewInternalHandler は新しいInternalHandlerを生成する。
func NewInternalHandler(store MessageStore) *InternalHandler {
	return &InternalHandler{store: store}
}

// HandleMessages は /_api/messages へのGET/DELETEリクエストを処理する。
func (h *InternalHandler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channel := r.URL.Query().Get("channel")
		messages := h.store.List(channel)
		channels := h.store.Channels()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"messages": messages,
			"channels": channels,
		})

	case http.MethodDelete:
		h.store.Clear()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 4: テストがパスすることを確認**

Run: `go test ./... -v -run "TestHandle(Get|Delete)Messages"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add handler_internal.go handler_internal_test.go
git commit -m "feat: add internal API handler (/_api/messages GET/DELETE)"
```

---

## Chunk 3: Server, UI, Main

### Task 6: HTTPサーバー + ルーティング

**Files:**
- Create: `server.go`

- [ ] **Step 1: server.go を実装**

```go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

// Server はSlackHogのHTTPサーバー。
type Server struct {
	mux *http.ServeMux
}

// NewServer は新しいServerを生成し、ルーティングを設定する。
func NewServer(slackHandler *SlackHandler, internalHandler *InternalHandler, wsHub *WebSocketHub) *Server {
	mux := http.NewServeMux()

	// Slack API互換
	mux.HandleFunc("/api/chat.postMessage", slackHandler.HandleChatPostMessage)
	mux.HandleFunc("/services/", slackHandler.HandleIncomingWebhook)

	// 内部API
	mux.HandleFunc("/_api/messages", internalHandler.HandleMessages)

	// WebSocket
	mux.HandleFunc("/ws", wsHub.HandleWS)

	// Web UI (静的ファイル)
	uiContent, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(uiContent)))

	return &Server{mux: mux}
}

// ServeHTTP はhttp.Handler インターフェースを満たす。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
```

- [ ] **Step 2: コンパイル確認（uiディレクトリがないとembedエラーになるため、空ファイルを作成）**

Run: `mkdir -p ui && touch ui/.gitkeep && go vet ./...`
Expected: エラーなし

- [ ] **Step 3: Commit**

```bash
git add server.go ui/.gitkeep
git commit -m "feat: add HTTP server with routing"
```

---

### Task 7: Web UI (HTML/CSS/JS)

**Files:**
- Create: `ui/index.html`
- Create: `ui/style.css`
- Create: `ui/app.js`

- [ ] **Step 1: ui/index.html を作成**

Slack風ダークテーマのレイアウト。左サイドバーにチャンネル一覧、メインエリアにメッセージ一覧。

- [ ] **Step 2: ui/style.css を作成**

Slackダークテーマ風の配色。サイドバー、メッセージ、バッジのスタイル。

- [ ] **Step 3: ui/app.js を作成**

WebSocket接続、メッセージ取得・表示、チャンネル切替、Clear All機能、自動再接続。
Blocks (text, section, divider) と attachments (color bar, title, text, fields) の基本レンダリング。

- [ ] **Step 4: ブラウザ表示確認**

静的ファイルを直接ブラウザで開いてレイアウト崩れがないことを目視確認。

- [ ] **Step 5: Commit**

```bash
git add ui/index.html ui/style.css ui/app.js
git rm ui/.gitkeep
git commit -m "feat: add Slack-like dark theme Web UI"
```

---

### Task 8: main.go エントリーポイント

**Files:**
- Create: `main.go`

- [ ] **Step 1: main.go を実装**

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 4112, "listen port")
	maxMessages := flag.Int("max-messages", 1000, "maximum number of messages to keep")
	flag.Parse()

	store := NewMemoryStore(*maxMessages)
	hub := NewWebSocketHub()
	slackHandler := NewSlackHandler(store, hub)
	internalHandler := NewInternalHandler(store)
	server := NewServer(slackHandler, internalHandler, hub)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("SlackHog listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build -o slackhog .`
Expected: バイナリが生成される

- [ ] **Step 3: 起動確認**

Run: `./slackhog &` → `curl -s -X POST -d 'channel=general&text=hello&username=bot' http://localhost:4112/api/chat.postMessage` → `curl -s http://localhost:4112/_api/messages | jq .`
Expected: 送信したメッセージがJSON応答に含まれる

- [ ] **Step 4: 停止してCommit**

```bash
kill %1
git add main.go
git commit -m "feat: add main.go entrypoint with CLI flags and DI wiring"
```

---

### Task 9: 全テスト実行 + 最終確認

- [ ] **Step 1: 全テスト実行**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: go vet**

Run: `go vet ./...`
Expected: エラーなし

- [ ] **Step 3: ビルド確認**

Run: `go build -o slackhog .`
Expected: バイナリが生成される

- [ ] **Step 4: .gitkeep 削除確認、不要ファイルのクリーンアップ**

Run: `git status`

- [ ] **Step 5: 最終Commit（必要な場合）**

```bash
git add -A
git commit -m "chore: final cleanup"
```
