// Package conformance_test contains IOL-inspired conformance tests
// exercising the public uiscsi API against an in-process mock target.
// All tests run automatically without manual SAN setup.
package conformance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
	testutil "github.com/rkujawa/uiscsi/test"
)

// setupTarget creates a MockTarget with login and logout handlers.
func setupTarget(t *testing.T) *testutil.MockTarget {
	t.Helper()
	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })
	tgt.HandleLogin()
	tgt.HandleLogout()
	return tgt
}

// TestLogin_AuthNone verifies basic login with AuthMethod=None.
// IOL: Login Phase - Normal Login with No Authentication.
func TestLogin_AuthNone(t *testing.T) {
	tgt := setupTarget(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.Close()

	// Session established successfully -- login passed.
}

// TestLogin_WithTarget verifies login with an explicit target IQN.
// IOL: Login Phase - Login with TargetName.
func TestLogin_WithTarget(t *testing.T) {
	tgt := setupTarget(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithTarget("iqn.2026-03.com.test:target"),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.Close()
}

// TestLogin_InvalidAddress verifies that dialing an unreachable address
// returns a *TransportError with Op="dial".
func TestLogin_InvalidAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := uiscsi.Dial(ctx, "192.0.2.1:59999") // RFC 5737 TEST-NET
	if err == nil {
		t.Fatal("expected error for unreachable address")
	}

	var te *uiscsi.TransportError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if te.Op != "dial" {
		t.Fatalf("Op: got %q, want %q", te.Op, "dial")
	}
}

// TestLogin_ContextCancel verifies that a canceled context returns an error.
func TestLogin_ContextCancel(t *testing.T) {
	tgt := setupTarget(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := uiscsi.Dial(ctx, tgt.Addr())
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

// TestLogin_MultipleSessions verifies that two sessions to the same target
// both succeed and both close cleanly.
func TestLogin_MultipleSessions(t *testing.T) {
	tgt := setupTarget(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess1, err := uiscsi.Dial(ctx, tgt.Addr())
	if err != nil {
		t.Fatalf("Dial(1): %v", err)
	}

	sess2, err := uiscsi.Dial(ctx, tgt.Addr())
	if err != nil {
		t.Fatalf("Dial(2): %v", err)
	}

	if err := sess1.Close(); err != nil {
		t.Fatalf("Close(1): %v", err)
	}
	if err := sess2.Close(); err != nil {
		t.Fatalf("Close(2): %v", err)
	}
}
