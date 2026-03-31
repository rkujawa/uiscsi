// Package login implements the iSCSI login phase protocol including text
// key-value encoding, parameter negotiation, and error handling per RFC 7143.
package login

import (
	"bytes"
	"strings"
)

// KeyValue represents a single iSCSI text key-value pair.
// Keys and values are UTF-8 strings separated by "=" on the wire,
// with pairs delimited by null bytes (0x00).
type KeyValue struct {
	Key   string
	Value string
}

// EncodeTextKV encodes key-value pairs into the iSCSI text format:
// "key1=val1\x00key2=val2\x00". Order is preserved (deterministic).
// Returns nil for nil or empty input.
func EncodeTextKV(pairs []KeyValue) []byte {
	if len(pairs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, p := range pairs {
		buf.WriteString(p.Key)
		buf.WriteByte('=')
		buf.WriteString(p.Value)
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

// DecodeTextKV decodes iSCSI text format data into key-value pairs.
// Handles trailing null bytes, empty values ("key=\x00"), and preserves
// comma-separated list values as-is. Returns nil for nil or empty input.
func DecodeTextKV(data []byte) []KeyValue {
	if len(data) == 0 {
		return nil
	}
	// Split on null bytes
	segments := bytes.Split(data, []byte{0})
	var pairs []KeyValue
	for _, seg := range segments {
		if len(seg) == 0 {
			continue // skip empty segments (trailing null)
		}
		key, value, _ := strings.Cut(string(seg), "=")
		pairs = append(pairs, KeyValue{Key: key, Value: value})
	}
	if len(pairs) == 0 {
		return nil
	}
	return pairs
}
