package transport

import (
	"context"
	"net"
	"time"
)

// Conn wraps a net.Conn with iSCSI transport-level state such as digest
// negotiation flags and MaxRecvDataSegmentLength enforcement.
type Conn struct {
	conn net.Conn

	// Digest flags -- set after login negotiation, default false.
	digestHeader bool
	digestData   bool

	// MaxRecvDataSegmentLength -- enforced at transport layer (D-06).
	// Zero means not yet negotiated.
	maxRecvDSL uint32
}

// NewConnFromNetConn wraps an existing net.Conn as a transport Conn.
// Used by the session layer and tests when the TCP connection is already
// established (e.g., after login).
func NewConnFromNetConn(nc net.Conn) *Conn {
	return &Conn{conn: nc}
}

// DigestHeader reports whether header digests are enabled.
func (c *Conn) DigestHeader() bool { return c.digestHeader }

// DigestData reports whether data digests are enabled.
func (c *Conn) DigestData() bool { return c.digestData }

// Dial connects to an iSCSI target at the given TCP address using the
// provided context for timeout/cancellation control.
func Dial(ctx context.Context, addr string) (*Conn, error) {
	d := net.Dialer{}
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Conn{conn: nc}, nil
}

// Close shuts down the underlying TCP connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// SetDeadline sets the read and write deadlines on the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// NetConn returns the underlying net.Conn. Used for testing and low-level access.
func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// SetDigests configures whether header and data digests are active on this
// connection. Called after login negotiation completes.
func (c *Conn) SetDigests(header, data bool) {
	c.digestHeader = header
	c.digestData = data
}

// SetMaxRecvDSL sets the MaxRecvDataSegmentLength for this connection.
// The transport layer enforces this limit when framing incoming PDUs.
func (c *Conn) SetMaxRecvDSL(maxDSL uint32) {
	c.maxRecvDSL = maxDSL
}

// MaxRecvDSL returns the negotiated MaxRecvDataSegmentLength.
// Zero means not yet negotiated (no limit enforced).
func (c *Conn) MaxRecvDSL() uint32 { return c.maxRecvDSL }
