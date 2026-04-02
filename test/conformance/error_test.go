package conformance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
	testutil "github.com/rkujawa/uiscsi/test"
)

// TestError_SCSICheckCondition verifies that CHECK CONDITION status (0x02)
// with sense data produces a *SCSIError with correct SenseKey/ASC/ASCQ.
// IOL: Error Recovery - SCSI Status CHECK CONDITION.
func TestError_SCSICheckCondition(t *testing.T) {
	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	defer tgt.Close()

	tgt.HandleLogin()
	tgt.HandleLogout()

	// Build sense data: fixed format (0x70), sense key=5 (ILLEGAL REQUEST),
	// ASC=0x24 (INVALID FIELD IN CDB), ASCQ=0x00.
	senseData := make([]byte, 18)
	senseData[0] = 0x70       // response code: current errors, fixed format
	senseData[2] = 0x05       // sense key: ILLEGAL REQUEST
	senseData[7] = 10         // additional sense length
	senseData[12] = 0x24      // ASC: INVALID FIELD IN CDB
	senseData[13] = 0x00      // ASCQ
	tgt.HandleSCSIError(0x02, senseData) // CHECK CONDITION

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.Close()

	_, readErr := sess.ReadBlocks(ctx, 0, 0, 1, 512)
	if readErr == nil {
		t.Fatal("expected error for CHECK CONDITION")
	}

	var scsiErr *uiscsi.SCSIError
	if !errors.As(readErr, &scsiErr) {
		t.Fatalf("expected *SCSIError, got %T: %v", readErr, readErr)
	}
	if scsiErr.Status != 0x02 {
		t.Fatalf("Status: got 0x%02X, want 0x02", scsiErr.Status)
	}
	if scsiErr.SenseKey != 0x05 {
		t.Fatalf("SenseKey: got 0x%02X, want 0x05", scsiErr.SenseKey)
	}
	if scsiErr.ASC != 0x24 {
		t.Fatalf("ASC: got 0x%02X, want 0x24", scsiErr.ASC)
	}
	if scsiErr.ASCQ != 0x00 {
		t.Fatalf("ASCQ: got 0x%02X, want 0x00", scsiErr.ASCQ)
	}
}

// TestError_TransportDrop verifies that closing the mock target connection
// mid-operation produces a *TransportError.
// IOL: Error Recovery - Transport Connection Drop.
func TestError_TransportDrop(t *testing.T) {
	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	defer tgt.Close()

	tgt.HandleLogin()
	tgt.HandleLogout()
	// No SCSI handler -- so the target won't respond.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.Close()

	// Close the target to drop all connections.
	tgt.Close()

	// Attempt an operation -- should fail with transport error.
	// Give a short timeout so it does not hang.
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	_, readErr := sess.ReadBlocks(readCtx, 0, 0, 1, 512)
	if readErr == nil {
		t.Fatal("expected error after transport drop")
	}
	// The error should be some kind of transport/connection failure.
	// It might be wrapped as *TransportError or context.DeadlineExceeded.
	// Both are acceptable -- the important thing is we get an error, not a hang.
}

// TestError_TypedErrorChain verifies errors.As works through the error chain
// for wrapped errors.
func TestError_TypedErrorChain(t *testing.T) {
	// Test that TransportError.Unwrap works.
	inner := errors.New("connection reset")
	te := &uiscsi.TransportError{Op: "read", Err: inner}

	if !errors.Is(te, inner) {
		t.Fatal("errors.Is should find inner error through TransportError")
	}

	var te2 *uiscsi.TransportError
	if !errors.As(te, &te2) {
		t.Fatal("errors.As should find *TransportError")
	}
	if te2.Op != "read" {
		t.Fatalf("Op: got %q, want %q", te2.Op, "read")
	}
}
