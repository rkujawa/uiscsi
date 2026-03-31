package login

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestTextCodecRoundTrip(t *testing.T) {
	pairs := []KeyValue{
		{Key: "HeaderDigest", Value: "None"},
		{Key: "DataDigest", Value: "None"},
		{Key: "MaxConnections", Value: "1"},
	}

	encoded := EncodeTextKV(pairs)
	decoded := DecodeTextKV(encoded)

	if len(decoded) != len(pairs) {
		t.Fatalf("round-trip: got %d pairs, want %d", len(decoded), len(pairs))
	}
	for i, p := range decoded {
		if p.Key != pairs[i].Key || p.Value != pairs[i].Value {
			t.Errorf("pair %d: got %q=%q, want %q=%q", i, p.Key, p.Value, pairs[i].Key, pairs[i].Value)
		}
	}
}

func TestTextCodecByteExact(t *testing.T) {
	pairs := []KeyValue{
		{Key: "key1", Value: "val1"},
		{Key: "key2", Value: "val2"},
	}
	got := EncodeTextKV(pairs)
	want := []byte("key1=val1\x00key2=val2\x00")
	if !bytes.Equal(got, want) {
		t.Errorf("encoded bytes:\n got: %q\nwant: %q", got, want)
	}
}

func TestTextCodecEmpty(t *testing.T) {
	// nil input
	if got := DecodeTextKV(nil); got != nil {
		t.Errorf("DecodeTextKV(nil) = %v, want nil", got)
	}
	// empty slice
	if got := DecodeTextKV([]byte{}); got != nil {
		t.Errorf("DecodeTextKV(empty) = %v, want nil", got)
	}
	// nil pairs
	got := EncodeTextKV(nil)
	if got != nil {
		t.Errorf("EncodeTextKV(nil) = %q, want nil", got)
	}
}

func TestTextCodecTrailingNull(t *testing.T) {
	// Data with trailing null should not produce an extra empty entry
	data := []byte("key=value\x00")
	pairs := DecodeTextKV(data)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1", len(pairs))
	}
	if pairs[0].Key != "key" || pairs[0].Value != "value" {
		t.Errorf("got %q=%q, want key=value", pairs[0].Key, pairs[0].Value)
	}
}

func TestTextCodecEmptyValue(t *testing.T) {
	data := []byte("AuthMethod=\x00")
	pairs := DecodeTextKV(data)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1", len(pairs))
	}
	if pairs[0].Key != "AuthMethod" || pairs[0].Value != "" {
		t.Errorf("got %q=%q, want AuthMethod=<empty>", pairs[0].Key, pairs[0].Value)
	}
}

func TestTextCodecCommaSeparated(t *testing.T) {
	// Comma-separated list values are preserved as-is (not split)
	data := []byte("HeaderDigest=CRC32C,None\x00")
	pairs := DecodeTextKV(data)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1", len(pairs))
	}
	if pairs[0].Value != "CRC32C,None" {
		t.Errorf("got value %q, want CRC32C,None", pairs[0].Value)
	}
}

func TestTextCodecOrderPreserved(t *testing.T) {
	pairs := []KeyValue{
		{Key: "Z", Value: "last"},
		{Key: "A", Value: "first"},
		{Key: "M", Value: "middle"},
	}
	decoded := DecodeTextKV(EncodeTextKV(pairs))
	for i, p := range decoded {
		if p.Key != pairs[i].Key {
			t.Errorf("order broken at %d: got key %q, want %q", i, p.Key, pairs[i].Key)
		}
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"HeaderDigest", d.HeaderDigest, false},
		{"DataDigest", d.DataDigest, false},
		{"MaxConnections", d.MaxConnections, uint32(1)},
		{"InitialR2T", d.InitialR2T, true},
		{"ImmediateData", d.ImmediateData, true},
		{"MaxRecvDataSegmentLength", d.MaxRecvDataSegmentLength, uint32(8192)},
		{"MaxBurstLength", d.MaxBurstLength, uint32(262144)},
		{"FirstBurstLength", d.FirstBurstLength, uint32(65536)},
		{"DefaultTime2Wait", d.DefaultTime2Wait, uint32(2)},
		{"DefaultTime2Retain", d.DefaultTime2Retain, uint32(20)},
		{"MaxOutstandingR2T", d.MaxOutstandingR2T, uint32(1)},
		{"DataPDUInOrder", d.DataPDUInOrder, true},
		{"DataSequenceInOrder", d.DataSequenceInOrder, true},
		{"ErrorRecoveryLevel", d.ErrorRecoveryLevel, uint32(0)},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Defaults().%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoginErrorFormat(t *testing.T) {
	e := &LoginError{
		StatusClass:  2,
		StatusDetail: 1,
		Message:      "authentication failure",
	}

	want := "iscsi login: class=2 detail=1: authentication failure"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestLoginErrorAs(t *testing.T) {
	var target *LoginError
	err := fmt.Errorf("wrapped: %w", &LoginError{StatusClass: 2, StatusDetail: 1, Message: "test"})
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for wrapped LoginError")
	}
	if target.StatusClass != 2 || target.StatusDetail != 1 {
		t.Errorf("unwrapped: class=%d detail=%d, want 2,1", target.StatusClass, target.StatusDetail)
	}
}

func TestStatusConstants(t *testing.T) {
	checks := []struct {
		name string
		val  uint16
		want uint16
	}{
		{"StatusSuccess", StatusSuccess, 0x0000},
		{"StatusRedirectTemp", StatusRedirectTemp, 0x0101},
		{"StatusRedirectPerm", StatusRedirectPerm, 0x0102},
		{"StatusInitiatorError", StatusInitiatorError, 0x0200},
		{"StatusAuthFailure", StatusAuthFailure, 0x0201},
		{"StatusForbidden", StatusForbidden, 0x0202},
		{"StatusTargetNotFound", StatusTargetNotFound, 0x0203},
		{"StatusTargetRemoved", StatusTargetRemoved, 0x0204},
		{"StatusTargetError", StatusTargetError, 0x0300},
		{"StatusServiceUnavailable", StatusServiceUnavailable, 0x0301},
		{"StatusOutOfResources", StatusOutOfResources, 0x0302},
	}

	for _, c := range checks {
		if c.val != c.want {
			t.Errorf("%s = 0x%04x, want 0x%04x", c.name, c.val, c.want)
		}
	}
}
