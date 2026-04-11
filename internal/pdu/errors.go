package pdu

import "fmt"

// ViolationKind classifies the type of iSCSI protocol violation detected.
type ViolationKind uint8

const (
	// BadOpcode indicates an unknown or invalid iSCSI opcode was received.
	BadOpcode ViolationKind = iota + 1
	// OversizedSegment indicates a data segment length exceeds the 24-bit maximum (0xFFFFFF).
	OversizedSegment
	// MalformedBHS indicates a structurally invalid Basic Header Segment.
	MalformedBHS
	// MRDSLExceeded indicates a data segment exceeds the negotiated MaxRecvDataSegmentLength.
	MRDSLExceeded
)

// String returns a human-readable name for the ViolationKind.
func (k ViolationKind) String() string {
	switch k {
	case BadOpcode:
		return "BadOpcode"
	case OversizedSegment:
		return "OversizedSegment"
	case MalformedBHS:
		return "MalformedBHS"
	case MRDSLExceeded:
		return "MRDSLExceeded"
	default:
		return fmt.Sprintf("ViolationKind(%d)", uint8(k))
	}
}

// ProtocolError represents a structured iSCSI protocol violation. It supports
// errors.As matching so callers can inspect the violation details.
type ProtocolError struct {
	// Kind classifies the type of violation.
	Kind ViolationKind
	// Op describes the operation during which the violation was detected
	// (e.g., "decode", "encode", "validate").
	Op string
	// Detail provides a human-readable description of the violation.
	Detail string
	// Opcode is the iSCSI opcode involved, if applicable. Zero for non-opcode errors.
	Opcode OpCode
	// Got is the observed value that triggered the violation (e.g., actual dsLen).
	Got uint32
	// Limit is the maximum allowed value, if applicable (e.g., 0xFFFFFF for 24-bit max).
	Limit uint32
}

// Error implements the error interface.
// Format: "iscsi protocol: {Op}: {Detail}"
func (e *ProtocolError) Error() string {
	return fmt.Sprintf("iscsi protocol: %s: %s", e.Op, e.Detail)
}
