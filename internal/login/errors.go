package login

import "fmt"

// AuthReason classifies authentication failure types for structured error
// inspection via errors.As. It allows callers to distinguish CHAP validation
// failures without parsing error message strings (D-07).
type AuthReason uint8

const (
	// ReasonNone indicates no structured auth reason (non-CHAP login failures).
	ReasonNone AuthReason = iota
	// ReasonShortChallenge indicates the CHAP_C challenge is shorter than the
	// minimum 16 bytes required by RFC 7143 best practices (RFC-05).
	ReasonShortChallenge
	// ReasonLowEntropy indicates the CHAP_C challenge has zero entropy
	// (all bytes are 0x00), which is a potential downgrade attack (D-05).
	ReasonLowEntropy
	// ReasonBadResponse indicates the mutual CHAP target response did not
	// match the expected value.
	ReasonBadResponse
	// ReasonUnsupportedAlgorithm indicates the target sent a CHAP_A value
	// other than 5 (MD5), which is the only supported algorithm.
	ReasonUnsupportedAlgorithm
)

// String returns the human-readable name of an AuthReason.
func (r AuthReason) String() string {
	switch r {
	case ReasonNone:
		return "none"
	case ReasonShortChallenge:
		return "short challenge"
	case ReasonLowEntropy:
		return "low entropy challenge"
	case ReasonBadResponse:
		return "bad mutual response"
	case ReasonUnsupportedAlgorithm:
		return "unsupported algorithm"
	default:
		return fmt.Sprintf("unknown reason %d", uint8(r))
	}
}

// LoginError represents an iSCSI login failure with the status class and
// detail from the Login Response PDU (RFC 7143 Section 11.13).
// For CHAP validation failures (which occur before any PDU exchange), the
// Reason field provides a structured failure classification (D-07).
type LoginError struct {
	StatusClass  uint8
	StatusDetail uint8
	Message      string
	// Reason provides a structured classification of CHAP authentication
	// failures. It is ReasonNone for non-CHAP login errors. Use errors.As
	// to retrieve the *LoginError and inspect this field.
	Reason AuthReason
}

func (e *LoginError) Error() string {
	if e.Reason != ReasonNone {
		return fmt.Sprintf("iscsi login: %s: %s", e.Reason, e.Message)
	}
	return fmt.Sprintf("iscsi login: class=%d detail=%d: %s", e.StatusClass, e.StatusDetail, e.Message)
}

// Status constants encoded as uint16 (class<<8 | detail).
// RFC 7143 Section 11.13.5.
const (
	StatusSuccess            uint16 = 0x0000 // Login succeeded
	StatusRedirectTemp       uint16 = 0x0101 // Target moved temporarily
	StatusRedirectPerm       uint16 = 0x0102 // Target moved permanently
	StatusInitiatorError     uint16 = 0x0200 // Initiator error (general)
	StatusAuthFailure        uint16 = 0x0201 // Authentication failure
	StatusForbidden          uint16 = 0x0202 // Authorization failure
	StatusTargetNotFound     uint16 = 0x0203 // Target not found
	StatusTargetRemoved      uint16 = 0x0204 // Target removed
	StatusTargetError        uint16 = 0x0300 // Target error (general)
	StatusServiceUnavailable uint16 = 0x0301 // Service unavailable
	StatusOutOfResources     uint16 = 0x0302 // Out of resources
)

// statusMessage returns a human-readable message for a login status code.
func statusMessage(class, detail uint8) string {
	code := uint16(class)<<8 | uint16(detail)
	switch code {
	case StatusSuccess:
		return "login succeeded"
	case StatusRedirectTemp:
		return "target moved temporarily"
	case StatusRedirectPerm:
		return "target moved permanently"
	case StatusInitiatorError:
		return "initiator error"
	case StatusAuthFailure:
		return "authentication failure"
	case StatusForbidden:
		return "authorization failure"
	case StatusTargetNotFound:
		return "target not found"
	case StatusTargetRemoved:
		return "target removed"
	case StatusTargetError:
		return "target error"
	case StatusServiceUnavailable:
		return "service unavailable"
	case StatusOutOfResources:
		return "out of resources"
	default:
		return fmt.Sprintf("unknown status 0x%04x", code)
	}
}
