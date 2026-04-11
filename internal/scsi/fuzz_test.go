package scsi

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/uiscsi/uiscsi/internal/session"
)

// FuzzSenseData exercises the full ParseSense code path including descriptor
// iteration without panicking (T-04-09). Seeds cover fixed-format,
// descriptor-format with Information and Stream Commands descriptors.
func FuzzSenseData(f *testing.F) {
	// Fixed format (0x70) — ILLEGAL REQUEST, Invalid field in CDB
	fixedGood := make([]byte, 18)
	fixedGood[0] = 0x70 // current fixed
	fixedGood[2] = 0x05 // ILLEGAL REQUEST
	fixedGood[7] = 10   // additional sense length
	fixedGood[12] = 0x24 // ASC: Invalid field in CDB
	f.Add(fixedGood)

	// Descriptor format (0x72) with Information descriptor (type 0x00)
	descInfo := []byte{
		0x72, 0x05, 0x24, 0x00, 0x00, 0x00, 0x00, 12, // header: key=5, ASC=0x24, addlLen=12
		0x00, 0x0A, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, // Information descriptor
	}
	f.Add(descInfo)

	// Descriptor format (0x72) with Stream Commands descriptor (type 0x04, filemark)
	descStream := []byte{
		0x72, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 4, // header: NO SENSE, ASC=0x00/ASCQ=0x01
		0x04, 0x02, 0x00, 0x80, // Stream: filemark=true
	}
	f.Add(descStream)

	// Deferred descriptor format (0x73)
	descDeferred := []byte{
		0x73, 0x02, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, // NOT READY, becoming ready
	}
	f.Add(descDeferred)

	// Minimal valid fixed format
	f.Add(make([]byte, 18))
	// Too short for any format
	f.Add([]byte{0x70})
	f.Add([]byte{})
	// Fixed format with valid bit set
	fixedValid := make([]byte, 18)
	fixedValid[0] = 0xF0 // deferred fixed, valid=1
	fixedValid[2] = 0x03 // MEDIUM ERROR
	fixedValid[7] = 10
	f.Add(fixedValid)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseSense(data) // must not panic
	})
}

// goodResult wraps raw bytes as a session.Result with GOOD status.
func fuzzResult(data []byte) session.Result {
	var r *bytes.Reader
	if data != nil {
		r = bytes.NewReader(data)
	}
	return session.Result{
		Status: StatusGood,
		Data:   r,
	}
}

// --- Tier 1: Sense data ---

func FuzzParseSense(f *testing.F) {
	f.Add([]byte{})     // empty
	f.Add([]byte{0xFF}) // single byte garbage

	// Fixed format 0x70 — current, valid
	f.Add([]byte{0xF0, 0x00, 0x03, 0x00, 0x00, 0x01, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x11, 0x00, 0x00, 0x00, 0x00, 0x00})
	// Fixed format 0x70 with filemark+EOM+ILI
	f.Add([]byte{0x70, 0x00, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// Deferred fixed 0x71
	f.Add([]byte{0x71, 0x00, 0x06, 0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x29, 0x00, 0x00, 0x00, 0x00, 0x00})
	// Descriptor format 0x72
	f.Add([]byte{0x72, 0x05, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00})
	// Descriptor deferred 0x73
	f.Add([]byte{0x73, 0x02, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00})
	// Too short for fixed
	f.Add([]byte{0x70, 0x00, 0x03})

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseSense(data) // must not panic
	})
}

// --- Tier 2: Variable-length SCSI responses ---

