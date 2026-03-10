# Thread Support Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Slack スレッド機能をサポートし、`thread_ts` パラメータによるスレッド返信の受信・表示を可能にする。

**Architecture:** `Message` に `ThreadTS` を追加。`MessageStore` インターフェースに `Replies(threadTS)` を追加。内部APIに `/_api/messages/{id}/replies` エンドポイントを追加。Web UI にスレッドパネル（右側スライド）を追加。

**Tech Stack:** Go (既存), 素のHTML/CSS/JS (既存)

---

## File Structure

| File | Change | Responsibility |
|------|--------|---------------|
| `message.go` | Modify | `ThreadTS`, `ReplyCount` フィールド追加 |
| `store.go` | Modify | `MessageStore` に `Replies` メソッド追加、`MemoryStore` で実装 |
| `store_test.go` | Modify | スレッド関連テスト追加 |
| `handler_slack.go` | Modify | `buildMessage` で `thread_ts` を抽出 |
| `handler_slack_test.go` | Modify | `thread_ts` 付きテスト追加 |
| `handler_internal.go` | Modify | `/_api/messages/{id}/replies` エンドポイント追加 |
| `handler_internal_test.go` | Modify | replies エンドポイントのテスト追加 |
| `server.go` | Modify | `InternalAPI` に `HandleReplies` 追加、ルート登録 |
| `ui/app.js` | Modify | スレッドパネルUI、返信数バッジ |
| `ui/style.css` | Modify | スレッドパネルのスタイル |
| `ui/index.html` | Modify | スレッドパネルのDOM |

---

## Chunk 1: Backend

### Task 1: Message にスレッドフィールド追加

**Files:**
- Modify: `message.go`

- [ ] **Step 1: `ThreadTS` と `ReplyCount` フィールドを追加**

```go
type Message struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Username    string    `json:"username"`
	Text        string    `json:"text"`
	ThreadTS    string    `json:"thread_ts,omitempty"`
	ReplyCount  int       `json:"reply_count"`
	IconEmoji   string    `json:"icon_emoji,omitempty"`
	IconURL     string    `json:"icon_url,omitempty"`
	Blocks      any       `json:"blocks,omitempty"`
	Attachments any       `json:"attachments,omitempty"`
	ReceivedAt  time.Time `json:"received_at"`
	RawPayload  any       `json:"raw_payload"`
}
```

- [ ] **Step 2: コンパイル確認**

Run: `go vet ./...`
Expected: エラーなし

- [ ] **Step 3: Commit**

```bash
git add message.go
git commit -m "feat: add ThreadTS and ReplyCount fields to Message"
```

---

### Task 2: MessageStore にスレッド機能追加 (TDD)

**Files:**
- Modify: `store.go`
- Modify: `store_test.go`

- [ ] **Step 1: store_test.go にスレッドテストを追加**

```go
func TestMemoryStore_ThreadReplyCount(t *testing.T) {
	s := NewMemoryStore(100)
	// 親メッセージ
	s.Add(Message{ID: "parent1", Channel: "general", Text: "parent"})
	// 返信
	s.Add(Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1"})
	s.Add(Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1"})

	// List はスレッド返信を除外し、親メッセージの ReplyCount を設定する
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
	s.Add(Message{ID: "parent1", Channel: "general", Text: "parent"})
	s.Add(Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1"})
	s.Add(Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1"})
	s.Add(Message{ID: "reply3", Channel: "general", Text: "other", ThreadTS: "parent2"})

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
	s.Add(Message{ID: "msg1", Channel: "general", Text: "normal"})
	s.Add(Message{ID: "msg2", Channel: "general", Text: "parent"})
	s.Add(Message{ID: "reply1", Channel: "general", Text: "reply", ThreadTS: "msg2"})

	msgs := s.List("general")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 top-level messages, got %d", len(msgs))
	}
}
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run "TestMemoryStore_(Thread|Replies|ListExcludes)"`
Expected: FAIL

- [ ] **Step 3: MessageStore インターフェースに Replies を追加し、MemoryStore を実装**

`store.go` の `MessageStore` インターフェースに追加:
```go
type MessageStore interface {
	Add(msg Message)
	List(channel string) []Message
	Channels() []string
	Replies(threadTS string) []Message
	Clear()
}
```

`MemoryStore.List` を修正: ThreadTS が空のメッセージ（トップレベル）のみ返し、各親の ReplyCount を計算。

`MemoryStore.Replies` を追加: 指定 threadTS に一致する返信を返す。

- [ ] **Step 4: テストがパスすることを確認**

Run: `go test ./... -v -run "TestMemoryStore"`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add store.go store_test.go
git commit -m "feat: add thread support to MessageStore (Replies, filtered List)"
```

---

### Task 3: handler_slack.go で thread_ts を処理 (TDD)

**Files:**
- Modify: `handler_slack.go`
- Modify: `handler_slack_test.go`

- [ ] **Step 1: handler_slack_test.go にスレッドテストを追加**

```go
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
	// 返信として保存されている
	replies := store.Replies("parent-id")
	if len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(replies))
	}
	if replies[0].ThreadTS != "parent-id" {
		t.Fatalf("expected ThreadTS='parent-id', got %q", replies[0].ThreadTS)
	}
}
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run TestHandleChatPostMessage_WithThreadTS`
Expected: FAIL（Replies メソッドがmockにない、または ThreadTS が空）

- [ ] **Step 3: buildMessage で thread_ts を抽出するよう修正**

`handler_slack.go` の `buildMessage` に `ThreadTS: str("thread_ts")` を追加。

- [ ] **Step 4: handler_slack_test.go の mockBroadcaster の下に mockStore を用意するか、既存テストが Replies を使えるよう修正**

既存テストは `NewMemoryStore` を使っているため、MemoryStore に Replies が実装済みであれば追加修正不要。テストがパスすることを確認。

Run: `go test ./... -v -run TestHandle`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add handler_slack.go handler_slack_test.go
git commit -m "feat: extract thread_ts in Slack handler"
```

