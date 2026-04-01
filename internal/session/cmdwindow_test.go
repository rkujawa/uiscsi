package session

import (
	"context"
	"testing"
	"time"
)

func TestCmdWindowAcquire(t *testing.T) {
	// Window [1, 1, 3] allows CmdSN 1, 2, 3.
	w := newCmdWindow(1, 1, 3)

	for want := uint32(1); want <= 3; want++ {
		got, err := w.acquire(context.Background())
		if err != nil {
			t.Fatalf("acquire(%d): unexpected error: %v", want, err)
		}
		if got != want {
			t.Fatalf("acquire: got CmdSN %d, want %d", got, want)
		}
	}
}

func TestCmdWindowBlocks(t *testing.T) {
	// Window of size 1: only CmdSN 1 is allowed initially.
	w := newCmdWindow(1, 1, 1)

	// Consume the only slot.
	sn, err := w.acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if sn != 1 {
		t.Fatalf("first acquire: got %d, want 1", sn)
	}

	// Second acquire should block because window is full (cmdSN=2, max=1).
	acquired := make(chan uint32, 1)
	go func() {
		sn2, err2 := w.acquire(context.Background())
		if err2 != nil {
			t.Errorf("second acquire: %v", err2)
			return
		}
		acquired <- sn2
	}()

	// Verify it doesn't complete immediately.
	select {
	case <-acquired:
		t.Fatal("second acquire should have blocked")
	case <-time.After(50 * time.Millisecond):
		// Good, it's blocking.
	}

	// Advance window: now MaxCmdSN=2.
	w.update(1, 2)

	select {
	case sn2 := <-acquired:
		if sn2 != 2 {
			t.Fatalf("second acquire: got %d, want 2", sn2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire did not unblock after window update")
	}
}

func TestCmdWindowContextCancel(t *testing.T) {
	// Window of size 1, consume the slot.
	w := newCmdWindow(1, 1, 1)
	if _, err := w.acquire(context.Background()); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := w.acquire(ctx)
		errCh <- err
	}()

	// Let the goroutine start waiting.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("acquire after cancel: got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acquire did not return after context cancel")
	}
}

func TestCmdWindowClose(t *testing.T) {
	w := newCmdWindow(1, 1, 1)
	// Consume slot.
	if _, err := w.acquire(context.Background()); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := w.acquire(context.Background())
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	w.close()

	select {
	case err := <-errCh:
		if err != errWindowClosed {
			t.Fatalf("acquire after close: got %v, want errWindowClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acquire did not return after close")
	}
}

func TestCmdWindowUpdateIgnoresStale(t *testing.T) {
	// Window [5, 5, 10].
	w := newCmdWindow(5, 5, 10)

	// Stale update: MaxCmdSN=3 < ExpCmdSN-1=4 (serial). Should be ignored.
	w.update(5, 3)

	w.mu.Lock()
	if w.maxCmdSN != 10 {
		t.Fatalf("maxCmdSN changed to %d after stale update, want 10", w.maxCmdSN)
	}
	w.mu.Unlock()
}

func TestCmdWindowWrapAround(t *testing.T) {
	// CmdSN near wrap boundary.
	start := uint32(0xFFFFFFFD)
	w := newCmdWindow(start, start, 1) // Window wraps: [0xFFFFFFFD .. 1]

	expected := []uint32{0xFFFFFFFD, 0xFFFFFFFE, 0xFFFFFFFF, 0, 1}
	for _, want := range expected {
		got, err := w.acquire(context.Background())
		if err != nil {
			t.Fatalf("acquire(want=%d): %v", want, err)
		}
		if got != want {
			t.Fatalf("acquire: got 0x%08X, want 0x%08X", got, want)
		}
	}
}

func TestCmdWindowCurrent(t *testing.T) {
	w := newCmdWindow(42, 42, 50)

	cur := w.current()
	if cur != 42 {
		t.Fatalf("current: got %d, want 42", cur)
	}

	// Acquire should not affect current until it returns.
	sn, _ := w.acquire(context.Background())
	if sn != 42 {
		t.Fatalf("acquire: got %d, want 42", sn)
	}
	// After acquire, current should be 43.
	cur = w.current()
	if cur != 43 {
		t.Fatalf("current after acquire: got %d, want 43", cur)
	}
}