func FuzzParseVPDDeviceIdentification(f *testing.F) {
	f.Add([]byte{})                     // empty
	f.Add(make([]byte, 4))              // minimal header, zero length
	f.Add([]byte{0, 0x83, 0, 0})       // valid header, no descriptors

	// Single descriptor: ProtocolID=6, CodeSet=1, Type=3, 8-byte identifier
	desc := make([]byte, 4+4+8)
	desc[1] = 0x83
	binary.BigEndian.PutUint16(desc[2:4], 12)
	desc[4] = 0x61 // ProtocolID=6, CodeSet=1
	desc[5] = 0x03 // Type=3
	desc[7] = 0x08 // identifier length
	copy(desc[8:], []byte("IDENT123"))
	f.Add(desc)

	// Two descriptors
	two := make([]byte, 4+4+4+4+4)
	two[1] = 0x83
	binary.BigEndian.PutUint16(two[2:4], 16)
	two[7] = 4                   // first: 4-byte identifier
	copy(two[8:12], []byte("AB12"))
	two[15] = 4                  // second: 4-byte identifier
	copy(two[16:20], []byte("CD34"))
	f.Add(two)

	// Truncated descriptor (length says 100, data is short)
	trunc := []byte{0, 0x83, 0, 8, 0, 0, 0, 100, 0xAA, 0xBB, 0xCC, 0xDD}
	f.Add(trunc)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDDeviceIdentification(fuzzResult(data)) // must not panic
	})
}

func FuzzParseModeSense6(f *testing.F) {
	f.Add([]byte{})                            // empty
	f.Add([]byte{0, 0, 0})                    // too short
	f.Add([]byte{11, 0x00, 0x00, 8, 0x00, 0, 0, 0, 0, 0, 0, 0}) // variable block
	f.Add([]byte{11, 0x00, 0x10, 8, 0x00, 0, 0, 0, 0, 0x01, 0x00, 0x00}) // fixed 65536, buffered
	f.Add([]byte{3, 0x00, 0x00, 0})           // no block descriptor
	f.Add([]byte{0xFF, 0x00, 0x00, 0xFF})     // max bdl, minimal data

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseModeSense6(fuzzResult(data)) // must not panic
	})
}

func FuzzParseModeSense10(f *testing.F) {
	f.Add([]byte{})                   // empty
	f.Add(make([]byte, 7))           // too short
	f.Add(make([]byte, 8))           // minimal valid
	// 16-byte block descriptor
	resp := make([]byte, 24)
	binary.BigEndian.PutUint16(resp[6:8], 16) // bdl=16
	f.Add(resp)
	// Max bdl
	maxBDL := make([]byte, 8)
	binary.BigEndian.PutUint16(maxBDL[6:8], 0xFFFF)
	f.Add(maxBDL)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseModeSense10(fuzzResult(data)) // must not panic
	})
}

func FuzzParseReportLuns(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short

	// 0 LUNs
	zero := make([]byte, 8)
	f.Add(zero)

	// 2 LUNs
	two := make([]byte, 8+16)
	binary.BigEndian.PutUint32(two[0:4], 16) // additional length = 16 bytes = 2 LUNs
	binary.BigEndian.PutUint16(two[8:10], 0)
	binary.BigEndian.PutUint16(two[16:18], 1)
	f.Add(two)

	// Length says 100 LUNs but data is short
	trunc := make([]byte, 16)
	binary.BigEndian.PutUint32(trunc[0:4], 800) // claims 100 LUNs
	f.Add(trunc)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseReportLuns(fuzzResult(data)) // must not panic
	})
}

func FuzzParsePersistReserveInKeys(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short

	// 0 keys
	f.Add(make([]byte, 8))

	// 2 keys
	two := make([]byte, 8+16)
	binary.BigEndian.PutUint32(two[4:8], 16)
	binary.BigEndian.PutUint64(two[8:16], 0xDEADBEEF)
	binary.BigEndian.PutUint64(two[16:24], 0xCAFEBABE)
	f.Add(two)

	// Length exceeds data
	trunc := make([]byte, 12)
	binary.BigEndian.PutUint32(trunc[4:8], 800)
	f.Add(trunc)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParsePersistReserveInKeys(fuzzResult(data)) // must not panic
	})
}

func FuzzParsePersistReserveInReservation(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short
	f.Add(make([]byte, 8)) // no reservation (addl len = 0)

	// Has reservation (24 bytes)
	res := make([]byte, 24)
	binary.BigEndian.PutUint32(res[4:8], 16) // additional length
	binary.BigEndian.PutUint64(res[8:16], 0x1234)
	res[21] = 0x01 // scope/type
	f.Add(res)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParsePersistReserveInReservation(fuzzResult(data)) // must not panic
	})
}