---

### Task 4: 内部API に replies エンドポイント追加 (TDD)

**Files:**
- Modify: `handler_internal.go`
- Modify: `handler_internal_test.go`
- Modify: `server.go`

- [ ] **Step 1: handler_internal_test.go にテストを追加**

```go
func TestHandleReplies(t *testing.T) {
	store := NewMemoryStore(100)
	store.Add(Message{ID: "parent1", Channel: "general", Text: "parent", ReceivedAt: time.Now()})
	store.Add(Message{ID: "reply1", Channel: "general", Text: "reply 1", ThreadTS: "parent1", ReceivedAt: time.Now()})
	store.Add(Message{ID: "reply2", Channel: "general", Text: "reply 2", ThreadTS: "parent1", ReceivedAt: time.Now()})
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
```

- [ ] **Step 2: テストが失敗することを確認**

Run: `go test ./... -v -run TestHandleReplies`
Expected: FAIL (HandleReplies 未定義)

- [ ] **Step 3: handler_internal.go に HandleReplies を実装**

```go
func (h *InternalHandler) HandleReplies(w http.ResponseWriter, r *http.Request, parentID string) {
	replies := h.store.Replies(parentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"parent_id": parentID,
		"replies":   replies,
	})
}
```

- [ ] **Step 4: server.go の InternalAPI インターフェースと NewServer にルートを追加**

`InternalAPI` に `HandleReplies` を追加:
```go
type InternalAPI interface {
	HandleMessages(w http.ResponseWriter, r *http.Request)
	HandleReplies(w http.ResponseWriter, r *http.Request, parentID string)
}
```

`NewServer` にルートを追加:
```go
mux.HandleFunc("/_api/messages/", func(w http.ResponseWriter, r *http.Request) {
	// /_api/messages/{id}/replies のパスからIDを抽出
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/_api/messages/"), "/")
	if len(parts) == 2 && parts[1] == "replies" {
		internal.HandleReplies(w, r, parts[0])
		return
	}
	http.NotFound(w, r)
})
```

- [ ] **Step 5: テストがパスすることを確認**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add handler_internal.go handler_internal_test.go server.go
git commit -m "feat: add /_api/messages/{id}/replies endpoint"
```

---

## Chunk 2: Frontend

### Task 5: Web UI にスレッドパネル追加

**Files:**
- Modify: `ui/index.html`
- Modify: `ui/style.css`
- Modify: `ui/app.js`

- [ ] **Step 1: index.html にスレッドパネルのDOMを追加**

`#main` の後にスレッドパネルを追加:
```html
<div id="thread-panel" class="thread-panel hidden">
  <div id="thread-header">
    <span id="thread-header-title">Thread</span>
    <button id="thread-close-btn">✕</button>
  </div>
  <div id="thread-parent"></div>
  <div id="thread-divider"><span>Replies</span></div>
  <div id="thread-replies"></div>
</div>
```

- [ ] **Step 2: style.css にスレッドパネルのスタイルを追加**

- `.thread-panel`: 右側固定幅(400px)、ボーダー左、背景色、flex column
- `.thread-panel.hidden`: display:none
- `#thread-header`: ヘッダー（タイトル + 閉じるボタン）
- `#thread-parent`: 親メッセージ表示エリア
- `#thread-divider`: 「Replies」区切り線
- `#thread-replies`: 返信メッセージリスト（スクロール可）
- `.message .reply-badge`: メッセージ内の「N件の返信」バッジ

- [ ] **Step 3: app.js にスレッド機能を追加**

主な変更:
1. メッセージ要素に `reply_count > 0` の場合「N件の返信」バッジを表示
2. バッジクリックで `openThread(parentID)` を呼び出し
3. `openThread(parentID)`: `/_api/messages/{id}/replies` をフェッチし、スレッドパネルに表示
4. `closeThread()`: パネルを非表示
5. WebSocket受信時、スレッド返信なら開いているスレッドパネルに追記

- [ ] **Step 4: ビルド・起動して動作確認**

Run: `go build -o slackhog . && ./slackhog`
テスト手順:
1. 親メッセージを送信
2. `thread_ts` 付きで返信を送信
3. 親メッセージに「N件の返信」バッジが表示されることを確認
4. クリックでスレッドパネルが開き、返信が表示されることを確認

- [ ] **Step 5: Commit**

```bash
git add ui/index.html ui/style.css ui/app.js
git commit -m "feat: add thread panel UI with reply count badges"
```

---

### Task 6: 全テスト + 最終確認

- [ ] **Step 1: 全テスト実行**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: go vet**

Run: `go vet ./...`
Expected: エラーなし

- [ ] **Step 3: E2E動作確認**

```bash
# 親メッセージ
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"channel":"general","text":"これは親メッセージです","username":"bot"}' \
  http://localhost:4112/api/chat.postMessage
# レスポンスの ts を控える → PARENT_TS

# スレッド返信
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"channel":"general","text":"スレッド返信1","username":"bot","thread_ts":"PARENT_ID"}' \
  http://localhost:4112/api/chat.postMessage

# replies API
curl -s http://localhost:4112/_api/messages/PARENT_ID/replies
```

- [ ] **Step 4: Commit（必要な場合）**

```bash
git add -A
git commit -m "chore: final cleanup for thread support"
```
