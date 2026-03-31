// Package serial implements RFC 1982 serial number arithmetic for iSCSI
// sequence number comparisons.
//
// iSCSI uses 32-bit serial numbers for CmdSN, StatSN, DataSN, and R2TSN.
// These sequence numbers wrap around at 2^32, requiring modular arithmetic
// for correct ordering comparisons. The comparison uses signed 32-bit
// subtraction: if int32(s1-s2) < 0, then s1 is "less than" s2 in the
// serial number space.
//
// Reference: RFC 1982 "Serial Number Arithmetic" Section 3.
package serial

// LessThan reports whether s1 is less than s2 using RFC 1982 serial
// number comparison. It correctly handles wrap-around at the 2^32 boundary.
func LessThan(s1, s2 uint32) bool {
	return s1 != s2 && int32(s1-s2) < 0
}

// GreaterThan reports whether s1 is greater than s2 using RFC 1982 serial
// number comparison. It correctly handles wrap-around at the 2^32 boundary.
func GreaterThan(s1, s2 uint32) bool {
	return s1 != s2 && int32(s1-s2) > 0
}

// InWindow reports whether sn is within the inclusive range [lo, hi] using
// serial number comparison. The window correctly wraps around the 2^32
// boundary.
func InWindow(sn, lo, hi uint32) bool {
	return sn == lo || sn == hi || (GreaterThan(sn, lo) && LessThan(sn, hi))
}

// Incr returns s + 1 mod 2^32. Natural uint32 overflow provides the
// modular arithmetic.
func Incr(s uint32) uint32 {
	return s + 1
}
