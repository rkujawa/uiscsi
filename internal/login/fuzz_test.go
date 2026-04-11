package login

import (
	"bytes"
	"testing"
)

// FuzzLoginTextCodec exercises EncodeTextKV(DecodeTextKV(input)) round-trip
// without panicking on arbitrary input (T-04-09). Seeds cover common iSCSI
// text key-value patterns.
func FuzzLoginTextCodec(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte("AuthMethod=CHAP\x00"))
	f.Add([]byte("HeaderDigest=CRC32C,None\x00DataDigest=None\x00"))
	f.Add([]byte("MaxRecvDataSegmentLength=262144\x00"))
	f.Add([]byte("InitiatorName=iqn.2024-01.com.example:test\x00"))
	f.Add([]byte("key=val"))                               // no trailing null
	f.Add([]byte("\x00\x00\x00"))                          // all nulls
	f.Add([]byte("=value\x00"))                            // empty key
	f.Add([]byte("noequals\x00"))                          // no equals sign
	f.Add([]byte("MaxBurstLength=262144\x00FirstBurstLength=65536\x00DefaultTime2Wait=2\x00DefaultTime2Retain=20\x00"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode: must not panic.
		pairs := DecodeTextKV(data)
		if pairs == nil {
			return // empty or nil input decoded to nothing — fine
		}
		// Re-encode: must not panic and must produce valid byte sequence.
		encoded := EncodeTextKV(pairs)
		// Re-decode the re-encoded result: must not panic.
		pairs2 := DecodeTextKV(encoded)
		// Verify length is preserved through round-trip.
		if len(pairs2) != len(pairs) {
			t.Fatalf("pair count mismatch after round-trip: got %d, want %d", len(pairs2), len(pairs))
		}
		// Verify each key-value pair is preserved.
		for i, p := range pairs {
			if pairs2[i].Key != p.Key {
				t.Fatalf("key[%d] mismatch: got %q, want %q", i, pairs2[i].Key, p.Key)
			}
			if pairs2[i].Value != p.Value {
				t.Fatalf("value[%d] mismatch: got %q, want %q", i, pairs2[i].Value, p.Value)
			}
		}
		// Re-encoded must consist of null-separated key=value pairs.
		if len(encoded) > 0 && !bytes.HasSuffix(encoded, []byte{0x00}) {
			t.Fatal("encoded output must end with null byte")
		}
	})
}

func FuzzDecodeTextKV(f *testing.F) {
	f.Add([]byte{})                                                        // empty
	f.Add([]byte{0x00})                                                    // single null
	f.Add([]byte("key1=val1\x00key2=val2\x00"))                            // typical
	f.Add([]byte("AuthMethod=\x00"))                                       // empty value
	f.Add([]byte("HeaderDigest=CRC32C,None\x00"))                          // comma list
	f.Add([]byte("SendTargets=All\x00"))                                   // discovery
	f.Add([]byte("=value\x00"))                                            // empty key
	f.Add([]byte("noequals\x00"))                                          // no equals sign
	f.Add([]byte("key=val"))                                               // no trailing null
	f.Add([]byte("\x00\x00\x00"))                                          // all nulls
	f.Add([]byte("a=b\x00c=d\x00e=f\x00g=h\x00i=j\x00"))                  // many pairs
	f.Add([]byte("key=val\x00key=val2\x00"))                               // duplicate key
	f.Add([]byte("MaxRecvDataSegmentLength=262144\x00MaxBurstLength=524288\x00")) // real negotiation

	f.Fuzz(func(t *testing.T, data []byte) {
		DecodeTextKV(data) // must not panic
	})
}
