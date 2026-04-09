package session

import "testing"

func FuzzParseSendTargetsResponse(f *testing.F) {
	f.Add([]byte{})     // empty
	f.Add([]byte{0xFF}) // garbage

	// Single target, single portal
	f.Add([]byte("TargetName=iqn.2001-04.com.example:storage1\x00TargetAddress=10.0.0.1:3260,1\x00"))

	// Multiple targets, multiple portals
	f.Add([]byte(
		"TargetName=iqn.2001-04.com.example:storage1\x00" +
			"TargetAddress=10.0.0.1:3260,1\x00" +
			"TargetName=iqn.2001-04.com.example:storage2\x00" +
			"TargetAddress=10.0.0.2:3260,1\x00" +
			"TargetAddress=10.0.0.3:3261,2\x00"))

	// TargetAddress without TargetName (orphan)
	f.Add([]byte("TargetAddress=10.0.0.1:3260,1\x00"))

	// IPv6 portal
	f.Add([]byte("TargetName=iqn.example:t1\x00TargetAddress=[2001:db8::1]:3260,1\x00"))

	f.Fuzz(func(t *testing.T, data []byte) {
		parseSendTargetsResponse(data) // must not panic
	})
}

func FuzzParsePortal(f *testing.F) {
	f.Add("")
	f.Add("10.0.0.1:3260,1")                   // IPv4 with port and tpgt
	f.Add("10.0.0.1:3260")                      // no tpgt
	f.Add("10.0.0.1")                           // no port, no tpgt
	f.Add("[2001:db8::1]:3260,1")               // IPv6
	f.Add("[::1]:3260,1")                        // IPv6 loopback
	f.Add("[fe80::1%25eth0]:3260,1")             // IPv6 with zone ID
	f.Add("[malformed")                          // unclosed bracket
	f.Add(":3260,1")                             // no address
	f.Add(",42")                                 // only tpgt
	f.Add("host:notaport,notanumber")            // non-numeric port/tpgt
	f.Add("192.168.1.1:3260,1,2,3")             // extra commas
	f.Add("[2001:db8::1],5")                     // IPv6 without port, with tpgt

	f.Fuzz(func(t *testing.T, s string) {
		parsePortal(s) // must not panic
	})
}
