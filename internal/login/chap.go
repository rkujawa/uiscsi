// Package login implements iSCSI login phase authentication and negotiation.
package login

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// chapResponse computes the CHAP response per RFC 1994 Section 4.1:
// MD5(id_byte || secret_bytes || challenge_bytes).
// The id is a single byte, not a multi-byte integer.
func chapResponse(id byte, secret, challenge []byte) [16]byte {
	h := md5.New()
	h.Write([]byte{id})
	h.Write(secret)
	h.Write(challenge)
	var digest [16]byte
	copy(digest[:], h.Sum(nil))
	return digest
}

// encodeCHAPBinary encodes binary data as a hex string with "0x" prefix,
// using lowercase hex digits per iSCSI convention.
func encodeCHAPBinary(data []byte) string {
	return "0x" + hex.EncodeToString(data)
}

// decodeCHAPBinary decodes a CHAP binary value. It supports two formats:
//   - "0x" or "0X" prefix: hex-encoded bytes
//   - "0b" or "0B" prefix: base64-encoded bytes
//
// Returns an error for unrecognized prefixes.
func decodeCHAPBinary(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return hex.DecodeString(s[2:])
	}
	if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		return base64.StdEncoding.DecodeString(s[2:])
	}
	return nil, fmt.Errorf("chap: unknown binary encoding prefix in %q", s)
}

// chapState holds the state for a CHAP authentication exchange.
type chapState struct {
	user               string
	secret             string
	mutualSecret       string
	isMutual           bool
	initiatorID        byte
	initiatorChallenge []byte
}

// newCHAPState creates a new CHAP state. If mutual is true, it generates
// a random initiator ID (single byte) and a random 16-byte challenge
// for the target to respond to.
func newCHAPState(user, secret string, mutual bool, mutualSecret string) (*chapState, error) {
	cs := &chapState{
		user:         user,
		secret:       secret,
		mutualSecret: mutualSecret,
		isMutual:     mutual,
	}
	if mutual {
		// Generate random initiator ID (1 byte).
		var idBuf [1]byte
		if _, err := rand.Read(idBuf[:]); err != nil {
			return nil, fmt.Errorf("chap: failed to read random byte for initiator ID: %w", err)
		}
		cs.initiatorID = idBuf[0]

		// Generate random initiator challenge (16 bytes).
		cs.initiatorChallenge = make([]byte, 16)
		if _, err := rand.Read(cs.initiatorChallenge); err != nil {
			return nil, fmt.Errorf("chap: failed to read random bytes for initiator challenge: %w", err)
		}
	}
	return cs, nil
}

// processChallenge handles the target's CHAP challenge (CHAP_A, CHAP_I, CHAP_C)
// and produces the initiator's response keys (CHAP_N, CHAP_R, and optionally
// CHAP_I, CHAP_C for mutual authentication).
func (cs *chapState) processChallenge(keys map[string]string) (map[string]string, error) {
	// Verify algorithm is MD5 (CHAP_A=5).
	algo, ok := keys["CHAP_A"]
	if !ok {
		return nil, fmt.Errorf("chap: missing CHAP_A key")
	}
	if algo != "5" {
		return nil, fmt.Errorf("chap: unsupported algorithm CHAP_A=%s (only MD5/5 is supported)", algo)
	}

	// Parse CHAP_I as decimal integer, take low byte as id.
	idStr, ok := keys["CHAP_I"]
	if !ok {
		return nil, fmt.Errorf("chap: missing CHAP_I key")
	}
	idVal, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, fmt.Errorf("chap: invalid CHAP_I value %q: %w", idStr, err)
	}
	id := byte(idVal)

	// Decode CHAP_C challenge bytes.
	cStr, ok := keys["CHAP_C"]
	if !ok {
		return nil, fmt.Errorf("chap: missing CHAP_C key")
	}
	challenge, err := decodeCHAPBinary(cStr)
	if err != nil {
		return nil, fmt.Errorf("chap: failed to decode CHAP_C: %w", err)
	}

	// Compute response = MD5(id || secret || challenge).
	response := chapResponse(id, []byte(cs.secret), challenge)

	// Build response keys.
	resp := map[string]string{
		"CHAP_N": cs.user,
		"CHAP_R": encodeCHAPBinary(response[:]),
	}

	// For mutual CHAP, include initiator's challenge.
	if cs.isMutual {
		resp["CHAP_I"] = strconv.Itoa(int(cs.initiatorID))
		resp["CHAP_C"] = encodeCHAPBinary(cs.initiatorChallenge)
	}

	return resp, nil
}

// verifyMutualResponse verifies the target's CHAP response during mutual
// authentication. It decodes the target's CHAP_R and compares it against
// the expected response using constant-time comparison.
func (cs *chapState) verifyMutualResponse(keys map[string]string) error {
	rStr, ok := keys["CHAP_R"]
	if !ok {
		return fmt.Errorf("chap: missing CHAP_R in target mutual response")
	}
	targetResp, err := decodeCHAPBinary(rStr)
	if err != nil {
		return fmt.Errorf("chap: failed to decode target CHAP_R: %w", err)
	}

	expected := chapResponse(cs.initiatorID, []byte(cs.mutualSecret), cs.initiatorChallenge)

	if subtle.ConstantTimeCompare(targetResp, expected[:]) != 1 {
		return fmt.Errorf("chap: mutual authentication failed: target response mismatch")
	}

	return nil
}
