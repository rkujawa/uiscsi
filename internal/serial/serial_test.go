package serial

import "testing"

func TestLessThan(t *testing.T) {
	tests := []struct {
		name string
		s1   uint32
		s2   uint32
		want bool
	}{
		{"normal ascending", 1, 2, true},
		{"normal descending", 2, 1, false},
		{"wrap max to zero", 0xFFFFFFFF, 0, true},
		{"wrap near-max to near-min", 0xFFFFFFFE, 1, true},
		{"equal", 5, 5, false},
		{"zero less than large", 0, 0x7FFFFFFE, true},
		{"large not less than zero", 0x7FFFFFFE, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LessThan(tt.s1, tt.s2); got != tt.want {
				t.Errorf("LessThan(0x%08X, 0x%08X) = %v, want %v", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		name string
		s1   uint32
		s2   uint32
		want bool
	}{
		{"normal ascending", 2, 1, true},
		{"normal descending", 1, 2, false},
		{"wrap inverse: zero > max", 0, 0xFFFFFFFF, true},
		{"equal", 5, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GreaterThan(tt.s1, tt.s2); got != tt.want {
				t.Errorf("GreaterThan(0x%08X, 0x%08X) = %v, want %v", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

func TestInWindow(t *testing.T) {
	tests := []struct {
		name string
		sn   uint32
		lo   uint32
		hi   uint32
		want bool
	}{
		{"within window", 5, 3, 7, true},
		{"below window", 2, 3, 7, false},
		{"above window", 8, 3, 7, false},
		{"at lower bound", 3, 3, 7, true},
		{"at upper bound", 7, 3, 7, true},
		{"wrapped window contains value", 1, 0xFFFFFFFE, 2, true},
		{"wrapped window lower bound", 0xFFFFFFFE, 0xFFFFFFFE, 2, true},
		{"wrapped window upper bound", 2, 0xFFFFFFFE, 2, true},
		{"outside wrapped window below", 0xFFFFFFFD, 0xFFFFFFFE, 2, false},
		{"outside wrapped window above", 3, 0xFFFFFFFE, 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InWindow(tt.sn, tt.lo, tt.hi); got != tt.want {
				t.Errorf("InWindow(0x%08X, 0x%08X, 0x%08X) = %v, want %v", tt.sn, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}

func TestIncr(t *testing.T) {
	tests := []struct {
		name string
		s    uint32
		want uint32
	}{
		{"normal", 0, 1},
		{"wrap at max", 0xFFFFFFFF, 0},
		{"mid value", 100, 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Incr(tt.s); got != tt.want {
				t.Errorf("Incr(0x%08X) = 0x%08X, want 0x%08X", tt.s, got, tt.want)
			}
		})
	}
}

// Note: The case where s1 - s2 == 2^31 (exactly half the number space apart)
// is undefined per RFC 1982 Section 3.2. Our implementation using int32 cast
// treats this as s1 < s2 because int32(2^31) is negative (math.MinInt32).
