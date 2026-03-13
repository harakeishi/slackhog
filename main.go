package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "", "path to config file (YAML or JSON)")
	port := flag.Int("port", 0, "listen port (overrides config)")
	maxMessages := flag.Int("max-messages", 0, "maximum number of messages to keep (overrides config)")
	flag.Parse()

	if *showVersion {
		fmt.Println("slackhog", version)
		return
	}

	cfg := DefaultConfig()
	if *configPath != "" {
		var err error
		cfg, err = LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// CLIフラグが明示的に指定された場合はconfigを上書き
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "port":
			cfg.Port = *port
		case "max-messages":
			cfg.MaxMessages = *maxMessages
		}
	})

	store := NewMemoryStore(cfg.MaxMessages)
	if len(cfg.Channels) > 0 {
		store.SetInitialChannels(cfg.Channels)
	}

	hub := NewWebSocketHub()
	slackHandler := NewSlackHandler(store, hub)
	internalHandler := NewInternalHandler(store)
	server := NewServer(slackHandler, internalHandler, hub)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("SlackHog listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
