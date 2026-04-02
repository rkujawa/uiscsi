// options.go defines functional options for Dial and Discover.
package uiscsi

import (
	"log/slog"
	"time"

	"github.com/rkujawa/uiscsi/internal/login"
	"github.com/rkujawa/uiscsi/internal/session"
	"github.com/rkujawa/uiscsi/internal/transport"
)

// Option configures a Dial or Discover call via the functional options pattern.
type Option func(*dialConfig)

// dialConfig holds the accumulated options for Dial/Discover.
type dialConfig struct {
	loginOpts   []login.LoginOption
	sessionOpts []session.SessionOption
}

// WithTarget sets the target IQN for login.
func WithTarget(iqn string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithTarget(iqn))
	}
}

// WithCHAP enables CHAP authentication.
func WithCHAP(user, secret string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithCHAP(user, secret))
	}
}

// WithMutualCHAP enables mutual CHAP authentication.
func WithMutualCHAP(user, secret, targetSecret string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithMutualCHAP(user, secret, targetSecret))
	}
}

// WithInitiatorName sets the initiator IQN.
func WithInitiatorName(iqn string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithInitiatorName(iqn))
	}
}

// WithHeaderDigest sets header digest preferences.
func WithHeaderDigest(prefs ...string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithHeaderDigest(prefs...))
	}
}

// WithDataDigest sets data digest preferences.
func WithDataDigest(prefs ...string) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithDataDigest(prefs...))
	}
}

// WithLogger sets the slog.Logger for both session and login diagnostics.
func WithLogger(l *slog.Logger) Option {
	return func(c *dialConfig) {
		c.loginOpts = append(c.loginOpts, login.WithLoginLogger(l))
		c.sessionOpts = append(c.sessionOpts, session.WithLogger(l))
	}
}

// WithKeepaliveInterval sets the keepalive ping interval.
func WithKeepaliveInterval(d time.Duration) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithKeepaliveInterval(d))
	}
}

// WithKeepaliveTimeout sets the keepalive timeout.
func WithKeepaliveTimeout(d time.Duration) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithKeepaliveTimeout(d))
	}
}

// WithAsyncHandler registers an async event callback.
func WithAsyncHandler(h func(AsyncEvent)) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithAsyncHandler(func(ae session.AsyncEvent) {
			h(convertAsyncEvent(ae))
		}))
	}
}

// WithPDUHook registers a PDU send/receive hook. The []byte argument is the
// concatenation of BHS (48 bytes) + DataSegment from the internal
// transport.RawPDU. This avoids exposing internal transport types.
func WithPDUHook(h func(PDUDirection, []byte)) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithPDUHook(func(dir session.PDUDirection, raw *transport.RawPDU) {
			pubDir := PDUDirection(dir)
			data := make([]byte, len(raw.BHS)+len(raw.DataSegment))
			copy(data, raw.BHS[:])
			copy(data[len(raw.BHS):], raw.DataSegment)
			h(pubDir, data)
		}))
	}
}

// WithMetricsHook registers a metrics callback.
func WithMetricsHook(h func(MetricEvent)) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithMetricsHook(func(me session.MetricEvent) {
			h(convertMetricEvent(me))
		}))
	}
}

// WithMaxReconnectAttempts sets the maximum number of ERL 0 reconnect attempts.
func WithMaxReconnectAttempts(n int) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithMaxReconnectAttempts(n))
	}
}

// WithReconnectBackoff sets the reconnect backoff duration.
func WithReconnectBackoff(base time.Duration) Option {
	return func(c *dialConfig) {
		c.sessionOpts = append(c.sessionOpts, session.WithReconnectBackoff(base))
	}
}
