package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"github.com/uiscsi/uiscsi/internal/digest"
	"github.com/uiscsi/uiscsi/internal/pdu"
)

// RawPDU holds the raw bytes of an iSCSI PDU as read from or to be written
// to the wire. The DataSegment is copied into caller-owned memory (not pooled).
type RawPDU struct {
	BHS          [pdu.BHSLength]byte
	AHS          []byte // nil if no AHS
	DataSegment  []byte // copied out, caller-owned
	HeaderDigest uint32 // 0 if not present
	DataDigest   uint32 // 0 if not present
	HasHDigest   bool
	HasDDigest   bool
}

// ReadRawPDU reads a complete iSCSI PDU from r. It uses io.ReadFull exclusively
// to handle partial TCP reads correctly (Pitfall 6). The digest booleans control
// whether header and data digests are expected on the wire.
//
// The returned RawPDU's DataSegment is a freshly allocated slice (caller-owned).
// Pool scratch buffers are used internally and returned after copying.
func ReadRawPDU(r io.Reader, digestHeader, digestData bool, maxRecvDSL uint32, digestByteOrder ...binary.ByteOrder) (*RawPDU, error) {
	byteOrder := binary.ByteOrder(binary.LittleEndian)
	if len(digestByteOrder) > 0 && digestByteOrder[0] != nil {
		byteOrder = digestByteOrder[0]
	}
	// Stage 1: Read exactly 48 bytes BHS.
	bhsBuf := GetBHS()
	defer PutBHS(bhsBuf)

	if _, err := io.ReadFull(r, bhsBuf[:]); err != nil {
		return nil, err
	}

	raw := &RawPDU{}
	raw.BHS = *bhsBuf

	// Stage 2: Parse lengths from BHS.
	ahsLen := uint32(raw.BHS[4]) * 4 // TotalAHSLength is in 4-byte words
	dsLen := uint32(raw.BHS[5])<<16 | uint32(raw.BHS[6])<<8 | uint32(raw.BHS[7])

	// RFC-01: Defense-in-depth guard against impossibly large segment lengths.
	// dsLen is decoded from 3 bytes so it cannot actually exceed 0xFFFFFF, but
	// this guard documents the invariant explicitly.
	if dsLen > 0xFFFFFF {
		return nil, &pdu.ProtocolError{
			Kind:   pdu.OversizedSegment,
			Op:     "decode",
			Detail: fmt.Sprintf("data segment length %d exceeds 24-bit maximum", dsLen),
			Got:    dsLen,
			Limit:  0xFFFFFF,
		}
	}

	// D-08: Log a warning if incoming PDU exceeds negotiated MaxRecvDataSegmentLength,
	// but continue processing for interoperability with non-compliant targets.
	if maxRecvDSL > 0 && dsLen > maxRecvDSL {
		slog.Warn("incoming PDU exceeds negotiated MaxRecvDataSegmentLength",
			"dsLen", dsLen, "maxRecvDSL", maxRecvDSL)
	}

	// Stage 3: Compute total remaining bytes after BHS.
	remaining := ahsLen
	if digestHeader {
		remaining += 4
	}
	padLen := pdu.PadLen(dsLen)
	remaining += dsLen + padLen
	if digestData && dsLen > 0 {
		remaining += 4
	}

	// Stage 4: Read all remaining in one io.ReadFull call.
	if remaining == 0 {
		return raw, nil
	}

	payloadBp := GetBuffer(int(remaining))
	payload := (*payloadBp)[:remaining]
	if _, err := io.ReadFull(r, payload); err != nil {
		PutBuffer(payloadBp)
		return nil, err
	}

	// Stage 5: Slice payload into components.
	off := uint32(0)

	// AHS
	if ahsLen > 0 {
		raw.AHS = make([]byte, ahsLen)
		copy(raw.AHS, payload[off:off+ahsLen])
		off += ahsLen
	}

	// Header digest — byte order configurable (default LittleEndian).
	if digestHeader {
		raw.HeaderDigest = byteOrder.Uint32(payload[off : off+4])
		raw.HasHDigest = true
		off += 4
	}

	// Data segment (copy out to caller-owned memory)
	if dsLen > 0 {
		raw.DataSegment = make([]byte, dsLen)
		copy(raw.DataSegment, payload[off:off+dsLen])
		off += dsLen

		// Skip padding
		off += padLen

		// Data digest — same byte order as header digest.
		if digestData {
			raw.DataDigest = byteOrder.Uint32(payload[off : off+4])
			raw.HasDDigest = true
		}
	}

	// Stage 6: Verify digests before returning.
	if digestHeader {
		var input []byte
		if len(raw.AHS) > 0 {
			input = make([]byte, pdu.BHSLength+len(raw.AHS))
			copy(input, raw.BHS[:])
			copy(input[pdu.BHSLength:], raw.AHS)
		} else {
			input = raw.BHS[:]
		}
		expected := digest.HeaderDigest(input)
		if expected != raw.HeaderDigest {
			PutBuffer(payloadBp)
			return nil, &digest.DigestError{
				Type:     digest.DigestHeader,
				Expected: expected,
				Actual:   raw.HeaderDigest,
			}
		}
	}
	if digestData && dsLen > 0 {
		expected := digest.DataDigest(raw.DataSegment)
		if expected != raw.DataDigest {
			PutBuffer(payloadBp)
			return nil, &digest.DigestError{
				Type:     digest.DigestData,
				Expected: expected,
				Actual:   raw.DataDigest,
			}
		}
	}

	PutBuffer(payloadBp)
	return raw, nil
}

