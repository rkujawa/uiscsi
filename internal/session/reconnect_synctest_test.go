package session

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/uiscsi/uiscsi/internal/login"
	"github.com/uiscsi/uiscsi/internal/transport"
)

// mockDialer records call count and delegates to a configurable dialFn.
type mockDialer struct {
	mu     sync.Mutex
	calls  int
	dialFn func(ctx context.Context, addr string) (*transport.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, addr string) (*transport.Conn, error) {
	m.mu.Lock()
	m.calls++
	fn := m.dialFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, addr)
	}
	return nil, errors.New("mockDialer: no dialFn set")
}

func (m *mockDialer) dialCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// newSynctestSession creates a session backed by a net.Pipe for use inside
// a synctest bubble. Both pipe ends are created inside the bubble so fake
// timers apply.
func newSynctestSession(t *testing.T, opts ...SessionOption) (*Session, net.Conn) {
	t.Helper()
	clientConn, targetConn := net.Pipe()
	tc := transport.NewConnFromNetConn(clientConn)
	params := login.Defaults()
	params.CmdSN = 1
	params.ExpStatSN = 1
	sess := NewSession(tc, params, opts...)
	return sess, targetConn
}

// TestReconnectSynctest tests the reconnect state machine using fake time.
// All goroutines and connections are created inside the synctest bubble so
// time.NewTimer in the reconnect backoff loop fires instantly via synctest.Wait().
func TestReconnectSynctest(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		// All reconnect attempts fail immediately. synctest.Wait() advances
		// fake time past all backoff delays without real sleeps.
		synctest.Test(t, func(t *testing.T) {
			md := &mockDialer{
				dialFn: func(ctx context.Context, addr string) (*transport.Conn, error) {
					return nil, errors.New("connection refused")
				},
			}

			sess, targetConn := newSynctestSession(t,
				WithDialer(md.Dial),
				WithReconnectInfo("127.0.0.1:9999"),
				WithMaxReconnectAttempts(3),
				WithReconnectBackoff(1*time.Second),
				WithKeepaliveInterval(999*time.Hour),
			)

			// Submit a command so we have an in-flight task.
			cmd := Command{Read: true, ExpectedDataTransferLen: 4}
			cmd.CDB[0] = 0x28
			resultCh, err := sess.Submit(context.Background(), cmd)
			if err != nil {
				t.Fatalf("Submit: %v", err)
			}

			// Close the target side of the pipe to trigger reconnect.
			targetConn.Close()

			// synctest.Wait() parks the main goroutine and fires fake timers.
			// The reconnect loop's time.NewTimer(delay) fires instantly here,
			// so all 3 attempts execute without real sleeps.
			synctest.Wait()

			// After all 3 attempts, the reconnect goroutine fails all tasks and exits.
			// Read the result — must be a reconnect failure error.
			var result Result
			select {
			case result = <-resultCh:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for reconnect failure result")
			}

			if result.Err == nil {
				t.Fatal("expected reconnect failure error, got nil")
			}
			if !containsStr(result.Err.Error(), "reconnect failed") {
				t.Fatalf("expected 'reconnect failed' in error, got: %v", result.Err)
			}

			// Close the session and wait for all goroutines in the bubble to exit.
			sess.Close()
			synctest.Wait()
		})
	})

	t.Run("cancel", func(t *testing.T) {
		// Close() during active reconnect must not leak the reconnect goroutine.
		// The mock dialer blocks on an explicit release channel (not ctx.Done,
		// which is derived from context.Background() and may not integrate cleanly
		// with the synctest fake clock). Close() is called in a goroutine; then
		// synctest.Wait() fires fake timers to drain the session's keepalive and
		// reconnect-backoff timers. We release the dialer and verify that the
		// bubble closes cleanly (no durable-block goroutines remain).
		synctest.Test(t, func(t *testing.T) {
			// release is closed by the test to unblock the mock dialer.
			release := make(chan struct{})

			md := &mockDialer{
				dialFn: func(ctx context.Context, addr string) (*transport.Conn, error) {
					// Block until the test releases us or ctx is cancelled.
					select {
					case <-release:
					case <-ctx.Done():
					}
					return nil, errors.New("dial cancelled")
				},
			}

			sess, targetConn := newSynctestSession(t,
				WithDialer(md.Dial),
				WithReconnectInfo("127.0.0.1:9999"),
				WithMaxReconnectAttempts(3),
				WithReconnectBackoff(1*time.Second),
				WithKeepaliveInterval(999*time.Hour),
			)

			// Trigger reconnect by closing the target side of the pipe.
			targetConn.Close()

			// Wait for the reconnect goroutine to park in the dialer's select.
			synctest.Wait()

			// Verify at least one dial was attempted.
			if md.dialCalls() == 0 {
				t.Fatal("expected at least one dial attempt before Wait")
			}

			// Release the dialer so the reconnect loop can continue to the
			// next attempt (or exhaust attempts and fail tasks).
			close(release)

			// Call Close in a goroutine; Close() will return once the pump WG
			// is satisfied (old pumps are already done after reconnect started).
			closeDone := make(chan struct{})
			go func() {
				defer close(closeDone)
				sess.Close()
			}()

			// synctest.Wait() advances fake time to fire all remaining backoff
			// timers in the reconnect loop, allowing it to exhaust attempts and exit.
			synctest.Wait()

			select {
			case <-closeDone:
				// Close returned cleanly — no goroutine leak.
			case <-time.After(5 * time.Second):
				t.Fatal("sess.Close() did not return after reconnect cancel")
			}
		})
	})

	t.Run("concurrent_reconnect", func(t *testing.T) {
		// Two Submit goroutines hit a connection drop simultaneously.
		// Only one reconnect should trigger (the recovering flag prevents a second).
		synctest.Test(t, func(t *testing.T) {
			var dialCalls int

			md := &mockDialer{
				dialFn: func(ctx context.Context, addr string) (*transport.Conn, error) {
					dialCalls++
					// Always fail — we're testing the reconnect guard, not success.
					return nil, errors.New("connection refused")
				},
			}

			sess, targetConn := newSynctestSession(t,
				WithDialer(md.Dial),
				WithReconnectInfo("127.0.0.1:9999"),
				WithMaxReconnectAttempts(1),
				WithReconnectBackoff(100*time.Millisecond),
				WithKeepaliveInterval(999*time.Hour),
			)

			// Pre-open window to allow 2 concurrent submits.
			sess.window.update(1, 10)

			// Submit two commands concurrently.
			resultCh1 := make(chan (<-chan Result), 1)
			resultCh2 := make(chan (<-chan Result), 1)

			go func() {
				cmd := Command{Read: true, ExpectedDataTransferLen: 4}
				cmd.CDB[0] = 0x28
				ch, err := sess.Submit(context.Background(), cmd)
				if err != nil {
					resultCh1 <- nil
					return
				}
				resultCh1 <- ch
			}()
			go func() {
				cmd := Command{Read: true, ExpectedDataTransferLen: 4}
				cmd.CDB[0] = 0x28
				ch, err := sess.Submit(context.Background(), cmd)
				if err != nil {
					resultCh2 <- nil
					return
				}
				resultCh2 <- ch
			}()

			// Let both submits complete.
			synctest.Wait()

			// Drop connection to trigger reconnect.
			targetConn.Close()

			// Wait for reconnect to run (all backoffs fire via fake time).
			synctest.Wait()

			// At most one reconnect attempt should have been made.
			// Each attempt tries exactly once (maxReconnectAttempts=1),
			// but the recovering flag should prevent a second reconnect from starting.
			if dialCalls > 1 {
				t.Errorf("expected at most 1 dial call (recovering guard), got %d", dialCalls)
			}

			// Both tasks should eventually resolve (with error, since reconnect failed).
			// Drain channels to prevent goroutine leaks.
			ch1 := <-resultCh1
			ch2 := <-resultCh2
			if ch1 != nil {
				select {
				case <-ch1:
				case <-time.After(5 * time.Second):
				}
			}
			if ch2 != nil {
				select {
				case <-ch2:
				case <-time.After(5 * time.Second):
				}
			}

			sess.Close()
			synctest.Wait()
		})
	})
}

