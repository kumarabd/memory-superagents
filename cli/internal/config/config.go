package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	OpenAIKey   string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is not set — add it to your shell profile")
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is not set — add it to your shell profile")
	}
	return &Config{DatabaseURL: dbURL, OpenAIKey: apiKey}, nil
}
