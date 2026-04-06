package uiscsi

import (
	"errors"
	"testing"
)

func TestParseSenseDataEmpty(t *testing.T) {
	si, err := ParseSenseData(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si != nil {
		t.Fatal("expected nil SenseInfo for empty data")
	}
}

func TestParseSenseDataFixed(t *testing.T) {
	// Fixed format: sense key=MEDIUM ERROR, ASC=0x11, ASCQ=0x00, Filemark=true
	data := []byte{
		0xF0,                   // valid=1, response code=0x70
		0x00,                   // segment
		0x83,                   // filemark=1, key=MEDIUM ERROR (0x03)
		0x00, 0x00, 0x00, 0x00, // information
		0x0A,                   // additional sense length
		0x00, 0x00, 0x00, 0x00, // command-specific
		0x11, 0x00,             // ASC=0x11, ASCQ=0x00
		0x00,                   // FRU
		0x00, 0x00, 0x00,       // sense key specific
	}

	si, err := ParseSenseData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Key != 0x03 {
		t.Errorf("Key: got 0x%02X, want 0x03", si.Key)
	}
	if si.ASC != 0x11 {
		t.Errorf("ASC: got 0x%02X, want 0x11", si.ASC)
	}
	if !si.Filemark {
		t.Error("expected Filemark=true")
	}
	if si.Description == "" {
		t.Error("expected non-empty Description")
	}
}

func TestParseSenseDataDescriptor(t *testing.T) {
	data := []byte{
		0x72,                         // response code=0x72
		0x05,                         // sense key = ILLEGAL REQUEST
		0x24, 0x00,                   // ASC/ASCQ
		0x00, 0x00, 0x00,             // reserved
		0x00,                         // additional sense length
	}

	si, err := ParseSenseData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Key != 0x05 {
		t.Errorf("Key: got 0x%02X, want 0x05", si.Key)
	}
	if si.ASC != 0x24 {
		t.Errorf("ASC: got 0x%02X, want 0x24", si.ASC)
	}
}

func TestParseSenseDataInvalid(t *testing.T) {
	_, err := ParseSenseData([]byte{0x7E, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected error for unknown response code")
	}
}

func TestCheckStatusGood(t *testing.T) {
	err := CheckStatus(0x00, nil)
	if err != nil {
		t.Fatalf("expected nil for GOOD status, got: %v", err)
	}
}

func TestCheckStatusCheckCondition(t *testing.T) {
	sense := []byte{
		0x70, 0x00, 0x05, // key=ILLEGAL REQUEST
		0x00, 0x00, 0x00, 0x00, 0x0A,
		0x00, 0x00, 0x00, 0x00,
		0x20, 0x00, // ASC=0x20 (invalid command operation code)
		0x00, 0x00, 0x00, 0x00,
	}

	err := CheckStatus(0x02, sense)
	if err == nil {
		t.Fatal("expected error for CHECK CONDITION")
	}

	var se *SCSIError
	if !errors.As(err, &se) {
		t.Fatalf("expected *SCSIError, got %T", err)
	}
	if se.Status != 0x02 {
		t.Errorf("Status: got 0x%02X, want 0x02", se.Status)
	}
	if se.SenseKey != 0x05 {
		t.Errorf("SenseKey: got 0x%02X, want 0x05", se.SenseKey)
	}
	if se.ASC != 0x20 {
		t.Errorf("ASC: got 0x%02X, want 0x20", se.ASC)
	}
}

func TestCheckStatusNoSenseData(t *testing.T) {
	err := CheckStatus(0x08, nil) // BUSY, no sense
	if err == nil {
		t.Fatal("expected error for BUSY status")
	}

	var se *SCSIError
	if !errors.As(err, &se) {
		t.Fatalf("expected *SCSIError, got %T", err)
	}
	if se.Status != 0x08 {
		t.Errorf("Status: got 0x%02X, want 0x08", se.Status)
	}
}

func TestCheckStatusUnparseableSense(t *testing.T) {
	err := CheckStatus(0x02, []byte{0x7E}) // bad response code
	if err == nil {
		t.Fatal("expected error")
	}

	var se *SCSIError
	if !errors.As(err, &se) {
		t.Fatalf("expected *SCSIError, got %T", err)
	}
	if se.Message == "" {
		t.Error("expected non-empty Message for unparseable sense")
	}
}
