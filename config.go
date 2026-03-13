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

	// バリデーション
	if err := cfg.validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// validate は設定値を検証する。
func (c *Config) validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 0-65535)", c.Port)
	}
	if c.MaxMessages < 0 {
		return fmt.Errorf("invalid max_messages: %d (must be >= 0)", c.MaxMessages)
	}

	// 空文字列のチャンネルを除外
	filtered := make([]string, 0, len(c.Channels))
	for _, ch := range c.Channels {
		if ch != "" {
			filtered = append(filtered, ch)
		}
	}
	c.Channels = filtered

	return nil
}
