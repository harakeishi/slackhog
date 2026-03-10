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
