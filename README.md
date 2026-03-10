# SlackHog

A [MailHog](https://github.com/mailhog/MailHog)-like tool for Slack. SlackHog catches Slack API requests locally and displays them in a Slack-like Web UI — useful for developing and testing Slack integrations without sending real messages.

## Screenshots

### Slack-like Web UI with channels and emoji avatars

![All Channels](docs/screenshots/all-channels.png)

### Thread support with side panel

![Thread](docs/screenshots/thread.png)

### Rich messages (attachments, blocks, fields)

![Rich Messages](docs/screenshots/rich-messages.png)

### Dark / Light theme

![Dark Theme](docs/screenshots/dark-theme.png)

## Features

- **Slack API compatible** — supports `chat.postMessage` and Incoming Webhooks
- **Real-time Web UI** — Slack-like interface with channels, threads, and emoji avatars
- **WebSocket push** — messages appear instantly without polling
- **Thread support** — view threaded conversations in a side panel
- **Rich messages** — renders attachments (color bars, fields) and blocks (sections, links)
- **Dark / Light theme** — toggle between themes
- **In-memory store** — no database required, configurable message retention
- **Single binary** — UI is embedded via `go:embed`

## Quick Start

### Docker (recommended)

```bash
docker run -p 4112:4112 ghcr.io/harakeishi/slackhog
```

### Go install

```bash
go install github.com/harakeishi/slackhog@latest
slackhog
```

### Build from source

```bash
git clone https://github.com/harakeishi/slackhog.git
cd slackhog
go build -o slackhog .
./slackhog
```

Open http://localhost:4112 to view the Web UI.

## Usage

```
slackhog [flags]

Flags:
  -port int          listen port (default 4112)
  -max-messages int  maximum number of messages to keep (default 1000)
```

## API Endpoints

### Slack-compatible

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/chat.postMessage` | Slack `chat.postMessage` compatible endpoint |
| POST | `/services/{webhook_id}` | Incoming Webhook compatible endpoint |

### Internal

| Method | Path | Description |
|--------|------|-------------|
| GET | `/_api/messages` | List messages (optional `?channel=` filter) |
| DELETE | `/_api/messages` | Clear all messages |
| GET | `/_api/messages/{id}/replies` | Get thread replies |
| GET | `/ws` | WebSocket for real-time updates |

## Examples

Send a message via `chat.postMessage`:

```bash
curl -X POST http://localhost:4112/api/chat.postMessage \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "#general",
    "text": "Hello from SlackHog!",
    "username": "test-bot",
    "icon_emoji": ":robot_face:"
  }'
```

Send a message via Incoming Webhook:

```bash
curl -X POST http://localhost:4112/services/T00000000/B00000000/XXXXXXXX \
  -H "Content-Type: application/json" \
  -d '{"text": "Webhook message!", "channel": "#alerts"}'
```

Send a threaded reply:

```bash
# First, send a parent message and note the returned message ID
# Then send a reply with thread_ts
curl -X POST http://localhost:4112/api/chat.postMessage \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "#general",
    "text": "This is a reply",
    "thread_ts": "<parent-message-id>"
  }'
```

## Architecture

SlackHog follows SOLID principles with clear interface boundaries:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Slack API   │     │ Internal API │     │  WebSocket   │
│  Handlers    │     │  Handlers    │     │     Hub      │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       ▼                    ▼                    ▼
   SlackAPI            InternalAPI           WSHandler
  interface            interface             interface
       │                    │                    │
       └────────┬───────────┘                    │
                ▼                                │
          MessageStore ◄─────────────────────────┘
           interface
                │
                ▼
          MemoryStore
```

## License

See [LICENSE](LICENSE).
