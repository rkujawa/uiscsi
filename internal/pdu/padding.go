// Package pdu provides iSCSI PDU encoding, decoding, and helpers.
package pdu

// PadLen returns the number of zero-padding bytes needed to align n to a
// 4-byte boundary, as required by RFC 7143 for PDU data segments and AHS.
// The result is always in the range [0, 3].
//
// The double-modulo formula (4 - (n % 4)) % 4 is used rather than
// (4 - (n % 4)) to correctly return 0 when n is already aligned.
func PadLen(n uint32) uint32 {
	return (4 - (n % 4)) % 4
}
