package login

import (
	"fmt"
	"strconv"
	"strings"
)

// NegotiationType defines how a key is resolved during iSCSI login
// negotiation per RFC 7143 Section 5.3.
type NegotiationType int

const (
	// ListSelect: initiator offers a list, target picks one.
	ListSelect NegotiationType = iota
	// BooleanAnd: result is "Yes" only if both sides say "Yes".
	BooleanAnd
	// BooleanOr: result is "Yes" if either side says "Yes".
	BooleanOr
	// NumericalMin: result is the smaller of the two values.
	NumericalMin
	// NumericalMax: result is the larger of the two values.
	NumericalMax
	// Declarative: each side independently declares its value.
	// The target's declared value is what the initiator uses.
	Declarative
)

// KeyDef describes a single negotiable iSCSI operational parameter.
type KeyDef struct {
	Name    string
	Type    NegotiationType
	Default string
	Min     uint32
	Max     uint32
	RFCRef  string
}

// keyRegistry contains the 14 mandatory operational parameter keys
// from RFC 7143 Section 13.
var keyRegistry = []KeyDef{
	{Name: "HeaderDigest", Type: ListSelect, Default: "None", RFCRef: "RFC 7143 Section 13.1"},
	{Name: "DataDigest", Type: ListSelect, Default: "None", RFCRef: "RFC 7143 Section 13.2"},
	{Name: "MaxConnections", Type: NumericalMin, Default: "1", Min: 1, Max: 65535, RFCRef: "RFC 7143 Section 13.3"},
	{Name: "InitialR2T", Type: BooleanOr, Default: "Yes", RFCRef: "RFC 7143 Section 13.10"},
	{Name: "ImmediateData", Type: BooleanAnd, Default: "Yes", RFCRef: "RFC 7143 Section 13.11"},
	{Name: "MaxRecvDataSegmentLength", Type: Declarative, Default: "8192", Min: 512, Max: 16777215, RFCRef: "RFC 7143 Section 13.12"},
	{Name: "MaxBurstLength", Type: NumericalMin, Default: "262144", Min: 512, Max: 16777215, RFCRef: "RFC 7143 Section 13.13"},
	{Name: "FirstBurstLength", Type: NumericalMin, Default: "65536", Min: 512, Max: 16777215, RFCRef: "RFC 7143 Section 13.14"},
	{Name: "DefaultTime2Wait", Type: NumericalMin, Default: "2", Min: 0, Max: 3600, RFCRef: "RFC 7143 Section 13.15"},
	{Name: "DefaultTime2Retain", Type: NumericalMin, Default: "20", Min: 0, Max: 3600, RFCRef: "RFC 7143 Section 13.16"},
	{Name: "MaxOutstandingR2T", Type: NumericalMin, Default: "1", Min: 1, Max: 65535, RFCRef: "RFC 7143 Section 13.17"},
	{Name: "DataPDUInOrder", Type: BooleanOr, Default: "Yes", RFCRef: "RFC 7143 Section 13.18"},
	{Name: "DataSequenceInOrder", Type: BooleanOr, Default: "Yes", RFCRef: "RFC 7143 Section 13.19"},
	{Name: "ErrorRecoveryLevel", Type: NumericalMin, Default: "0", Min: 0, Max: 2, RFCRef: "RFC 7143 Section 13.20"},
}

