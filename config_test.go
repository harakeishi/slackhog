package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `port: 8080
max_messages: 500
channels:
  - general
  - random
  - alerts
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.MaxMessages != 500 {
		t.Errorf("MaxMessages = %d, want 500", cfg.MaxMessages)
	}
	if len(cfg.Channels) != 3 {
		t.Errorf("Channels length = %d, want 3", len(cfg.Channels))
	}
	if cfg.Channels[0] != "general" {
		t.Errorf("Channels[0] = %q, want %q", cfg.Channels[0], "general")
	}
}

func TestLoadConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"port": 9090, "max_messages": 200, "channels": ["dev", "ops"]}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if len(cfg.Channels) != 2 {
		t.Errorf("Channels length = %d, want 2", len(cfg.Channels))
	}
}

func TestLoadConfig_PartialYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `channels:
  - general
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// デフォルト値が使われる
	if cfg.Port != 4112 {
		t.Errorf("Port = %d, want default 4112", cfg.Port)
	}
	if cfg.MaxMessages != 1000 {
		t.Errorf("MaxMessages = %d, want default 1000", cfg.MaxMessages)
	}
	if len(cfg.Channels) != 1 || cfg.Channels[0] != "general" {
		t.Errorf("Channels = %v, want [general]", cfg.Channels)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestStoreInitialChannels(t *testing.T) {
	store := NewMemoryStore(100)
	store.SetInitialChannels([]string{"general", "random"})

	channels := store.Channels()
	if len(channels) != 2 {
		t.Fatalf("Channels length = %d, want 2", len(channels))
	}
	if channels[0] != "general" || channels[1] != "random" {
		t.Errorf("Channels = %v, want [general random]", channels)
	}
}

func TestStoreInitialChannels_MergedWithMessages(t *testing.T) {
	store := NewMemoryStore(100)
	store.SetInitialChannels([]string{"general", "random"})

	msg := &Message{ID: "1", Channel: "alerts", Text: "test"}
	store.Add(msg)

	// generalもメッセージ追加
	msg2 := &Message{ID: "2", Channel: "general", Text: "hello"}
	store.Add(msg2)

	channels := store.Channels()
	if len(channels) != 3 {
		t.Fatalf("Channels length = %d, want 3", len(channels))
	}
	// 初期チャンネルが先、メッセージ由来が後
	if channels[0] != "general" {
		t.Errorf("Channels[0] = %q, want general", channels[0])
	}
	if channels[1] != "random" {
		t.Errorf("Channels[1] = %q, want random", channels[1])
	}
	if channels[2] != "alerts" {
		t.Errorf("Channels[2] = %q, want alerts", channels[2])
	}
}
