// Package session implements the iSCSI session layer: command dispatch,
// CmdSN flow control, Data-In reassembly, and session lifecycle management
// per RFC 7143.
package session

import (
	"io"
	"log/slog"
	"time"
)

// Command represents a SCSI command to be submitted via the session.
// The caller fills in CDB, data direction flags, and transfer length;
// the session assigns CmdSN, ITT, and ExpStatSN.
type Command struct {
	CDB                    [16]byte
	Read                   bool
	Write                  bool
	ExpectedDataTransferLen uint32
	LUN                    uint64
	ImmediateData          []byte
	TaskAttributes         uint8
}

// Result carries the outcome of a submitted SCSI command.
// For read commands, Data is an io.Reader that streams the response data
// assembled from one or more Data-In PDUs. For non-read commands Data is nil.
type Result struct {
	Status        uint8
	SenseData     []byte
	Data          io.Reader // nil for non-read commands
	Overflow      bool
	Underflow     bool
	ResidualCount uint32
	Err           error // non-nil if the command failed at the transport level
}

// AsyncEvent carries an asynchronous event message from the target.
// RFC 7143 Section 11.9.
type AsyncEvent struct {
	EventCode  uint8
	VendorCode uint8
	Parameter1 uint16
	Parameter2 uint16
	Parameter3 uint16
	Data       []byte
}

// DiscoveryTarget represents a target discovered via SendTargets.
type DiscoveryTarget struct {
	Name    string
	Portals []Portal
}

// Portal represents a target portal (address + port + portal group tag).
type Portal struct {
	Address  string
	Port     int
	GroupTag int
}

// SessionOption configures a Session via the functional options pattern.
type SessionOption func(*sessionConfig)

// sessionConfig holds tunable session parameters. Unexported to enforce
// construction via SessionOption functions.
type sessionConfig struct {
	keepaliveInterval time.Duration
	keepaliveTimeout  time.Duration
	asyncHandler      func(AsyncEvent)
	logger            *slog.Logger
}

// defaultConfig returns a sessionConfig with sensible defaults.
func defaultConfig() sessionConfig {
	return sessionConfig{
		keepaliveInterval: 30 * time.Second,
		keepaliveTimeout:  5 * time.Second,
		logger:            slog.Default(),
	}
}

// WithKeepaliveInterval sets the interval between NOP-Out keepalive pings.
func WithKeepaliveInterval(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.keepaliveInterval = d
	}
}

// WithKeepaliveTimeout sets the deadline for a NOP-In reply to a keepalive ping.
func WithKeepaliveTimeout(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.keepaliveTimeout = d
	}
}

// WithAsyncHandler registers a callback invoked for each AsyncMsg received
// from the target. If nil, async events are logged and discarded.
func WithAsyncHandler(h func(AsyncEvent)) SessionOption {
	return func(c *sessionConfig) {
		c.asyncHandler = h
	}
}

// WithLogger overrides the default slog.Logger for session diagnostics.
func WithLogger(l *slog.Logger) SessionOption {
	return func(c *sessionConfig) {
		c.logger = l
	}
}
