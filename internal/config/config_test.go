package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOverridesAndFallbacks proves that values present in config.toml
// override the defaults, while keys omitted from the file keep their defaults.
func TestLoadOverridesAndFallbacks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// Only a few keys are set; everything else must fall back to Default().
	content := `
[server]
listen = "127.0.0.1:9999"

[anki]
model = "Custom Model"

[claude]
lemmatize = false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Overridden by the file.
	if cfg.Server.Listen != "127.0.0.1:9999" {
		t.Errorf("Server.Listen = %q, want the file value", cfg.Server.Listen)
	}
	if cfg.Anki.Model != "Custom Model" {
		t.Errorf("Anki.Model = %q, want the file value", cfg.Anki.Model)
	}
	if cfg.Claude.Lemmatize {
		t.Errorf("Claude.Lemmatize = true, want false from the file")
	}

	// Not in the file -> must keep defaults.
	def := Default()
	if cfg.Anki.URL != def.Anki.URL {
		t.Errorf("Anki.URL = %q, want default %q", cfg.Anki.URL, def.Anki.URL)
	}
	if cfg.Claude.Provider != def.Claude.Provider {
		t.Errorf("Claude.Provider = %q, want default %q", cfg.Claude.Provider, def.Claude.Provider)
	}
	if cfg.Pending.RetryInterval != def.Pending.RetryInterval {
		t.Errorf("Pending.RetryInterval = %d, want default %d", cfg.Pending.RetryInterval, def.Pending.RetryInterval)
	}
}
