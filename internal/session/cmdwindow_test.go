package session

import (
	"context"
	"sync"
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

// TestCmdWindowContextCancelIsolated proves that one caller's context timeout
// does not poison the window for subsequent callers (SESS-01).
// A per-waiter done channel must be used; w.closed must NOT be set on ctx.Done().
func TestCmdWindowContextCancelIsolated(t *testing.T) {
	// Zero-slot window: MaxCmdSN=0 < ExpCmdSN=1 in serial arithmetic.
	w := newCmdWindow(1, 1, 0)

	ctx1, cancel1 := context.WithCancel(context.Background())
	errCh1 := make(chan error, 1)
	go func() {
		_, err := w.acquire(ctx1)
		errCh1 <- err
	}()

	// Let goroutine 1 enter the slow path.
	time.Sleep(20 * time.Millisecond)

	// Start goroutine 2 before cancelling goroutine 1.
	errCh2 := make(chan uint32, 1)
	go func() {
		sn, err := w.acquire(context.Background())
		if err != nil {
			t.Errorf("goroutine 2 acquire: unexpected error: %v", err)
			return
		}
		errCh2 <- sn
	}()

	// Let goroutine 2 enter the slow path too.
	time.Sleep(20 * time.Millisecond)

	// Cancel goroutine 1's context.
	cancel1()

	select {
	case err := <-errCh1:
		if err != context.Canceled {
			t.Fatalf("goroutine 1: got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine 1 did not return after context cancel")
	}

	// Open a slot — goroutine 2 must succeed.
	w.update(1, 2)

	select {
	case sn := <-errCh2:
		if sn != 1 {
			t.Fatalf("goroutine 2: got CmdSN %d, want 1", sn)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine 2 did not succeed after window opened (window was poisoned by goroutine 1's cancel)")
	}
}

// TestCmdWindowContextCancelDoesNotSetClosed verifies that a context cancel in
// acquire() does not set w.closed, and a subsequent acquire() with a fresh
// context succeeds normally.
func TestCmdWindowContextCancelDoesNotSetClosed(t *testing.T) {
	// Zero-slot window.
	w := newCmdWindow(1, 1, 0)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := w.acquire(ctx)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acquire did not return after context cancel")
	}

	// w.closed must still be false.
	w.mu.Lock()
	closed := w.closed
	w.mu.Unlock()
	if closed {
		t.Fatal("w.closed was set to true by context cancel — window is permanently poisoned")
	}

	// A subsequent acquire with a fresh context and an open window must succeed.
	w.update(1, 2)
	sn, err := w.acquire(context.Background())
	if err != nil {
		t.Fatalf("subsequent acquire: unexpected error: %v", err)
	}
	if sn != 1 {
		t.Fatalf("subsequent acquire: got CmdSN %d, want 1", sn)
	}
}

// TestCmdWindowCloseStillWorks verifies that the per-waiter channel pattern
// does not break session-level window.close() — all waiters must still be
// woken with errWindowClosed.
func TestCmdWindowCloseStillWorks(t *testing.T) {
	// Zero-slot window.
	w := newCmdWindow(1, 1, 0)

	const numWaiters = 5
	errChs := make([]chan error, numWaiters)
	for i := range errChs {
		errChs[i] = make(chan error, 1)
		go func(ch chan error) {
			_, err := w.acquire(context.Background())
			ch <- err
		}(errChs[i])
	}

	time.Sleep(50 * time.Millisecond)
	w.close()

	for i, ch := range errChs {
		select {
		case err := <-ch:
			if err != errWindowClosed {
				t.Errorf("waiter %d: got %v, want errWindowClosed", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("waiter %d did not return after close", i)
		}
	}
}

// TestCmdWindowAcquireConcurrent exercises 10 concurrent callers against a
// window of size 5, verifying all eventually succeed and no goroutines leak.
func TestCmdWindowAcquireConcurrent(t *testing.T) {
	const total = 10
	// Window [1, 1, 5]: allows CmdSN 1 through 5.
	w := newCmdWindow(1, 1, 5)

	var wg sync.WaitGroup
	results := make(chan uint32, total)
	errs := make(chan error, total)

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sn, err := w.acquire(context.Background())
			if err != nil {
				errs <- err
				return
			}
			results <- sn
		}()
	}

	// First 5 acquire immediately; advance window for the remaining 5.
	time.Sleep(30 * time.Millisecond)
	w.update(1, 10) // expand MaxCmdSN to 10

	// Wait for all goroutines to finish.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent acquire did not complete in time")
	}

	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("unexpected error from concurrent acquire: %v", err)
	}

	collected := make([]uint32, 0, total)
	for sn := range results {
		collected = append(collected, sn)
	}
	if len(collected) != total {
		t.Fatalf("got %d results, want %d", len(collected), total)
	}
}
