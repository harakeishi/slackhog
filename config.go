package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config はアプリケーション設定を表す。
type Config struct {
	Port        int      `json:"port" yaml:"port"`
	MaxMessages int      `json:"max_messages" yaml:"max_messages"`
	Channels    []string `json:"channels" yaml:"channels"`
}

// DefaultConfig はデフォルト設定を返す。
func DefaultConfig() Config {
	return Config{
		Port:        4112,
		MaxMessages: 1000,
	}
}

// LoadConfig は指定パスから設定ファイルを読み込む。
// 拡張子に応じて YAML または JSON としてパースする。
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		// デフォルトはYAMLとして試行
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse config (unsupported extension %q): %w", ext, err)
		}
	}

	return cfg, nil
}
