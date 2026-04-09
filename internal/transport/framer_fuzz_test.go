package transport

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/uiscsi/uiscsi/internal/pdu"
)

func FuzzReadRawPDU(f *testing.F) {
	// Minimal valid PDU: 48-byte BHS, no AHS, no data segment.
	f.Add(make([]byte, pdu.BHSLength), false, false)

	// BHS with small data segment (4 bytes, no padding needed).
	withData := make([]byte, pdu.BHSLength+4)
	withData[5] = 0 // dsLen byte 0
	withData[6] = 0 // dsLen byte 1
	withData[7] = 4 // dsLen byte 2 = 4 bytes
	f.Add(withData, false, false)

	// BHS with data segment requiring padding (3 bytes data + 1 pad).
	withPad := make([]byte, pdu.BHSLength+4) // 3 data + 1 pad
	withPad[7] = 3                           // dsLen = 3
	f.Add(withPad, false, false)

	// BHS with AHS (TotalAHSLength = 1 word = 4 bytes).
	withAHS := make([]byte, pdu.BHSLength+4)
	withAHS[4] = 1 // TotalAHSLength = 1 (4 bytes)
	f.Add(withAHS, false, false)

	// BHS with large dsLen (but data truncated — tests error path).
	bigDS := make([]byte, pdu.BHSLength)
	bigDS[5] = 0x00
	bigDS[6] = 0x10
	bigDS[7] = 0x00 // dsLen = 4096
	f.Add(bigDS, false, false)

	// Various opcodes in BHS.
	for _, op := range []byte{0x00, 0x01, 0x20, 0x21, 0x25, 0x31, 0x3F} {
		b := make([]byte, pdu.BHSLength)
		b[0] = op
		f.Add(b, false, false)
	}

	// With digests enabled.
	f.Add(make([]byte, pdu.BHSLength+4+4), true, false) // header digest
	f.Add(make([]byte, pdu.BHSLength+4), false, true)   // data digest (no data though)
	f.Add(make([]byte, pdu.BHSLength+4+4+4), true, true) // both digests

	f.Fuzz(func(t *testing.T, data []byte, digestHeader, digestData bool) {
		r := bytes.NewReader(data)
		ReadRawPDU(r, digestHeader, digestData, 65536, binary.LittleEndian) // must not panic
	})
}
