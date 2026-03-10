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
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}
