package pdu

import (
	"errors"
	"strings"
	"testing"
)

// TestProtocolErrorStructure verifies the ProtocolError type and ViolationKind enum.
func TestProtocolErrorStructure(t *testing.T) {
	pe := &ProtocolError{
		Kind:   BadOpcode,
		Op:     "decode",
		Detail: "unknown opcode 0x0f",
		Opcode: OpCode(0x0f),
		Got:    0x0f,
	}

	// Test Error() format.
	want := "iscsi protocol: decode: unknown opcode 0x0f"
	if got := pe.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Test errors.As.
	var extracted *ProtocolError
	err := error(pe)
	if !errors.As(err, &extracted) {
		t.Fatal("errors.As should match *ProtocolError")
	}
	if extracted.Kind != BadOpcode {
		t.Errorf("Kind = %v, want BadOpcode", extracted.Kind)
	}
	if extracted.Op != "decode" {
		t.Errorf("Op = %q, want %q", extracted.Op, "decode")
	}
	if extracted.Detail != "unknown opcode 0x0f" {
		t.Errorf("Detail = %q, want %q", extracted.Detail, "unknown opcode 0x0f")
	}
	if extracted.Opcode != OpCode(0x0f) {
		t.Errorf("Opcode = 0x%02x, want 0x0f", extracted.Opcode)
	}
	if extracted.Got != 0x0f {
		t.Errorf("Got = %d, want 15", extracted.Got)
	}
}

// TestProtocolErrorViolationKindString verifies the ViolationKind String() method.
func TestProtocolErrorViolationKindString(t *testing.T) {
	tests := []struct {
		kind ViolationKind
		want string
	}{
		{BadOpcode, "BadOpcode"},
		{OversizedSegment, "OversizedSegment"},
		{MalformedBHS, "MalformedBHS"},
		{MRDSLExceeded, "MRDSLExceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("ViolationKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

// TestProtocolErrorAllFields verifies all fields are accessible via errors.As.
func TestProtocolErrorAllFields(t *testing.T) {
	pe := &ProtocolError{
		Kind:   OversizedSegment,
		Op:     "encode",
		Detail: "DataSegmentLength 16777217 exceeds 24-bit maximum 0xFFFFFF",
		Got:    0x1000001,
		Limit:  0xFFFFFF,
	}

	var extracted *ProtocolError
	if !errors.As(error(pe), &extracted) {
		t.Fatal("errors.As should match *ProtocolError")
	}
	if extracted.Kind != OversizedSegment {
		t.Errorf("Kind = %v, want OversizedSegment", extracted.Kind)
	}
	if extracted.Got != 0x1000001 {
		t.Errorf("Got = %d, want 16777217", extracted.Got)
	}
	if extracted.Limit != 0xFFFFFF {
		t.Errorf("Limit = 0x%x, want 0xFFFFFF", extracted.Limit)
	}
}

// TestDecodeBHSUnknownOpcodeProtocolError verifies that DecodeBHS returns *ProtocolError for unknown opcodes.
func TestDecodeBHSUnknownOpcodeProtocolError(t *testing.T) {
	// These opcodes are NOT in the valid set (see opcode.go).
	unknownOpcodes := []OpCode{
		0x07, // gap in initiator range
		0x08, // gap in initiator range
		0x0F, // unused
		0x1F, // unused
		0x27, // gap in target range
		0x3E, // unused target opcode
	}

	for _, opcode := range unknownOpcodes {
		t.Run(opcode.String(), func(t *testing.T) {
			var bhs [BHSLength]byte
			bhs[0] = byte(opcode)

			_, err := DecodeBHS(bhs)
			if err == nil {
				t.Fatalf("DecodeBHS(0x%02x): expected error, got nil", uint8(opcode))
			}

			var pe *ProtocolError
			if !errors.As(err, &pe) {
				t.Fatalf("DecodeBHS(0x%02x): expected *ProtocolError, got %T: %v", uint8(opcode), err, err)
			}
			if pe.Kind != BadOpcode {
				t.Errorf("Kind = %v, want BadOpcode", pe.Kind)
			}
			if pe.Opcode != opcode {
				t.Errorf("Opcode = 0x%02x, want 0x%02x", uint8(pe.Opcode), uint8(opcode))
			}
			if pe.Op != "decode" {
				t.Errorf("Op = %q, want %q", pe.Op, "decode")
			}
			if !strings.Contains(pe.Detail, "unknown opcode") {
				t.Errorf("Detail %q should mention 'unknown opcode'", pe.Detail)
			}
		})
	}
}

// TestDecodeBHSValidOpcodes verifies all 18 valid opcodes decode without error (regression).
func TestDecodeBHSValidOpcodes(t *testing.T) {
	validOpcodes := []OpCode{
		// Initiator opcodes
		OpNOPOut, OpSCSICommand, OpTaskMgmtReq, OpLoginReq,
		OpTextReq, OpDataOut, OpLogoutReq, OpSNACKReq,
		// Target opcodes
		OpNOPIn, OpSCSIResponse, OpTaskMgmtResp, OpLoginResp,
		OpTextResp, OpDataIn, OpLogoutResp, OpR2T, OpAsyncMsg, OpReject,
	}

	for _, opcode := range validOpcodes {
		t.Run(opcode.String(), func(t *testing.T) {
			var bhs [BHSLength]byte
			bhs[0] = byte(opcode)
			// Bit 7 of byte 1 set = Final flag (required for valid packets).
			bhs[1] = 0x80

			_, err := DecodeBHS(bhs)
			if err != nil {
				t.Errorf("DecodeBHS(valid opcode 0x%02x): unexpected error: %v", uint8(opcode), err)
			}
		})
	}
}

// TestEncodeDataSegmentLengthProtocolError verifies encodeDataSegmentLength returns *ProtocolError.
func TestEncodeDataSegmentLengthProtocolError(t *testing.T) {
	bhs := make([]byte, BHSLength)
	err := encodeDataSegmentLength(bhs, 0x1000000) // > 0xFFFFFF
	if err == nil {
		t.Fatal("expected error for dsLen > 0xFFFFFF")
	}

	var pe *ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ProtocolError, got %T: %v", err, err)
	}
	if pe.Kind != OversizedSegment {
		t.Errorf("Kind = %v, want OversizedSegment", pe.Kind)
	}
	if pe.Op != "encode" {
		t.Errorf("Op = %q, want encode", pe.Op)
	}
	if pe.Got != 0x1000000 {
		t.Errorf("Got = %d, want 16777216", pe.Got)
	}
	if pe.Limit != 0xFFFFFF {
		t.Errorf("Limit = 0x%x, want 0xFFFFFF", pe.Limit)
	}
}

// TestEncodeDataSegmentLengthMaxValid verifies 0xFFFFFF (24-bit max) succeeds (regression).
func TestEncodeDataSegmentLengthMaxValid(t *testing.T) {
	bhs := make([]byte, BHSLength)
	if err := encodeDataSegmentLength(bhs, 0xFFFFFF); err != nil {
		t.Fatalf("encodeDataSegmentLength(0xFFFFFF): unexpected error: %v", err)
	}
	got := decodeDataSegmentLength(bhs)
	if got != 0xFFFFFF {
		t.Errorf("decoded value = 0x%x, want 0xFFFFFF", got)
	}
}