// dropAfterSCSICommandTarget accepts login and one SCSICommand PDU, then
// drops the connection before sending any response (simulating connection loss
// mid-burst before R2T arrives). Subsequent connections are handled normally.
type dropAfterSCSICommandTarget struct {
	ln     net.Listener
	t      *testing.T
	stopCh chan struct{}
}

func startDropAfterSCSICommandTarget(t *testing.T) *dropAfterSCSICommandTarget {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	dt := &dropAfterSCSICommandTarget{
		ln:     ln,
		t:      t,
		stopCh: make(chan struct{}),
	}
	go dt.acceptLoop()
	t.Cleanup(func() {
		close(dt.stopCh)
		ln.Close()
	})
	return dt
}

func (dt *dropAfterSCSICommandTarget) addr() string {
	return dt.ln.Addr().String()
}

func (dt *dropAfterSCSICommandTarget) acceptLoop() {
	first := true
	for {
		conn, err := dt.ln.Accept()
		if err != nil {
			select {
			case <-dt.stopCh:
				return
			default:
				return
			}
		}
		if first {
			first = false
			go dt.handleFirstConn(conn)
		} else {
			// Subsequent connections: respond normally (full target behaviour).
			rt := &recoverableTarget{t: dt.t, readData: nil}
			go rt.handleConn(conn)
		}
	}
}

