package conformance_test

import (
	"context"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
	"github.com/rkujawa/uiscsi/internal/pdu"
	testutil "github.com/rkujawa/uiscsi/test"
	"github.com/rkujawa/uiscsi/test/pducapture"
)

// TestDataOut_DataSN verifies that Data-Out DataSN starts at 0 and increments
// by 1 for each PDU within a single R2T burst.
// Conformance: DATA-01 (FFP #6.1)
func TestDataOut_DataSN(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	// Bilateral negotiation: ImmediateData=No, InitialR2T=Yes.
	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData:            testutil.BoolPtr(false),
		InitialR2T:               testutil.BoolPtr(true),
		MaxRecvDataSegmentLength: testutil.Uint32Ptr(512),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		// Send single R2T for full EDTL (2048 bytes).
		r2t := &pdu.R2T{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			TargetTransferTag:         0x00000001,
			StatSN:                    tc.StatSN(),
			ExpCmdSN:                  expCmdSN,
			MaxCmdSN:                  maxCmdSN,
			R2TSN:                     0,
			BufferOffset:              0,
			DesiredDataTransferLength: cmd.ExpectedDataTransferLength,
		}
		if err := tc.SendPDU(r2t); err != nil {
			return err
		}

		// Read all Data-Out PDUs until F-bit.
		if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
			return err
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	// Write 2048 bytes: at MaxRecvDSL=512, expect 4 Data-Out PDUs.
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := sess.WriteBlocks(ctx, 0, 0, 4, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) != 4 {
		t.Fatalf("captured Data-Out PDUs: got %d, want 4", len(douts))
	}

	for i, c := range douts {
		dout := c.Decoded.(*pdu.DataOut)
		if dout.DataSN != uint32(i) {
			t.Errorf("DataOut[%d].DataSN=%d, want %d", i, dout.DataSN, i)
		}
		if dout.TargetTransferTag != 0x00000001 {
			t.Errorf("DataOut[%d].TTT=0x%08X, want 0x00000001", i, dout.TargetTransferTag)
		}
	}
}

// TestDataOut_TTTEcho verifies that Data-Out PDUs echo the Target Transfer Tag
// from the R2T PDU.
// Conformance: DATA-05 (FFP #9.1)
func TestDataOut_TTTEcho(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData: testutil.BoolPtr(false),
		InitialR2T:    testutil.BoolPtr(true),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		// Use distinctive TTT value.
		r2t := &pdu.R2T{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			TargetTransferTag:         0xDEADBEEF,
			StatSN:                    tc.StatSN(),
			ExpCmdSN:                  expCmdSN,
			MaxCmdSN:                  maxCmdSN,
			R2TSN:                     0,
			BufferOffset:              0,
			DesiredDataTransferLength: cmd.ExpectedDataTransferLength,
		}
		if err := tc.SendPDU(r2t); err != nil {
			return err
		}

		if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
			return err
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	data := make([]byte, 512)
	if err := sess.WriteBlocks(ctx, 0, 0, 1, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) < 1 {
		t.Fatalf("captured Data-Out PDUs: got %d, want >= 1", len(douts))
	}

	for i, c := range douts {
		dout := c.Decoded.(*pdu.DataOut)
		if dout.TargetTransferTag != 0xDEADBEEF {
			t.Errorf("DataOut[%d].TTT=0x%08X, want 0xDEADBEEF", i, dout.TargetTransferTag)
		}
	}
}

