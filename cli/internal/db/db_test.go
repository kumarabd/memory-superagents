package db_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/db"
)

func TestVecString(t *testing.T) {
	result := db.VecString([]float32{1.0, 2.5, -0.5})
	expected := "[1,2.5,-0.5]"
	if result != expected {
		t.Errorf("VecString: got %q, want %q", result, expected)
	}
}

func TestVecStringEmpty(t *testing.T) {
	result := db.VecString([]float32{})
	if result != "[]" {
		t.Errorf("VecString empty: got %q", result)
	}
}
