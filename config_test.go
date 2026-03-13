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

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("port: [invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("port: 99999"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestLoadConfig_NegativeMaxMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("max_messages: -1"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for negative max_messages")
	}
}

func TestLoadConfig_EmptyChannelsFiltered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `channels:
  - general
  - ""
  - random
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.Channels) != 2 {
		t.Errorf("Channels length = %d, want 2 (empty string filtered)", len(cfg.Channels))
	}
	if cfg.Channels[0] != "general" || cfg.Channels[1] != "random" {
		t.Errorf("Channels = %v, want [general random]", cfg.Channels)
	}
}
