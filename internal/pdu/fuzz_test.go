package pdu

import "testing"

func FuzzDecodeBHS(f *testing.F) {
	// Valid opcodes: NOP-Out through Reject.
	f.Add(make([]byte, BHSLength))                                 // all zeros (NOP-Out)
	f.Add(append([]byte{0x01}, make([]byte, BHSLength-1)...))      // SCSI Command
	f.Add(append([]byte{0x20}, make([]byte, BHSLength-1)...))      // NOP-In
	f.Add(append([]byte{0x21}, make([]byte, BHSLength-1)...))      // SCSI Response
	f.Add(append([]byte{0x25}, make([]byte, BHSLength-1)...))      // Data-In
	f.Add(append([]byte{0x31}, make([]byte, BHSLength-1)...))      // R2T
	f.Add(append([]byte{0x3F}, make([]byte, BHSLength-1)...))      // Reject
	f.Add(append([]byte{0x41}, make([]byte, BHSLength-1)...))      // immediate SCSI Command
	f.Add(append([]byte{0xFF}, make([]byte, BHSLength-1)...))      // invalid opcode
	f.Add([]byte{0x21, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) // SCSI Response with Final bit

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) != BHSLength {
			return
		}
		var bhs [BHSLength]byte
		copy(bhs[:], data)
		DecodeBHS(bhs) // must not panic
	})
}

func FuzzUnmarshalAHS(f *testing.F) {
	f.Add([]byte{})     // empty
	f.Add([]byte{0xFF}) // single byte garbage

	// Valid Extended CDB AHS: length=6 (2 data bytes + type + type-specific), type=1
	f.Add([]byte{0x00, 0x06, 0x01, 0x00, 0xAA, 0xBB, 0x00, 0x00}) // padded to 8

	// Two segments: ExtendedCDB + BidiReadDataLen
	f.Add([]byte{
		0x00, 0x06, 0x01, 0x00, 0xAA, 0xBB, 0x00, 0x00, // ExtendedCDB
		0x00, 0x06, 0x02, 0x00, 0x00, 0x01, 0x00, 0x00, // BidiRead
	})

	// Minimum valid: length=2 (just type + type-specific, no data)
	f.Add([]byte{0x00, 0x02, 0x01, 0x00})

	// Large AHS length (but within limit)
	large := make([]byte, 4+100)
	large[0] = 0x00
	large[1] = 0x66 // 102 = 100 data + 2
	large[2] = 0x01
	f.Add(large)

	// Truncated: header says 100 bytes but only 10 present
	f.Add([]byte{0x00, 0x64, 0x01, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06})

	f.Fuzz(func(t *testing.T, data []byte) {
		UnmarshalAHS(data) // must not panic
	})
}
