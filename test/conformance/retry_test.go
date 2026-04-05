package conformance_test

import (
	"context"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
	"github.com/rkujawa/uiscsi/internal/pdu"
	"github.com/rkujawa/uiscsi/internal/transport"
	testutil "github.com/rkujawa/uiscsi/test"
	"github.com/rkujawa/uiscsi/test/pducapture"
)

// TestRetry_RejectCallerReissue tests the initiator's behavior after receiving
// a Reject PDU at ERL=1 (CMDSEQ-07 / FFP #4.1).
//
// FFP #4.1 specifies same-connection retry with original ITT, CDB, CmdSN.
// However, the production code (recovery.go retryTasks) always allocates new
// ITT and CmdSN. At ERL=1, a Reject cancels the in-flight task; the caller
// receives an error and re-issues a new command.
//
// This test verifies: Reject -> task cancelled -> caller re-issues ->
// new command succeeds with new ITT/CmdSN but same CDB (READ(10) for same LBA).
//
// TODO(conformance): Implement same-connection retry with original ITT/CmdSN
// per RFC 7143 Section 6.2.1 for full FFP #4.1 compliance.
func TestRetry_RejectCallerReissue(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	// ERL=1 required for SNACK support and Reject handling.
	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ErrorRecoveryLevel: testutil.Uint32Ptr(1),
	})
	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()

	// Register a SNACK handler to drain any SNACK PDUs the initiator sends
	// (the Status SNACK timer may fire after the task is cancelled).
	tgt.Handle(pdu.OpSNACKReq, func(tc *testutil.TargetConn, raw *transport.RawPDU, decoded pdu.PDU) error {
		// Silently consume.
		return nil
	})

	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		if callCount == 0 {
			// First command: send a Reject PDU with Reason=0x09 (Invalid PDU Field).
			// The data segment contains the complete BHS of the rejected command.
			rejectedBHS, err := cmd.MarshalBHS()
			if err != nil {
				return err
			}

			reject := &pdu.Reject{
				Header: pdu.Header{
					Final:            true,
					InitiatorTaskTag: 0xFFFFFFFF,
					DataSegmentLen:   uint32(len(rejectedBHS)),
				},
				Reason:   0x09, // Invalid PDU Field
				StatSN:   tc.NextStatSN(),
				ExpCmdSN: expCmdSN,
				MaxCmdSN: maxCmdSN,
				Data:     rejectedBHS[:],
			}
			return tc.SendPDU(reject)
		}

		// callCount >= 1: respond normally with Data-In (HasStatus=true).
		data := make([]byte, 512)
		din := &pdu.DataIn{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
				DataSegmentLen:   512,
			},
			DataSN:       0,
			BufferOffset: 0,
			HasStatus:    true,
			Status:       0x00,
			StatSN:       tc.NextStatSN(),
			ExpCmdSN:     expCmdSN,
			MaxCmdSN:     maxCmdSN,
			Data:         data,
		}
		return tc.SendPDU(din)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ErrorRecoveryLevel": "1",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	// First ReadBlocks should fail (Reject cancels the task).
	_, firstErr := sess.ReadBlocks(ctx, 0, 0, 1, 512)
	if firstErr == nil {
		t.Fatal("expected first ReadBlocks to fail after Reject")
	}
	t.Logf("first ReadBlocks error (expected): %v", firstErr)

	// Allow async processing to settle.
	time.Sleep(200 * time.Millisecond)

	// Second ReadBlocks should succeed (caller re-issues with new ITT/CmdSN).
	data, secondErr := sess.ReadBlocks(ctx, 0, 0, 1, 512)
	if secondErr != nil {
		t.Fatalf("second ReadBlocks should succeed, got: %v", secondErr)
	}
	if len(data) != 512 {
		t.Fatalf("second ReadBlocks returned %d bytes, want 512", len(data))
	}

	// Verify pducapture: at least 2 SCSI Command PDUs were sent.
	cmds := rec.Sent(pdu.OpSCSICommand)
	if len(cmds) < 2 {
		t.Fatalf("captured SCSI commands: got %d, want >= 2", len(cmds))
	}

	first := cmds[0].Decoded.(*pdu.SCSICommand)
	second := cmds[1].Decoded.(*pdu.SCSICommand)

	// Assert CmdSN[1] > CmdSN[0] (NEW CmdSN, not original).
	if second.CmdSN <= first.CmdSN {
		t.Fatalf("CmdSN not incremented: first=%d, second=%d (want second > first)",
			first.CmdSN, second.CmdSN)
	}
	t.Logf("CmdSN: first=%d, second=%d (incremented as expected)", first.CmdSN, second.CmdSN)

	// Assert ITT[1] != ITT[0] (NEW ITT, not original).
	if second.InitiatorTaskTag == first.InitiatorTaskTag {
		t.Fatalf("ITT not changed: first=0x%08X, second=0x%08X (want different)",
			first.InitiatorTaskTag, second.InitiatorTaskTag)
	}
	t.Logf("ITT: first=0x%08X, second=0x%08X (different as expected)",
		first.InitiatorTaskTag, second.InitiatorTaskTag)

	// Assert both have the same CDB bytes (both are READ(10) for LBA 0, 1 block).
	if len(first.CDB) != len(second.CDB) {
		t.Fatalf("CDB length mismatch: first=%d, second=%d", len(first.CDB), len(second.CDB))
	}
	for i := range first.CDB {
		if first.CDB[i] != second.CDB[i] {
			t.Fatalf("CDB byte %d differs: first=0x%02X, second=0x%02X",
				i, first.CDB[i], second.CDB[i])
		}
	}
	t.Logf("CDB: identical (both READ(10) for same LBA/block count)")
}
