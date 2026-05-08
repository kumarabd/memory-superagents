package ai_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/ai"
)

func TestNewClientPanicsWithEmptyKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with empty API key")
		}
	}()
	ai.NewClient("")
}