func (dt *dropAfterSCSICommandTarget) handleFirstConn(conn net.Conn) {
	defer conn.Close()
	// Complete login.
	rt := &recoverableTarget{t: dt.t, readData: nil}
	if err := rt.handleLogin(conn); err != nil {
		dt.t.Logf("dropAfterSCSICommandTarget: login error: %v", err)
		return
	}
	// Read one SCSICommand PDU, then drop connection without responding.
	// This simulates losing the connection after the initiator sends its command
	// but before any R2T / SCSIResponse arrives.
	_, _ = transport.ReadRawPDU(conn, false, false, 0)
	// conn.Close() via defer.
}

// TestPartialWriteRecovery tests write command recovery edge cases across
// reconnect boundaries. Does not use synctest — exercises real timer paths
// with very short backoffs to stay fast.
func TestPartialWriteRecovery(t *testing.T) {
	t.Run("seekable_mid_burst", func(t *testing.T) {
		// Write command with seekable reader. Connection drops after the
		// SCSICommand PDU is sent but before the target sends an R2T.
		// Reconnect succeeds. reader.Seek(0, io.SeekStart) is called.
		// Retried command succeeds.
		target := startDropAfterSCSICommandTarget(t)

		tc, params := connectAndLogin(t, target.addr())

		writeData := make([]byte, 128)
		for i := range writeData {
			writeData[i] = byte(i)
		}

		sess := NewSession(tc, *params,
			WithReconnectInfo(target.addr()),
			WithReconnectBackoff(10*time.Millisecond),
			WithMaxReconnectAttempts(3),
			WithKeepaliveInterval(60*time.Second),
		)
		t.Cleanup(func() { sess.Close() })

		cmd := Command{
			Write:                   true,
			ExpectedDataTransferLen: uint32(len(writeData)),
			Data:                    newSeekableReader(writeData),
		}
		cmd.CDB[0] = 0x2A // WRITE(10)

		resultCh, err := sess.Submit(context.Background(), cmd)
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		select {
		case result := <-resultCh:
			if result.Err != nil {
				t.Fatalf("expected success after seekable retry, got: %v", result.Err)
			}
			if result.Status != 0x00 {
				t.Fatalf("status: got 0x%02X, want 0x00", result.Status)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for seekable write result after reconnect")
		}
	})

	t.Run("nonseeakble_fails", func(t *testing.T) {
		// Write command with non-seekable reader. Connection drops. Task must
		// fail with ErrRetryNotPossible — the reader cannot be seeked back.
		target := startDropAfterSCSICommandTarget(t)

		tc, params := connectAndLogin(t, target.addr())

		writeData := []byte("non-seekable-data")

		sess := NewSession(tc, *params,
			WithReconnectInfo(target.addr()),
			WithReconnectBackoff(10*time.Millisecond),
			WithMaxReconnectAttempts(3),
			WithKeepaliveInterval(60*time.Second),
		)
		t.Cleanup(func() { sess.Close() })

		cmd := Command{
			Write:                   true,
			ExpectedDataTransferLen: uint32(len(writeData)),
			Data:                    &nonSeekableReader{r: newSeekableReader(writeData)},
		}
		cmd.CDB[0] = 0x2A

		resultCh, err := sess.Submit(context.Background(), cmd)
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		select {
		case result := <-resultCh:
			if result.Err == nil {
				t.Fatal("expected ErrRetryNotPossible for non-seekable writer, got nil")
			}
			if !errors.Is(result.Err, ErrRetryNotPossible) {
				t.Fatalf("expected ErrRetryNotPossible, got: %v", result.Err)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for non-seekable write result")
		}
	})

	t.Run("seekable_after_all_dataout", func(t *testing.T) {
		// Write command with seekable reader. The target receives the command
		// but the connection is dropped before the SCSIResponse arrives.
		// Reconnect succeeds. Full retry from Seek(0, SeekStart) succeeds.
		target := startRecoverableTarget(t, nil)

		tc, params := connectAndLogin(t, target.addr())

		writeData := make([]byte, 64)
		for i := range writeData {
			writeData[i] = byte(i * 2)
		}

		sess := NewSession(tc, *params,
			WithReconnectInfo(target.addr()),
			WithReconnectBackoff(10*time.Millisecond),
			WithMaxReconnectAttempts(3),
			WithKeepaliveInterval(60*time.Second),
		)
		t.Cleanup(func() { sess.Close() })

		cmd := Command{
			Write:                   true,
			ExpectedDataTransferLen: uint32(len(writeData)),
			Data:                    newSeekableReader(writeData),
		}
		cmd.CDB[0] = 0x2A // WRITE(10)

		resultCh, err := sess.Submit(context.Background(), cmd)
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}

		// Give the write pump a moment to send the SCSICommand PDU and any
		// immediate data, then drop the underlying TCP connection to simulate
		// losing connectivity after Data-Out PDUs are in flight.
		time.Sleep(20 * time.Millisecond)
		tc.NetConn().Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		select {
		case result := <-resultCh:
			if result.Err != nil {
				t.Fatalf("expected success after seekable retry post data-out, got: %v", result.Err)
			}
			if result.Status != 0x00 {
				t.Fatalf("status: got 0x%02X, want 0x00", result.Status)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for seekable write result after all data-out + reconnect")
		}
	})
}

// newSeekableReader returns a seekable io.Reader backed by the given byte slice.
// Unlike bytes.NewReader, this is a distinct named type so tests can confirm
// the Seeker interface is present.
func newSeekableReader(data []byte) *seekableReader {
	return &seekableReader{data: data}
}

// seekableReader implements io.Reader and io.Seeker over a byte slice.
type seekableReader struct {
	data []byte
	pos  int
}

func (r *seekableReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *seekableReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = int64(r.pos) + offset
	case io.SeekEnd:
		abs = int64(len(r.data)) + offset
	default:
		return 0, errors.New("seekableReader: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("seekableReader: negative position")
	}
	r.pos = int(abs)
	return abs, nil
}
