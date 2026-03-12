package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed ui
var uiFS embed.FS

// SlackAPI はSlack互換エンドポイントのインターフェース。
type SlackAPI interface {
	HandleChatPostMessage(w http.ResponseWriter, r *http.Request)
	HandleChatUpdate(w http.ResponseWriter, r *http.Request)
	HandleConversationsInfo(w http.ResponseWriter, r *http.Request)
	HandleIncomingWebhook(w http.ResponseWriter, r *http.Request)
}

// InternalAPI は内部APIエンドポイントのインターフェース。
type InternalAPI interface {
	HandleMessages(w http.ResponseWriter, r *http.Request)
	HandleReplies(w http.ResponseWriter, r *http.Request, parentID string)
}

// WSHandler はWebSocket接続ハンドラーのインターフェース。
type WSHandler interface {
	HandleWS(w http.ResponseWriter, r *http.Request)
}

// Server はSlackHogのHTTPサーバー。
type Server struct {
	mux *http.ServeMux
}

// NewServer は新しいServerを生成し、ルーティングを設定する。
func NewServer(slack SlackAPI, internal InternalAPI, ws WSHandler) *Server {
	mux := http.NewServeMux()

	// Slack API compatible
	mux.HandleFunc("/api/chat.postMessage", slack.HandleChatPostMessage)
	mux.HandleFunc("/api/chat.update", slack.HandleChatUpdate)
	mux.HandleFunc("/api/conversations.info", slack.HandleConversationsInfo)
	mux.HandleFunc("/services/", slack.HandleIncomingWebhook)

	// Internal API
	mux.HandleFunc("/_api/messages/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/_api/messages/"), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] == "replies" {
			internal.HandleReplies(w, r, parts[0])
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/_api/messages", internal.HandleMessages)

	// WebSocket
	mux.HandleFunc("/ws", ws.HandleWS)

	// Web UI static files
	uiContent, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(uiContent)))

	return &Server{mux: mux}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
