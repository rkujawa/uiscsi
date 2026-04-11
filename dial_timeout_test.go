package uiscsi

import (
	"testing"
	"time"
)

// TestDialTimeoutOption verifies WithDialTimeout sets the dialTimeout field.
func TestDialTimeoutOption(t *testing.T) {
	cfg := &dialConfig{}
	WithDialTimeout(100 * time.Millisecond)(cfg)

	if cfg.dialTimeout != 100*time.Millisecond {
		t.Errorf("dialTimeout = %v, want 100ms", cfg.dialTimeout)
	}
}

// TestDialTimeoutZero verifies zero means no explicit timeout.
func TestDialTimeoutZero(t *testing.T) {
	cfg := &dialConfig{}
	WithDialTimeout(0)(cfg)

	if cfg.dialTimeout != 0 {
		t.Errorf("dialTimeout = %v, want 0", cfg.dialTimeout)
	}
}
