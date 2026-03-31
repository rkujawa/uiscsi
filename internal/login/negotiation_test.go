package login

import (
	"testing"
)

func TestNegotiationBooleanAnd(t *testing.T) {
	def := KeyDef{Name: "ImmediateData", Type: BooleanAnd}
	tests := []struct {
		initiator, target, want string
	}{
		{"Yes", "Yes", "Yes"},
		{"Yes", "No", "No"},
		{"No", "Yes", "No"},
		{"No", "No", "No"},
	}
	for _, tt := range tests {
		t.Run(tt.initiator+"_"+tt.target, func(t *testing.T) {
			got, err := resolveKey(def, tt.initiator, tt.target)
			if err != nil {
				t.Fatalf("resolveKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("BooleanAnd(%s,%s) = %s, want %s", tt.initiator, tt.target, got, tt.want)
			}
		})
	}
}

func TestNegotiationBooleanOr(t *testing.T) {
	def := KeyDef{Name: "InitialR2T", Type: BooleanOr}
	tests := []struct {
		initiator, target, want string
	}{
		{"Yes", "Yes", "Yes"},
		{"Yes", "No", "Yes"},
		{"No", "Yes", "Yes"},
		{"No", "No", "No"},
	}
	for _, tt := range tests {
		t.Run(tt.initiator+"_"+tt.target, func(t *testing.T) {
			got, err := resolveKey(def, tt.initiator, tt.target)
			if err != nil {
				t.Fatalf("resolveKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("BooleanOr(%s,%s) = %s, want %s", tt.initiator, tt.target, got, tt.want)
			}
		})
	}
}

func TestNegotiationNumericalMin(t *testing.T) {
	def := KeyDef{Name: "MaxBurstLength", Type: NumericalMin, Min: 512, Max: 16777215}
	tests := []struct {
		name              string
		initiator, target string
		want              string
	}{
		{"both_equal", "262144", "262144", "262144"},
		{"initiator_smaller", "131072", "262144", "131072"},
		{"target_smaller", "262144", "131072", "131072"},
		{"at_min", "512", "16777215", "512"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKey(def, tt.initiator, tt.target)
			if err != nil {
				t.Fatalf("resolveKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("NumericalMin(%s,%s) = %s, want %s", tt.initiator, tt.target, got, tt.want)
			}
		})
	}
}

func TestNegotiationNumericalMax(t *testing.T) {
	def := KeyDef{Name: "test", Type: NumericalMax, Min: 1, Max: 65535}
	tests := []struct {
		name              string
		initiator, target string
		want              string
	}{
		{"both_equal", "1", "1", "1"},
		{"initiator_larger", "65535", "1", "65535"},
		{"target_larger", "1", "65535", "65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKey(def, tt.initiator, tt.target)
			if err != nil {
				t.Fatalf("resolveKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("NumericalMax(%s,%s) = %s, want %s", tt.initiator, tt.target, got, tt.want)
			}
		})
	}
}

func TestNegotiationListSelect(t *testing.T) {
	def := KeyDef{Name: "HeaderDigest", Type: ListSelect}
	tests := []struct {
		name              string
		initiator, target string
		want              string
		wantErr           bool
	}{
		{"crc32c_from_both", "CRC32C,None", "CRC32C", "CRC32C", false},
		{"none_selected", "CRC32C,None", "None", "None", false},
		{"single_match", "None", "None", "None", false},
		{"no_overlap", "CRC32C", "None", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKey(def, tt.initiator, tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for no overlap, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("ListSelect(%s,%s) = %s, want %s", tt.initiator, tt.target, got, tt.want)
			}
		})
	}
}

func TestNegotiationDeclarative(t *testing.T) {
	def := KeyDef{Name: "MaxRecvDataSegmentLength", Type: Declarative, Min: 512, Max: 16777215}
	// For Declarative, the target's value is what we use (it declares its own)
	got, err := resolveKey(def, "65536", "131072")
	if err != nil {
		t.Fatalf("resolveKey: %v", err)
	}
	if got != "131072" {
		t.Errorf("Declarative: got %s, want 131072 (target's declared value)", got)
	}
}

func TestNegotiationDeclarativeClamped(t *testing.T) {
	def := KeyDef{Name: "MaxRecvDataSegmentLength", Type: Declarative, Min: 512, Max: 16777215}
	// Value below minimum should be clamped
	got, err := resolveKey(def, "65536", "256")
	if err != nil {
		t.Fatalf("resolveKey: %v", err)
	}
	if got != "512" {
		t.Errorf("Declarative clamped: got %s, want 512 (clamped to min)", got)
	}
}

func TestNegotiateFullParams(t *testing.T) {
	// Simulate a full negotiation: initiator proposals + target responses
	targetResponse := []KeyValue{
		{Key: "HeaderDigest", Value: "None"},
		{Key: "DataDigest", Value: "None"},
		{Key: "MaxConnections", Value: "1"},
		{Key: "InitialR2T", Value: "Yes"},
		{Key: "ImmediateData", Value: "Yes"},
		{Key: "MaxRecvDataSegmentLength", Value: "65536"},
		{Key: "MaxBurstLength", Value: "131072"},
		{Key: "FirstBurstLength", Value: "65536"},
		{Key: "DefaultTime2Wait", Value: "2"},
		{Key: "DefaultTime2Retain", Value: "20"},
		{Key: "MaxOutstandingR2T", Value: "1"},
		{Key: "DataPDUInOrder", Value: "Yes"},
		{Key: "DataSequenceInOrder", Value: "Yes"},
		{Key: "ErrorRecoveryLevel", Value: "0"},
	}

	params := Defaults()
	applyNegotiatedKeys(&params, targetResponse)

	if params.HeaderDigest != false {
		t.Error("HeaderDigest should be false")
	}
	if params.MaxRecvDataSegmentLength != 65536 {
		t.Errorf("MaxRecvDataSegmentLength = %d, want 65536", params.MaxRecvDataSegmentLength)
	}
	if params.MaxBurstLength != 131072 {
		t.Errorf("MaxBurstLength = %d, want 131072", params.MaxBurstLength)
	}
	if params.FirstBurstLength != 65536 {
		t.Errorf("FirstBurstLength = %d, want 65536", params.FirstBurstLength)
	}
}

func TestFirstBurstLengthClamping(t *testing.T) {
	// FirstBurstLength must not exceed MaxBurstLength
	keys := []KeyValue{
		{Key: "MaxBurstLength", Value: "65536"},
		{Key: "FirstBurstLength", Value: "131072"},
	}
	params := Defaults()
	applyNegotiatedKeys(&params, keys)

	if params.FirstBurstLength > params.MaxBurstLength {
		t.Errorf("FirstBurstLength (%d) > MaxBurstLength (%d)", params.FirstBurstLength, params.MaxBurstLength)
	}
	if params.FirstBurstLength != 65536 {
		t.Errorf("FirstBurstLength = %d, want 65536 (clamped to MaxBurstLength)", params.FirstBurstLength)
	}
}

func TestKeyRegistryCompleteness(t *testing.T) {
	// Verify all 14 mandatory keys are in the registry
	required := []string{
		"HeaderDigest", "DataDigest", "MaxConnections",
		"InitialR2T", "ImmediateData", "MaxRecvDataSegmentLength",
		"MaxBurstLength", "FirstBurstLength", "DefaultTime2Wait",
		"DefaultTime2Retain", "MaxOutstandingR2T", "DataPDUInOrder",
		"DataSequenceInOrder", "ErrorRecoveryLevel",
	}

	registered := make(map[string]bool)
	for _, kd := range keyRegistry {
		registered[kd.Name] = true
	}

	for _, name := range required {
		if !registered[name] {
			t.Errorf("missing key in registry: %s", name)
		}
	}

	if len(keyRegistry) != 14 {
		t.Errorf("keyRegistry has %d entries, want 14", len(keyRegistry))
	}
}
