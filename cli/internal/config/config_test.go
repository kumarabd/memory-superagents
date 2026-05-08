package config_test

import (
	"os"
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
)

func TestLoadMissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("OPENAI_API_KEY")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadMissingOpenAIKey(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x")
	os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("DATABASE_URL")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is missing")
	}
}

func TestLoadSuccess(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x")
	os.Setenv("OPENAI_API_KEY", "sk-test")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("OPENAI_API_KEY")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.OpenAIKey != "sk-test" {
		t.Errorf("unexpected OpenAIKey: %s", cfg.OpenAIKey)
	}
}
