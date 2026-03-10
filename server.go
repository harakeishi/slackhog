package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

type Server struct {
	mux *http.ServeMux
}

func NewServer(slackHandler *SlackHandler, internalHandler *InternalHandler, wsHub *WebSocketHub) *Server {
	mux := http.NewServeMux()

	// Slack API compatible
	mux.HandleFunc("/api/chat.postMessage", slackHandler.HandleChatPostMessage)
	mux.HandleFunc("/services/", slackHandler.HandleIncomingWebhook)

	// Internal API
	mux.HandleFunc("/_api/messages", internalHandler.HandleMessages)

	// WebSocket
	mux.HandleFunc("/ws", wsHub.HandleWS)

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