// TestDataOut_MaxRecvDSL verifies that each Data-Out PDU's data segment length
// does not exceed the target's MaxRecvDataSegmentLength.
// Conformance: DATA-08 (FFP #11.1.1)
func TestDataOut_MaxRecvDSL(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData:            testutil.BoolPtr(false),
		InitialR2T:               testutil.BoolPtr(true),
		MaxRecvDataSegmentLength: testutil.Uint32Ptr(256),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		r2t := &pdu.R2T{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			TargetTransferTag:         0x00000001,
			StatSN:                    tc.StatSN(),
			ExpCmdSN:                  expCmdSN,
			MaxCmdSN:                  maxCmdSN,
			R2TSN:                     0,
			BufferOffset:              0,
			DesiredDataTransferLength: 1024,
		}
		if err := tc.SendPDU(r2t); err != nil {
			return err
		}

		if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
			return err
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	data := make([]byte, 1024)
	if err := sess.WriteBlocks(ctx, 0, 0, 2, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) < 1 {
		t.Fatalf("captured Data-Out PDUs: got %d, want >= 1", len(douts))
	}

	for i, c := range douts {
		dout := c.Decoded.(*pdu.DataOut)
		if dout.DataSegmentLen > 256 {
			t.Errorf("DataOut[%d].DataSegmentLen=%d, want <= 256", i, dout.DataSegmentLen)
		}
	}
}

// TestDataOut_FBitSolicited verifies that only the last Data-Out PDU in a
// solicited burst has the Final bit set.
// Conformance: DATA-11 (FFP #11.2.2)
func TestDataOut_FBitSolicited(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData:            testutil.BoolPtr(false),
		InitialR2T:               testutil.BoolPtr(true),
		MaxRecvDataSegmentLength: testutil.Uint32Ptr(512),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		// R2T for 1536 bytes -> 3 PDUs at 512.
		r2t := &pdu.R2T{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			TargetTransferTag:         0x00000001,
			StatSN:                    tc.StatSN(),
			ExpCmdSN:                  expCmdSN,
			MaxCmdSN:                  maxCmdSN,
			R2TSN:                     0,
			BufferOffset:              0,
			DesiredDataTransferLength: 1536,
		}
		if err := tc.SendPDU(r2t); err != nil {
			return err
		}

		if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
			return err
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	data := make([]byte, 1536)
	if err := sess.WriteBlocks(ctx, 0, 0, 3, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) != 3 {
		t.Fatalf("captured Data-Out PDUs: got %d, want 3", len(douts))
	}

	for i, c := range douts {
		dout := c.Decoded.(*pdu.DataOut)
		expectFinal := i == len(douts)-1
		if dout.Header.Final != expectFinal {
			t.Errorf("DataOut[%d].Final=%v, want %v", i, dout.Header.Final, expectFinal)
		}
	}
}

// TestDataOut_DataSNPerR2T verifies that DataSN resets to 0 for each new R2T
// sequence. Two R2T bursts should each start DataSN at 0.
// Conformance: DATA-12 (FFP #11.3)
func TestDataOut_DataSNPerR2T(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData:            testutil.BoolPtr(false),
		InitialR2T:               testutil.BoolPtr(true),
		MaxRecvDataSegmentLength: testutil.Uint32Ptr(512),
		MaxBurstLength:           testutil.Uint32Ptr(1024),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		// Send 2 R2Ts with distinct TTTs, each for 1024 bytes.
		ttts, err := testutil.SendR2TSequence(tc, cmd.InitiatorTaskTag,
			0, cmd.ExpectedDataTransferLength, 1024, 0x100, tgt.Session())
		if err != nil {
			return err
		}

		// Read Data-Out PDUs for each R2T burst.
		for range ttts {
			if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
				return err
			}
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	data := make([]byte, 2048)
	if err := sess.WriteBlocks(ctx, 0, 0, 4, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) != 4 {
		t.Fatalf("captured Data-Out PDUs: got %d, want 4", len(douts))
	}

	// Group by TTT. First 2 PDUs should be TTT=0x100, next 2 TTT=0x101.
	// Each group should have DataSN 0, 1.
	burst1TTT := douts[0].Decoded.(*pdu.DataOut).TargetTransferTag
	burst2TTT := douts[2].Decoded.(*pdu.DataOut).TargetTransferTag
	if burst1TTT == burst2TTT {
		t.Fatalf("both bursts have same TTT=0x%08X, expected different", burst1TTT)
	}

	// Burst 1: DataSN 0, 1.
	for i := 0; i < 2; i++ {
		dout := douts[i].Decoded.(*pdu.DataOut)
		if dout.TargetTransferTag != burst1TTT {
			t.Errorf("DataOut[%d].TTT=0x%08X, want 0x%08X", i, dout.TargetTransferTag, burst1TTT)
		}
		if dout.DataSN != uint32(i) {
			t.Errorf("DataOut[%d].DataSN=%d, want %d (burst 1)", i, dout.DataSN, i)
		}
	}

	// Burst 2: DataSN 0, 1 (reset).
	for i := 2; i < 4; i++ {
		dout := douts[i].Decoded.(*pdu.DataOut)
		if dout.TargetTransferTag != burst2TTT {
			t.Errorf("DataOut[%d].TTT=0x%08X, want 0x%08X", i, dout.TargetTransferTag, burst2TTT)
		}
		expectedDSN := uint32(i - 2)
		if dout.DataSN != expectedDSN {
			t.Errorf("DataOut[%d].DataSN=%d, want %d (burst 2)", i, dout.DataSN, expectedDSN)
		}
	}
}

