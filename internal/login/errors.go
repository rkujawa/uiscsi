package login

import "fmt"

// LoginError represents an iSCSI login failure with the status class and
// detail from the Login Response PDU (RFC 7143 Section 11.13).
type LoginError struct {
	StatusClass  uint8
	StatusDetail uint8
	Message      string
}

func (e *LoginError) Error() string {
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
