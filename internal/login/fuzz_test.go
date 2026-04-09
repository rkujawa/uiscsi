package login

import "testing"

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
