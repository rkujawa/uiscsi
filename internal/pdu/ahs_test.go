package pdu

import (
	"bytes"
	"testing"
)

func TestMarshalAHSNil(t *testing.T) {
	out := MarshalAHS(nil)
	if len(out) != 0 {
		t.Errorf("MarshalAHS(nil) returned %d bytes, want 0", len(out))
	}
}

func TestMarshalAHSEmpty(t *testing.T) {
	out := MarshalAHS([]AHS{})
	if len(out) != 0 {
		t.Errorf("MarshalAHS([]) returned %d bytes, want 0", len(out))
	}
}

func TestAHSRoundTripSingle(t *testing.T) {
	// ExtendedCDB with 20 bytes of data
	data := make([]byte, 20)
	for i := range data {
		data[i] = byte(i + 1)
	}
	segments := []AHS{
		{Type: AHSExtendedCDB, Data: data},
	}
	encoded := MarshalAHS(segments)
	// Must be 4-byte aligned
	if len(encoded)%4 != 0 {
		t.Errorf("MarshalAHS output length %d is not 4-byte aligned", len(encoded))
	}

	decoded, err := UnmarshalAHS(encoded)
	if err != nil {
		t.Fatalf("UnmarshalAHS error: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 AHS segment, got %d", len(decoded))
	}
	if decoded[0].Type != AHSExtendedCDB {
		t.Errorf("type = %d, want %d", decoded[0].Type, AHSExtendedCDB)
	}
	if !bytes.Equal(decoded[0].Data, data) {
		t.Errorf("data mismatch after round-trip")
	}
}

func TestAHSRoundTripMultiple(t *testing.T) {
	segments := []AHS{
		{Type: AHSExtendedCDB, Data: []byte{1, 2, 3, 4, 5}},
		{Type: AHSBidiReadDataLen, Data: []byte{0, 0, 0x10, 0}},
	}
	encoded := MarshalAHS(segments)
	if len(encoded)%4 != 0 {
		t.Errorf("MarshalAHS output length %d is not 4-byte aligned", len(encoded))
	}

	decoded, err := UnmarshalAHS(encoded)
	if err != nil {
		t.Fatalf("UnmarshalAHS error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 AHS segments, got %d", len(decoded))
	}
	for i, seg := range segments {
		if decoded[i].Type != seg.Type {
			t.Errorf("segment %d: type = %d, want %d", i, decoded[i].Type, seg.Type)
		}
		if !bytes.Equal(decoded[i].Data, seg.Data) {
			t.Errorf("segment %d: data mismatch", i)
		}
	}
}

func TestUnmarshalAHSEmpty(t *testing.T) {
	decoded, err := UnmarshalAHS(nil)
	if err != nil {
		t.Errorf("UnmarshalAHS(nil) error: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected 0 segments, got %d", len(decoded))
	}

	decoded, err = UnmarshalAHS([]byte{})
	if err != nil {
		t.Errorf("UnmarshalAHS([]) error: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected 0 segments, got %d", len(decoded))
	}
}

func TestUnmarshalAHS_UnknownType(t *testing.T) {
	// Build AHS with unknown type 99 — should succeed (forward compatibility).
	segments := []AHS{{Type: 99, Data: []byte{0x01, 0x02}}}
	encoded := MarshalAHS(segments)
	decoded, err := UnmarshalAHS(encoded)
	if err != nil {
		t.Fatalf("unexpected error for unknown AHS type: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Type != 99 {
		t.Fatal("expected one segment with type 99")
	}
}

func TestUnmarshalAHS_ExcessiveLength(t *testing.T) {
	// Craft AHS header claiming dataLen > 16384.
	data := make([]byte, 8)
	data[0] = 0x40 // ahsLen high byte: 0x4002 = 16386
	data[1] = 0x04 // ahsLen low byte: ahsLen=16388 -> dataLen=16386
	data[2] = byte(AHSExtendedCDB)
	_, err := UnmarshalAHS(data)
	if err == nil {
		t.Fatal("expected error for excessive AHS length")
	}
}

func TestUnmarshalAHSTruncated(t *testing.T) {
	// Only 2 bytes -- not enough for a valid AHS header (4 bytes minimum)
	_, err := UnmarshalAHS([]byte{0x00, 0x01})
	if err == nil {
		t.Error("expected error for truncated AHS, got nil")
	}

	// Valid header claiming 8 bytes of data, but only 4 bytes provided total
	_, err = UnmarshalAHS([]byte{0x00, 0x08, 0x01, 0x00})
	if err == nil {
		t.Error("expected error for AHS with insufficient data, got nil")
	}
}
