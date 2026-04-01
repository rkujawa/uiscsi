package transport

import (
	"io"
	"net"
	"sync"
	"testing"
)

func TestFaultConn(t *testing.T) {
	t.Run("read_passthrough", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, nil, nil)

		want := []byte("hello")
		go func() {
			c2.Write(want)
		}()

		buf := make([]byte, 16)
		n, err := fc.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if string(buf[:n]) != "hello" {
			t.Fatalf("Read: got %q, want %q", buf[:n], "hello")
		}
	})

	t.Run("write_passthrough", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, nil, nil)

		want := []byte("world")
		go func() {
			fc.Write(want)
		}()

		buf := make([]byte, 16)
		n, err := c2.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if string(buf[:n]) != "world" {
			t.Fatalf("Read: got %q, want %q", buf[:n], "world")
		}
	})

	t.Run("read_fault_after_bytes", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, WithReadFaultAfter(100, io.ErrClosedPipe), nil)

		// Write 120 bytes from c2 side.
		go func() {
			c2.Write(make([]byte, 120))
		}()

		// Read in chunks. First reads succeed until cumulative >= 100.
		total := 0
		buf := make([]byte, 30)
		for {
			n, err := fc.Read(buf)
			total += n
			if err != nil {
				if err != io.ErrClosedPipe {
					t.Fatalf("Read error: got %v, want %v", err, io.ErrClosedPipe)
				}
				break
			}
		}
		// We should have read some bytes before the fault triggers.
		// The fault triggers when readCount >= 100, which happens BEFORE
		// the next Read call after 100 bytes are accumulated.
		if total < 90 {
			t.Fatalf("total bytes read before fault: got %d, want >= 90", total)
		}
	})

	t.Run("write_fault_after_bytes", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, nil, WithWriteFaultAfter(50, io.ErrClosedPipe))

		// Drain from c2 side.
		go func() {
			io.Copy(io.Discard, c2)
		}()

		// Write in chunks. First writes succeed until cumulative >= 50.
		total := 0
		chunk := make([]byte, 20)
		for i := range 10 {
			n, err := fc.Write(chunk)
			total += n
			if err != nil {
				if err != io.ErrClosedPipe {
					t.Fatalf("Write error at iteration %d: got %v, want %v", i, err, io.ErrClosedPipe)
				}
				break
			}
		}
		if total < 40 {
			t.Fatalf("total bytes written before fault: got %d, want >= 40", total)
		}
	})

	t.Run("concurrent_safety", func(t *testing.T) {
		// Verify no data race when SetReadFault/SetWriteFault are called
		// concurrently with Read/Write operations.
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, nil, nil)

		var wg sync.WaitGroup

		// Concurrently set faults from multiple goroutines.
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				fc.SetReadFault(func(_ int64) error { return nil })
				fc.SetWriteFault(func(_ int64) error { return nil })
			}()
		}

		wg.Wait()

		// After concurrent setting, verify the conn still works.
		go func() {
			c2.Write([]byte("ok"))
		}()
		buf := make([]byte, 4)
		n, err := fc.Read(buf)
		if err != nil {
			t.Fatalf("Read after concurrent Set: %v", err)
		}
		if string(buf[:n]) != "ok" {
			t.Fatalf("got %q, want %q", buf[:n], "ok")
		}
	})

	t.Run("set_fault_at_runtime", func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()

		fc := NewFaultConn(c1, nil, nil)

		// Write should succeed initially.
		go func() {
			io.Copy(io.Discard, c2)
		}()
		_, err := fc.Write([]byte("ok"))
		if err != nil {
			t.Fatalf("initial Write: %v", err)
		}

		// Set write fault at runtime.
		fc.SetWriteFault(func(_ int64) error {
			return io.ErrClosedPipe
		})

		_, err = fc.Write([]byte("fail"))
		if err != io.ErrClosedPipe {
			t.Fatalf("Write after SetWriteFault: got %v, want %v", err, io.ErrClosedPipe)
		}
	})
}
