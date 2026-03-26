package id

import (
	"regexp"
	"testing"
	"time"
)

func TestRandomHex(t *testing.T) {
	hex := RandomHex(6)
	if len(hex) != 6 {
		t.Errorf("expected length 6, got %d", len(hex))
	}

	// Should be valid hex
	matched, _ := regexp.MatchString("^[a-f0-9]+$", hex)
	if !matched {
		t.Errorf("expected hex string, got %s", hex)
	}

	// Two calls should produce different values
	hex2 := RandomHex(6)
	if hex == hex2 {
		t.Error("RandomHex should produce different values")
	}
}

func TestNewSessionID(t *testing.T) {
	id := NewSessionID()

	// Should be in format YYYY-MM-DD-XXXXXX
	if len(id) < 17 {
		t.Errorf("ID too short: %s", id)
	}

	// Check date prefix
	datePrefix := id[:10]
	_, err := time.Parse("2006-01-02", datePrefix)
	if err != nil {
		t.Errorf("ID date prefix invalid: %s", datePrefix)
	}

	// Check separator
	if id[10] != '-' {
		t.Errorf("expected dash separator at position 10, got %c", id[10])
	}
}
