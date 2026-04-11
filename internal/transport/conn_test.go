package transport

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestConnDial_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()

	conn, err := Dial(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if conn.NetConn() == nil {
		t.Error("NetConn() returned nil")
	}
}

func TestConnDial_CancelledContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before dialing

	_, err = Dial(ctx, ln.Addr().String())
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestConnClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()

	conn, err := Dial(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Reading from closed connection should fail
	buf := make([]byte, 1)
	conn.NetConn().SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = conn.NetConn().Read(buf)
	if err == nil {
		t.Error("expected error reading from closed connection")
	}
}

func TestPoolBHS(t *testing.T) {
	bhs := GetBHS()
	if bhs == nil {
		t.Fatal("GetBHS returned nil")
	}
	if len(bhs) != 48 {
		t.Errorf("BHS length: got %d, want 48", len(bhs))
	}
	// Write something, return it, get a new one
	bhs[0] = 0xFF
	PutBHS(bhs)

	bhs2 := GetBHS()
	if bhs2 == nil {
		t.Fatal("second GetBHS returned nil")
	}
}

func TestPoolBuffer(t *testing.T) {
	buf := GetBuffer(1024)
	if len(*buf) < 1024 {
		t.Errorf("GetBuffer(1024): got len %d, want >= 1024", len(*buf))
	}
	PutBuffer(buf)

	// Large buffer
	big := GetBuffer(100000)
	if len(*big) < 100000 {
		t.Errorf("GetBuffer(100000): got len %d, want >= 100000", len(*big))
	}
	PutBuffer(big)
}