// resolveKey applies the negotiation algorithm for a given key definition,
// using the initiator's proposed value and the target's response value.
func resolveKey(def KeyDef, initiatorValue, targetValue string) (string, error) {
	switch def.Type {
	case BooleanAnd:
		if initiatorValue == "Yes" && targetValue == "Yes" {
			return "Yes", nil
		}
		return "No", nil

	case BooleanOr:
		if initiatorValue == "Yes" || targetValue == "Yes" {
			return "Yes", nil
		}
		return "No", nil

	case NumericalMin:
		iv, err := strconv.ParseUint(initiatorValue, 10, 32)
		if err != nil {
			return "", fmt.Errorf("negotiation %s: invalid initiator value %q: %w", def.Name, initiatorValue, err)
		}
		tv, err := strconv.ParseUint(targetValue, 10, 32)
		if err != nil {
			return "", fmt.Errorf("negotiation %s: invalid target value %q: %w", def.Name, targetValue, err)
		}
		result := iv
		if tv < iv {
			result = tv
		}
		result = clampValue(result, def)
		return strconv.FormatUint(result, 10), nil

	case NumericalMax:
		iv, err := strconv.ParseUint(initiatorValue, 10, 32)
		if err != nil {
			return "", fmt.Errorf("negotiation %s: invalid initiator value %q: %w", def.Name, initiatorValue, err)
		}
		tv, err := strconv.ParseUint(targetValue, 10, 32)
		if err != nil {
			return "", fmt.Errorf("negotiation %s: invalid target value %q: %w", def.Name, targetValue, err)
		}
		result := iv
		if tv > iv {
			result = tv
		}
		result = clampValue(result, def)
		return strconv.FormatUint(result, 10), nil

	case ListSelect:
		// Target picks from initiator's list. Find first match in
		// target's selection against initiator's offered list.
		initiatorItems := strings.Split(initiatorValue, ",")
		targetItems := strings.Split(targetValue, ",")
		for _, ti := range targetItems {
			ti = strings.TrimSpace(ti)
			for _, ii := range initiatorItems {
				ii = strings.TrimSpace(ii)
				if ii == ti {
					return ti, nil
				}
			}
		}
		return "", fmt.Errorf("negotiation %s: no overlap between %q and %q", def.Name, initiatorValue, targetValue)

	case Declarative:
		// Each side declares independently; we use the target's value
		// (it tells us what it supports).
		tv, err := strconv.ParseUint(targetValue, 10, 32)
		if err != nil {
			return "", fmt.Errorf("negotiation %s: invalid target value %q: %w", def.Name, targetValue, err)
		}
		tv = clampValue(tv, def)
		return strconv.FormatUint(tv, 10), nil

	default:
		return "", fmt.Errorf("negotiation %s: unknown type %d", def.Name, def.Type)
	}
}

// clampValue clamps a numeric value to the key's valid range [Min, Max].
// If Min and Max are both 0, no clamping is applied.
func clampValue(v uint64, def KeyDef) uint64 {
	if def.Min == 0 && def.Max == 0 {
		return v
	}
	if v < uint64(def.Min) {
		return uint64(def.Min)
	}
	if def.Max > 0 && v > uint64(def.Max) {
		return uint64(def.Max)
	}
	return v
}

// applyNegotiatedKeys takes resolved key-value pairs and sets the
// corresponding fields on NegotiatedParams. Parses "Yes"/"No" to bool,
// numeric strings to uint32. Applies post-negotiation validation
// (FirstBurstLength <= MaxBurstLength).
func applyNegotiatedKeys(params *NegotiatedParams, keys []KeyValue) {
	for _, kv := range keys {
		switch kv.Key {
		case "HeaderDigest":
			params.HeaderDigest = kv.Value == "CRC32C"
		case "DataDigest":
			params.DataDigest = kv.Value == "CRC32C"
		case "MaxConnections":
			params.MaxConnections = parseUint32(kv.Value)
		case "InitialR2T":
			params.InitialR2T = kv.Value == "Yes"
		case "ImmediateData":
			params.ImmediateData = kv.Value == "Yes"
		case "MaxRecvDataSegmentLength":
			params.MaxRecvDataSegmentLength = parseUint32(kv.Value)
		case "MaxBurstLength":
			params.MaxBurstLength = parseUint32(kv.Value)
		case "FirstBurstLength":
			params.FirstBurstLength = parseUint32(kv.Value)
		case "DefaultTime2Wait":
			params.DefaultTime2Wait = parseUint32(kv.Value)
		case "DefaultTime2Retain":
			params.DefaultTime2Retain = parseUint32(kv.Value)
		case "MaxOutstandingR2T":
			params.MaxOutstandingR2T = parseUint32(kv.Value)
		case "DataPDUInOrder":
			params.DataPDUInOrder = kv.Value == "Yes"
		case "DataSequenceInOrder":
			params.DataSequenceInOrder = kv.Value == "Yes"
		case "ErrorRecoveryLevel":
			params.ErrorRecoveryLevel = parseUint32(kv.Value)
		}
	}

	// Post-negotiation validation: FirstBurstLength must not exceed MaxBurstLength
	if params.FirstBurstLength > params.MaxBurstLength {
		params.FirstBurstLength = params.MaxBurstLength
	}
}

// parseUint32 parses a decimal string to uint32, returning 0 on error.
func parseUint32(s string) uint32 {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(v)
}
