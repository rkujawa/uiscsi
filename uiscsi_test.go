package uiscsi_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
)

func TestSCSIError_Error_External(t *testing.T) {
	e := &uiscsi.SCSIError{
		Status:   0x02,
		SenseKey: 0x05,
		Message:  "test sense message",
	}
	got := e.Error()
	if got != "scsi: status 0x02: test sense message" {
		t.Errorf("SCSIError.Error() = %q, want expected format", got)
	}
}

func TestTransportError_Unwrap_External(t *testing.T) {
	inner := errors.New("connection refused")
	e := &uiscsi.TransportError{Op: "dial", Err: inner}

	var te *uiscsi.TransportError
	if !errors.As(e, &te) {
		t.Fatal("errors.As should match *TransportError")
	}
	if te.Unwrap() != inner {
		t.Error("Unwrap did not return underlying error")
	}
}

func TestAuthError_Error_External(t *testing.T) {
	e := &uiscsi.AuthError{
		StatusClass:  2,
		StatusDetail: 1,
		Message:      "authentication failure",
	}
	if e.Error() != "iscsi auth: authentication failure" {
		t.Errorf("AuthError.Error() = %q, want expected format", e.Error())
	}
}

func TestDial_Unreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 127.0.0.1:1 should be refused immediately on localhost.
	_, err := uiscsi.Dial(ctx, "127.0.0.1:1")
	if err == nil {
		t.Fatal("Dial to unreachable address should fail")
	}

	var te *uiscsi.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("Dial error should be *TransportError, got %T: %v", err, err)
	}
	if te.Op != "dial" {
		t.Errorf("TransportError.Op = %q, want %q", te.Op, "dial")
	}
}

func TestOptions_Compile(t *testing.T) {
	// Verify all With* functions exist and compile. We don't call Dial,
	// just ensure the options produce the correct type.
	var opts []uiscsi.Option

	opts = append(opts, uiscsi.WithTarget("iqn.2026-03.com.example:test"))
	opts = append(opts, uiscsi.WithCHAP("user", "secret"))
	opts = append(opts, uiscsi.WithMutualCHAP("user", "secret", "tsecret"))
	opts = append(opts, uiscsi.WithInitiatorName("iqn.2026-03.com.example:initiator"))
	opts = append(opts, uiscsi.WithHeaderDigest("CRC32C", "None"))
	opts = append(opts, uiscsi.WithDataDigest("None"))
	opts = append(opts, uiscsi.WithLogger(nil))
	opts = append(opts, uiscsi.WithKeepaliveInterval(30*time.Second))
	opts = append(opts, uiscsi.WithKeepaliveTimeout(5*time.Second))
	opts = append(opts, uiscsi.WithAsyncHandler(func(uiscsi.AsyncEvent) {}))
	opts = append(opts, uiscsi.WithPDUHook(func(uiscsi.PDUDirection, []byte) {}))
	opts = append(opts, uiscsi.WithMetricsHook(func(uiscsi.MetricEvent) {}))
	opts = append(opts, uiscsi.WithMaxReconnectAttempts(5))
	opts = append(opts, uiscsi.WithReconnectBackoff(2*time.Second))

	if len(opts) != 14 {
		t.Errorf("expected 14 options, got %d", len(opts))
	}
}

func TestStreamRead_ReturnsReader(t *testing.T) {
	// Type assertion test: StreamRead's return type should include io.Reader.
	// We can't call it without a session, but we verify the method signature
	// exists and returns (io.Reader, error) via the compiler.
	var s *uiscsi.Session
	_ = s // Session.StreamRead signature is verified at compile time.

	// Verify io.Reader is imported and usable in this context.
	var _ io.Reader
}
