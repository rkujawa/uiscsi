package session

import (
	"net"
	"sync"
	"testing"

	"github.com/uiscsi/uiscsi/internal/login"
	"github.com/uiscsi/uiscsi/internal/transport"
)

// TestSessionState_String verifies the human-readable names for each SessionState constant.
func TestSessionState_String(t *testing.T) {
	tests := []struct {
		state SessionState
		want  string
	}{
		{SessionLogin, "login"},
		{SessionFullFeature, "full-feature"},
		{SessionReconnecting, "reconnecting"},
		{SessionClosed, "closed"},
		{SessionState(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("SessionState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestStateChangeHook_Option verifies WithStateChangeHook sets the config field correctly.
func TestStateChangeHook_Option(t *testing.T) {
	called := false
	hook := func(SessionState) { called = true }

	cfg := defaultConfig()
	WithStateChangeHook(hook)(&cfg)

	if cfg.stateChangeHook == nil {
		t.Fatal("WithStateChangeHook: stateChangeHook field is nil after applying option")
	}

	cfg.stateChangeHook(SessionFullFeature)
	if !called {
		t.Error("WithStateChangeHook: hook was not invoked")
	}
}

// TestStateChangeHook_Transitions verifies the hook fires for FullFeature on NewSession
// and Closed on Close.
func TestStateChangeHook_Transitions(t *testing.T) {
	var states []SessionState
	var mu sync.Mutex
	hook := func(s SessionState) {
		mu.Lock()
		states = append(states, s)
		mu.Unlock()
	}

	clientConn, targetConn := net.Pipe()
	tc := transport.NewConnFromNetConn(clientConn)
	params := login.Defaults()
	params.CmdSN = 1
	params.ExpStatSN = 1

	sess := NewSession(tc, params, WithStateChangeHook(hook))

	// After NewSession, hook should have been called with SessionFullFeature.
	mu.Lock()
	if len(states) == 0 || states[0] != SessionFullFeature {
		mu.Unlock()
		t.Errorf("after NewSession: states = %v, want [full-feature]", states)
	} else {
		mu.Unlock()
	}

	// Close the session — hook should fire with SessionClosed.
	go respondToLogout(targetConn)
	sess.Close()
	targetConn.Close()

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, s := range states {
		if s == SessionClosed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("after Close: states = %v, want SessionClosed in list", states)
	}
}

// TestStateChangeHook_Nil verifies no panic when no hook is registered.
func TestStateChangeHook_Nil(t *testing.T) {
	clientConn, targetConn := net.Pipe()
	tc := transport.NewConnFromNetConn(clientConn)
	params := login.Defaults()
	params.CmdSN = 1
	params.ExpStatSN = 1

	// No hook registered — should not panic.
	sess := NewSession(tc, params)

	go respondToLogout(targetConn)
	sess.Close()
	targetConn.Close()
}
