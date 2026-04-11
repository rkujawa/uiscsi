package session

// SessionState represents the lifecycle state of an iSCSI session.
type SessionState uint8

// Session lifecycle state constants.
const (
	// SessionLogin indicates login negotiation is in progress.
	SessionLogin SessionState = iota
	// SessionFullFeature indicates the session is in full-feature phase
	// and accepts SCSI commands.
	SessionFullFeature
	// SessionReconnecting indicates an ERL 0 reconnect is in progress.
	// Commands submitted during this state block until reconnect succeeds
	// or the session is closed.
	SessionReconnecting
	// SessionClosed indicates the session has been permanently closed.
	SessionClosed
)

// String returns the human-readable name for the session state.
func (s SessionState) String() string {
	switch s {
	case SessionLogin:
		return "login"
	case SessionFullFeature:
		return "full-feature"
	case SessionReconnecting:
		return "reconnecting"
	case SessionClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// WithStateChangeHook registers a callback invoked when the session
// transitions between lifecycle states. The hook receives the new
// SessionState value. The hook is called from internal goroutines
// and MUST NOT block or call back into the session (Submit, Close,
// etc.) — doing so risks deadlock. fireStateChange is always called
// outside s.mu to prevent deadlock per the metricsHook pattern.
func WithStateChangeHook(h func(SessionState)) SessionOption {
	return func(c *sessionConfig) {
		c.stateChangeHook = h
	}
}
