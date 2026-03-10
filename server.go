package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

// SlackAPI はSlack互換エンドポイントのインターフェース。
type SlackAPI interface {
	HandleChatPostMessage(w http.ResponseWriter, r *http.Request)
	HandleIncomingWebhook(w http.ResponseWriter, r *http.Request)
}

// InternalAPI は内部APIエンドポイントのインターフェース。
type InternalAPI interface {
	HandleMessages(w http.ResponseWriter, r *http.Request)
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
	mux.HandleFunc("/services/", slack.HandleIncomingWebhook)

	// Internal API
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
