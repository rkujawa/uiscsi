package transport

import (
	"net"
	"sync"
)

// FaultConn wraps a net.Conn with injectable read/write faults for
// deterministic error injection in tests. Faults trigger based on
// cumulative byte counts, allowing precise control over when errors
// occur during iSCSI PDU exchanges.
type FaultConn struct {
	net.Conn

	mu         sync.Mutex
	readCount  int64
	writeCount int64
	readFault  func(bytesRead int64) error
	writeFault func(bytesWritten int64) error
}

// NewFaultConn wraps conn with optional read and write fault functions.
// If readFault or writeFault is nil, the corresponding operation passes
// through to the underlying connection without fault injection.
func NewFaultConn(conn net.Conn, readFault, writeFault func(int64) error) *FaultConn {
	return &FaultConn{
		Conn:       conn,
		readFault:  readFault,
		writeFault: writeFault,
	}
}

// Read implements io.Reader with fault injection. Before each read, it
// checks the readFault function (if set) against the cumulative byte count.
func (fc *FaultConn) Read(p []byte) (int, error) {
	fc.mu.Lock()
	if fc.readFault != nil {
		if err := fc.readFault(fc.readCount); err != nil {
			fc.mu.Unlock()
			return 0, err
		}
	}
	fc.mu.Unlock()

	n, err := fc.Conn.Read(p)

	fc.mu.Lock()
	fc.readCount += int64(n)
	fc.mu.Unlock()

	return n, err
}

// Write implements io.Writer with fault injection. Before each write, it
// checks the writeFault function (if set) against the cumulative byte count.
func (fc *FaultConn) Write(p []byte) (int, error) {
	fc.mu.Lock()
	if fc.writeFault != nil {
		if err := fc.writeFault(fc.writeCount); err != nil {
			fc.mu.Unlock()
			return 0, err
		}
	}
	fc.mu.Unlock()

	n, err := fc.Conn.Write(p)

	fc.mu.Lock()
	fc.writeCount += int64(n)
	fc.mu.Unlock()

	return n, err
}

// SetReadFault sets or replaces the read fault function at runtime.
func (fc *FaultConn) SetReadFault(f func(int64) error) {
	fc.mu.Lock()
	fc.readFault = f
	fc.mu.Unlock()
}

// SetWriteFault sets or replaces the write fault function at runtime.
func (fc *FaultConn) SetWriteFault(f func(int64) error) {
	fc.mu.Lock()
	fc.writeFault = f
	fc.mu.Unlock()
}

// WithReadFaultAfter returns a fault function that triggers the given error
// once cumulative bytes read reaches or exceeds n.
func WithReadFaultAfter(n int64, err error) func(int64) error {
	return func(bytesRead int64) error {
		if bytesRead >= n {
			return err
		}
		return nil
	}
}

// WithWriteFaultAfter returns a fault function that triggers the given error
// once cumulative bytes written reaches or exceeds n.
func WithWriteFaultAfter(n int64, err error) func(int64) error {
	return func(bytesWritten int64) error {
		if bytesWritten >= n {
			return err
		}
		return nil
	}
}