// --- Tier 3: Fixed-format responses ---

func FuzzParseInquiry(f *testing.F) {
	f.Add([]byte{})          // empty
	f.Add(make([]byte, 35)) // too short

	// Standard INQUIRY response (36 bytes)
	inq := make([]byte, 36)
	inq[0] = 0x00 // device type: disk
	copy(inq[8:16], []byte("VENDOR  "))
	copy(inq[16:32], []byte("PRODUCT         "))
	copy(inq[32:36], []byte("1.00"))
	f.Add(inq)

	// Tape device
	inqTape := make([]byte, 36)
	inqTape[0] = 0x01
	f.Add(inqTape)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseInquiry(fuzzResult(data)) // must not panic
	})
}

func FuzzParseReadCapacity10(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short

	cap := make([]byte, 8)
	binary.BigEndian.PutUint32(cap[0:4], 1000000) // LBA
	binary.BigEndian.PutUint32(cap[4:8], 512)     // block size
	f.Add(cap)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseReadCapacity10(fuzzResult(data)) // must not panic
	})
}

func FuzzParseReadCapacity16(f *testing.F) {
	f.Add([]byte{})          // empty
	f.Add(make([]byte, 31)) // too short

	cap := make([]byte, 32)
	binary.BigEndian.PutUint64(cap[0:8], 1000000000)
	binary.BigEndian.PutUint32(cap[8:12], 512)
	f.Add(cap)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseReadCapacity16(fuzzResult(data)) // must not panic
	})
}

func FuzzParseVPDSupportedPages(f *testing.F) {
	f.Add([]byte{})                                           // empty
	f.Add([]byte{0, 0, 0, 3, 0x00, 0x80, 0x83})              // 3 pages
	f.Add([]byte{0, 0, 0, 0})                                 // zero pages
	f.Add([]byte{0, 0, 0, 100})                               // length exceeds data

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDSupportedPages(fuzzResult(data)) // must not panic
	})
}

func FuzzParseVPDSerialNumber(f *testing.F) {
	f.Add([]byte{})                                             // empty
	f.Add([]byte{0, 0x80, 0, 8, 'A', 'B', 'C', '1', '2', '3', '4', '5'}) // normal
	f.Add([]byte{0, 0x80, 0, 0})                               // zero length
	f.Add([]byte{0, 0x80, 0, 100})                              // length exceeds data

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDSerialNumber(fuzzResult(data)) // must not panic
	})
}

func FuzzParseVPDBlockLimits(f *testing.F) {
	f.Add([]byte{})          // empty
	f.Add(make([]byte, 31)) // too short

	bl := make([]byte, 64)
	bl[1] = 0xB0
	binary.BigEndian.PutUint16(bl[2:4], 60)
	binary.BigEndian.PutUint32(bl[8:12], 65536)
	f.Add(bl)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDBlockLimits(fuzzResult(data)) // must not panic
	})
}

func FuzzParseVPDBlockCharacteristics(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short

	bc := make([]byte, 8)
	binary.BigEndian.PutUint16(bc[4:6], 7200) // rotation rate
	bc[7] = 0x02                               // form factor
	f.Add(bc)

	// SSD (non-rotating)
	ssd := make([]byte, 8)
	binary.BigEndian.PutUint16(ssd[4:6], 1)
	f.Add(ssd)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDBlockCharacteristics(fuzzResult(data)) // must not panic
	})
}

func FuzzParseVPDLogicalBlockProvisioning(f *testing.F) {
	f.Add([]byte{})         // empty
	f.Add(make([]byte, 7)) // too short

	lbp := make([]byte, 8)
	lbp[4] = 0x04          // threshold exponent
	lbp[5] = 0xE0          // LBPU + LBPWS + LBPWS10
	lbp[6] = 0x02          // provisioning type
	f.Add(lbp)

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseVPDLogicalBlockProvisioning(fuzzResult(data)) // must not panic
	})
}
