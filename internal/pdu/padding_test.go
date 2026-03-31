package pdu

import "testing"

func TestPadLen(t *testing.T) {
	tests := []struct {
		name string
		n    uint32
		want uint32
	}{
		{"zero", 0, 0},
		{"one", 1, 3},
		{"two", 2, 2},
		{"three", 3, 1},
		{"four (aligned)", 4, 0},
		{"five", 5, 3},
		{"48 (aligned)", 48, 0},
		{"49", 49, 3},
		{"100 (aligned)", 100, 0},
		{"large aligned", 4096, 0},
		{"large unaligned", 4097, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PadLen(tt.n); got != tt.want {
				t.Errorf("PadLen(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}
