# SlackHog 設計書

MailHogのSlack版。ローカルでSlack APIリクエストをキャッチし、Slack風Web UIで確認できる開発用ツール。

## アーキテクチャ

```
[アプリ] --POST--> [SlackHog API Server :4112] --WebSocket--> [SlackHog Web UI :4112]
```

- 単一バイナリ、単一ポートでAPIサーバーとWeb UIの両方を提供
- メッセージはインメモリ保持（永続化なし、再起動でクリア）
- Go の `embed` パッケージでフロントエンド静的ファイルを同梱
- 依存ゼロ。バイナリをダウンロードまたは `go install` で即利用可能

## 技術スタック

- **言語**: Go
- **Web UI**: 素のHTML/CSS/JS（フレームワークなし、`embed`でバイナリに同梱）
- **リアルタイム更新**: WebSocket
- **外部依存**: 最小限（標準ライブラリ + WebSocketライブラリ程度）

## エンドポイント

### Slack API互換（アプリからのリクエスト受信用）

| メソッド | パス | 用途 |
|---------|------|------|
| POST | `/api/chat.postMessage` | Slack API互換。メッセージ受信 |
| POST | `/services/:id` | Incoming Webhook互換 |

### 内部API（Web UI用）

| メソッド | パス | 用途 |
|---------|------|------|
| GET | `/` | Web UI表示 |
| GET | `/ws` | WebSocket接続（リアルタイム更新） |
| GET | `/_api/messages` | 受信メッセージ一覧取得 |
| GET | `/_api/messages?channel=general` | チャンネル別フィルタ |
| DELETE | `/_api/messages` | メッセージ全クリア |

内部APIは `/_api` プレフィックスでSlack APIパスとの衝突を回避。

## データモデル

```go
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

## インターフェース

```go
// MessageStore はメッセージの保存・取得・削除を抽象化するインターフェース。
// ハンドラーはこのインターフェースに依存し、具体実装には依存しない。（DIP, OCP）
type MessageStore interface {
    Add(msg Message)
    List(channel string) []Message
    Channels() []string
    Clear()
}

// Broadcaster はWebSocket経由のメッセージ配信を抽象化するインターフェース。
type Broadcaster interface {
    Broadcast(msg Message)
}
```

- ハンドラー（`handler_slack.go`, `handler_internal.go`）は `MessageStore` / `Broadcaster` インターフェースに依存
- `main.go` で具体実装（`MemoryStore`, `WebSocketHub`）を生成し、ハンドラーに注入（DI）
- 将来の永続化対応時はインターフェースを満たす新実装を追加するだけでハンドラー側の変更は不要

### インメモリ実装

- `MemoryStore`: `[]Message` スライスで保持（mutex保護）
- チャンネル一覧は受信メッセージから動的に生成

## Web UI

### レイアウト

```
+------------------+------------------------------------------+
| # general        |  bot  10:30 AM                           |
| # alerts         |  デプロイが完了しました                    |
| # deploy         |                                          |
|                  |  bot  10:31 AM                           |
|                  |  テストが失敗しました                      |
|                  |                                          |
|   [Clear All]    |                                          |
+------------------+------------------------------------------+
```

- 左サイドバー: チャンネル一覧（受信メッセージから自動生成、件数バッジ付き）
- メインエリア: 選択チャンネルのメッセージ一覧
- Slack Blocks の基本レンダリング（text, section, divider）
- attachments の基本レンダリング（color bar, title, text, fields）
- 配色はSlackダークテーマ風

### リアルタイム更新

- WebSocket接続でサーバーからメッセージをプッシュ
- 新メッセージ着信時にサイドバーのチャンネル一覧とメインエリアを即時更新
- WebSocket切断時は自動再接続

## 使い方

```bash
# インストール
go install github.com/<owner>/slackhog@latest

# 起動（デフォルト: port 4112）
slackhog

# ポート指定
slackhog -port 8080

# アプリ側の設定例
export SLACK_API_URL=http://localhost:4112
```

### アプリ側の変更

Slack SDKのベースURLを `http://localhost:4112` に向けるだけ。

```ruby
# Ruby (slack-ruby-client)
Slack.configure do |config|
  config.token = 'dummy-token'
  config.api_endpoint = 'http://localhost:4112/api/'
end
```

```python
# Python (slack-sdk)
from slack_sdk import WebClient
client = WebClient(token="dummy-token", base_url="http://localhost:4112/api/")
```

```javascript
// Node.js (@slack/web-api)
const { WebClient } = require('@slack/web-api');
const client = new WebClient('dummy-token', { slackApiUrl: 'http://localhost:4112/api/' });
```

## CLIオプション

| フラグ | デフォルト | 説明 |
|--------|----------|------|
| `-port` | `4112` | 待ち受けポート |
| `-max-messages` | `1000` | 保持するメッセージの最大数 |

## 依存関係

```
main.go ──生成・注入──┬── MemoryStore (store.go)
                      └── WebSocketHub (websocket.go)
                             │
server.go ──ルーティング──┬── handler_slack.go ──依存──┬── MessageStore (interface)
                          │                           └── Broadcaster (interface)
                          └── handler_internal.go ──依存──── MessageStore (interface)
```

- 依存の方向は常に「具象 → 抽象」で一方向（ADP準拠）
- 安定コンポーネント（`MessageStore`, `Broadcaster`）は抽象（SAP準拠）
- 不安定コンポーネント（ハンドラー）が安定コンポーネント（インターフェース）に依存（SDP準拠）

## プロジェクト構成

```
slackhog/
├── main.go              # エントリーポイント、CLIフラグ解析、DI
├── server.go            # HTTPサーバー、ルーティング
├── store.go             # MessageStoreインターフェース + MemoryStore実装
├── websocket.go         # Broadcasterインターフェース + WebSocketHub実装
├── handler_slack.go     # Slack API互換エンドポイント（MessageStore, Broadcasterに依存）
├── handler_internal.go  # 内部API（MessageStoreに依存）
├── message.go           # データモデル
├── ui/                  # embed対象の静的ファイル
│   ├── index.html
│   ├── style.css
│   └── app.js
├── go.mod
├── go.sum
└── README.md
```

## スコープ外（将来拡張候補）

- メッセージの永続化（SQLite等）
- Events API / Interactive Components のシミュレーション
- OAuth / ユーザー管理
- ファイルアップロード（`files.upload`）
- Docker イメージ配布
- `conversations.list` 等のチャンネル系API
