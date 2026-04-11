package login

import (
	"encoding/hex"
	"testing"
)

// FuzzCHAPChallenge exercises decodeCHAPBinary + validateChallenge without
// panicking on arbitrary input (T-04-09). Seeds cover hex-encoded, base64-encoded,
// short, all-zeros, and invalid prefix inputs.
func FuzzCHAPChallenge(f *testing.F) {
	// Valid 16-byte hex challenge (non-zero entropy)
	validBytes := make([]byte, 16)
	for i := range validBytes {
		validBytes[i] = byte(i + 1)
	}
	f.Add("0x" + hex.EncodeToString(validBytes))

	// Valid 32-byte hex challenge
	validBytes32 := make([]byte, 32)
	for i := range validBytes32 {
		validBytes32[i] = byte(i)
	}
	f.Add("0x" + hex.EncodeToString(validBytes32))

	// Short challenge (2 bytes — too short, fails validateChallenge)
	f.Add("0x0102")

	// All-zeros 16-byte (should fail entropy check in validateChallenge)
	f.Add("0x" + hex.EncodeToString(make([]byte, 16)))

	// Base64 encoded 16 bytes (0–15)
	f.Add("0bAAECAwQFBgcICQoLDA0ODw==")

	// Invalid prefix — no "0x" or "0b"
	f.Add("invalid_no_prefix")

	// Empty string
	f.Add("")

	// Hex with odd number of hex digits (invalid hex)
	f.Add("0xabc")

	// Uppercase prefix
	f.Add("0X" + hex.EncodeToString(validBytes))

	// Very long hex challenge (256 bytes)
	longBytes := make([]byte, 256)
	for i := range longBytes {
		longBytes[i] = byte(i)
	}
	f.Add("0x" + hex.EncodeToString(longBytes))

	f.Fuzz(func(t *testing.T, input string) {
		decoded, err := decodeCHAPBinary(input)
		if err != nil {
			return // invalid encoding is fine
		}
		validateChallenge(decoded) // must not panic regardless of input
	})
}