// WriteRawPDU writes a complete iSCSI PDU to w as a single contiguous write.
// This prevents TCP byte interleaving when used with the write pump (Pitfall 7).
func WriteRawPDU(w io.Writer, p *RawPDU, digestByteOrder ...binary.ByteOrder) error {
	byteOrder := binary.ByteOrder(binary.LittleEndian)
	if len(digestByteOrder) > 0 && digestByteOrder[0] != nil {
		byteOrder = digestByteOrder[0]
	}
	dsLen := uint32(len(p.DataSegment))
	padLen := pdu.PadLen(dsLen)

	// Compute total wire size.
	total := pdu.BHSLength + uint32(len(p.AHS))
	if p.HasHDigest {
		total += 4
	}
	total += dsLen + padLen
	if p.HasDDigest && dsLen > 0 {
		total += 4
	}

	bufBp := GetBuffer(int(total))
	defer PutBuffer(bufBp)
	buf := (*bufBp)[:total]

	off := 0

	// BHS
	copy(buf[off:], p.BHS[:])
	off += pdu.BHSLength

	// AHS
	if len(p.AHS) > 0 {
		copy(buf[off:], p.AHS)
		off += len(p.AHS)
	}

	// Header digest — byte order configurable (default LittleEndian).
	if p.HasHDigest {
		byteOrder.PutUint32(buf[off:off+4], p.HeaderDigest)
		off += 4
	}

	// Data segment
	if dsLen > 0 {
		copy(buf[off:], p.DataSegment)
		off += int(dsLen)

		// Zero padding
		for i := 0; i < int(padLen); i++ {
			buf[off+i] = 0
		}
		off += int(padLen)

		// Data digest — same byte order as header digest.
		if p.HasDDigest {
			byteOrder.PutUint32(buf[off:off+4], p.DataDigest)
			off += 4
		}
	}

	_, err := w.Write(buf[:off])
	return err
}

// ValidateOutgoingSegmentLength checks that the outgoing data segment does not
// exceed the target's negotiated MaxRecvDataSegmentLength. Returns a
// *pdu.ProtocolError with Kind=MRDSLExceeded if the segment is too large.
//
// Per D-09: fail fast before sending — outgoing PDUs that would violate the
// target's MRDSL are rejected here, not silently truncated or sent.
// A targetMRDSL of 0 means unlimited (no check performed).
func ValidateOutgoingSegmentLength(dsLen, targetMRDSL uint32) error {
	if targetMRDSL > 0 && dsLen > targetMRDSL {
		return &pdu.ProtocolError{
			Kind:   pdu.MRDSLExceeded,
			Op:     "validate",
			Detail: fmt.Sprintf("outgoing data segment %d exceeds target MaxRecvDataSegmentLength %d", dsLen, targetMRDSL),
			Got:    dsLen,
			Limit:  targetMRDSL,
		}
	}
	return nil
}