// TestDataOut_BufferOffset verifies that BufferOffset increases correctly
// across Data-Out PDUs within a burst.
// Conformance: DATA-13 (FFP #11.4)
func TestDataOut_BufferOffset(t *testing.T) {
	rec := &pducapture.Recorder{}

	tgt, err := testutil.NewMockTarget()
	if err != nil {
		t.Fatalf("NewMockTarget: %v", err)
	}
	t.Cleanup(func() { tgt.Close() })

	tgt.SetNegotiationConfig(testutil.NegotiationConfig{
		ImmediateData:            testutil.BoolPtr(false),
		InitialR2T:               testutil.BoolPtr(true),
		MaxRecvDataSegmentLength: testutil.Uint32Ptr(512),
	})

	tgt.HandleLogin()
	tgt.HandleLogout()
	tgt.HandleNOPOut()
	tgt.HandleSCSIFunc(func(tc *testutil.TargetConn, cmd *pdu.SCSICommand, callCount int) error {
		expCmdSN, maxCmdSN := tgt.Session().Update(cmd.CmdSN, cmd.Header.Immediate)

		r2t := &pdu.R2T{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			TargetTransferTag:         0x00000001,
			StatSN:                    tc.StatSN(),
			ExpCmdSN:                  expCmdSN,
			MaxCmdSN:                  maxCmdSN,
			R2TSN:                     0,
			BufferOffset:              0,
			DesiredDataTransferLength: 2048,
		}
		if err := tc.SendPDU(r2t); err != nil {
			return err
		}

		if _, err := testutil.ReadDataOutPDUs(tc); err != nil {
			return err
		}

		resp := &pdu.SCSIResponse{
			Header: pdu.Header{
				Final:            true,
				InitiatorTaskTag: cmd.InitiatorTaskTag,
			},
			Status:   0x00,
			StatSN:   tc.NextStatSN(),
			ExpCmdSN: expCmdSN,
			MaxCmdSN: maxCmdSN,
		}
		return tc.SendPDU(resp)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr(),
		uiscsi.WithPDUHook(rec.Hook()),
		uiscsi.WithKeepaliveInterval(30*time.Second),
		uiscsi.WithOperationalOverrides(map[string]string{
			"ImmediateData": "No",
			"InitialR2T":    "Yes",
		}),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { sess.Close() })

	data := make([]byte, 2048)
	if err := sess.WriteBlocks(ctx, 0, 0, 4, 512, data); err != nil {
		t.Fatalf("WriteBlocks: %v", err)
	}

	douts := rec.Sent(pdu.OpDataOut)
	if len(douts) != 4 {
		t.Fatalf("captured Data-Out PDUs: got %d, want 4", len(douts))
	}

	expectedOffsets := []uint32{0, 512, 1024, 1536}
	for i, c := range douts {
		dout := c.Decoded.(*pdu.DataOut)
		if dout.BufferOffset != expectedOffsets[i] {
			t.Errorf("DataOut[%d].BufferOffset=%d, want %d", i, dout.BufferOffset, expectedOffsets[i])
		}
	}
}
