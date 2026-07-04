// Package config loads vocab2anki's TOML configuration and the Anthropic API
// key from the environment.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the fully-resolved runtime configuration. Filesystem paths have
// already had a leading "~" expanded to the user's home directory.
type Config struct {
	Server  ServerConfig  `toml:"server"`
	Anki    AnkiConfig    `toml:"anki"`
	Claude  ClaudeConfig  `toml:"claude"`
	Queue   QueueConfig   `toml:"queue"`
	Pending PendingConfig `toml:"pending"`
	Audio   AudioConfig   `toml:"audio"`

	// AnthropicAPIKey comes from the ANTHROPIC_API_KEY environment variable,
	// never from the config file.
	AnthropicAPIKey string `toml:"-"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
}

type AnkiConfig struct {
	URL   string `toml:"url"`
	Deck  string `toml:"deck"`
	Model string `toml:"model"`
}

type ClaudeConfig struct {
	// Provider selects the backend: "cli" (local `claude` command, no API key)
	// or "api" (Anthropic HTTP API, needs ANTHROPIC_API_KEY).
	Provider string `toml:"provider"`
	// Model name/alias. For "cli", aliases like "haiku" work; for "api", use a
	// full model id.
	Model string `toml:"model"`
	// MaxTokens caps generation (API only).
	MaxTokens int `toml:"max_tokens"`
	// CLIPath is the `claude` executable to run (CLI only); empty means "claude".
	CLIPath string `toml:"cli_path"`
}

type QueueConfig struct {
	File string `toml:"file"`
}

type PendingConfig struct {
	File          string `toml:"file"`
	RetryInterval int    `toml:"retry_interval"`
}

type AudioConfig struct {
	Dir string `toml:"dir"`
}

// Default returns a Config with sensible fallbacks so that a missing or partial
// config file still yields a working setup.
func Default() *Config {
	return &Config{
		Server:  ServerConfig{Listen: "127.0.0.1:8766"},
		Anki:    AnkiConfig{URL: "http://127.0.0.1:8765", Deck: "Vocabulary::English", Model: "Vocab2Anki"},
		Claude:  ClaudeConfig{Provider: "cli", Model: "haiku", MaxTokens: 1024, CLIPath: "claude"},
		Queue:   QueueConfig{File: "~/Library/Mobile Documents/com~apple~CloudDocs/vocab2anki/vocab-queue.txt"},
		Pending: PendingConfig{File: "~/Library/Application Support/vocab2anki/pending.json", RetryInterval: 60},
		Audio:   AudioConfig{Dir: "~/Library/Application Support/vocab2anki/audio"},
	}
}

// Load reads the TOML file at path over the defaults, expands ~ paths, and
// pulls the API key from the environment.
func Load(path string) (*Config, error) {
	cfg := Default()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	cfg.Queue.File = expand(cfg.Queue.File, home)
	cfg.Pending.File = expand(cfg.Pending.File, home)
	cfg.Audio.Dir = expand(cfg.Audio.Dir, home)

	cfg.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	return cfg, nil
}

// expand replaces a leading "~" with home.
func expand(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
