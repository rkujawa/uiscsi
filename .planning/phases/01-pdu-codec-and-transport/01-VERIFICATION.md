---
phase: 01-pdu-codec-and-transport
verified: 2026-03-31T00:00:00Z
status: passed
score: 15/15 must-haves verified
re_verification: false
gaps: []
human_verification: []
---

# Phase 1: PDU Codec and Transport Verification Report

**Phase Goal:** A Go application can encode, decode, and frame all iSCSI PDU types over TCP with correct padding, digest computation, and sequence number arithmetic
**Verified:** 2026-03-31
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                             | Status     | Evidence                                                                 |
|----|---------------------------------------------------------------------------------------------------|------------|--------------------------------------------------------------------------|
| 1  | Serial number comparison correctly handles wrap-around at 2^32 boundary                           | VERIFIED   | `serial.go` uses `int32(s1-s2)`; `serial_test.go:14` tests 0xFFFFFFFF wrap |
| 2  | CRC32C digest matches all known test vectors from RFC 3720                                         | VERIFIED   | `crc32c_test.go:18,28,33` asserts 0xe3069283, 0x8a9136aa, 0x62a8ab43   |
| 3  | Padding computation produces correct 0-3 bytes for all input lengths                              | VERIFIED   | `padding.go` uses double-modulo `(4-(n%4))%4`; padding_test.go exercises 0-5, 48, 49, 100 |
| 4  | All 18 iSCSI opcodes round-trip encode and decode with byte-perfect BHS fidelity                  | VERIFIED   | `pdu_test.go:9` has 19 round-trip cases covering all 18 opcode types    |
| 5  | PDU data segments are padded to 4-byte boundaries with zero bytes                                 | VERIFIED   | `pdu.go:EncodePDU` uses `PadLen`; `pdu_test.go:561` TestEncodePDUPadding |
| 6  | AHS segments are correctly encoded and decoded                                                    | VERIFIED   | `ahs.go` implements MarshalAHS/UnmarshalAHS; `ahs_test.go` round-trips single + multi |
| 7  | DataSegmentLength is 24-bit big-endian in bytes 5-7 without corrupting TotalAHSLength in byte 4   | VERIFIED   | `bhs.go` uses manual 3-byte encode; `bhs_test.go:30` 0xAA regression test |
| 8  | Table-driven unit tests cover all PDU types including edge cases                                  | VERIFIED   | `pdu_test.go` 770 lines, 11 test functions; all 18 opcode names in table |
| 9  | PDUs can be framed over TCP: read BHS, parse lengths, read payload in one io.ReadFull call        | VERIFIED   | `framer.go:28` ReadRawPDU uses io.ReadFull exclusively; framer_test.go covers all variants |
| 10 | A single write pump goroutine serializes all PDU writes                                           | VERIFIED   | `pump.go:13` WritePump; `pump_test.go:TestPump_ConcurrentWriters` passes -race |
| 11 | A read pump goroutine dispatches PDUs by ITT                                                      | VERIFIED   | `pump.go:34` ReadPump dispatches via Router; `TestReadPump_BasicDispatch` passes |
| 12 | ITT-based routing delivers response PDUs to the correct waiting goroutine                         | VERIFIED   | `router.go` implements Register/Dispatch/Unregister; router_test.go covers all cases |
| 13 | Concurrent read and write pumps do not corrupt data under -race                                   | VERIFIED   | `go test -race ./internal/...` exits 0; TestPump_FullRoundTrip + TestRouterConcurrent pass |
| 14 | TCP connection supports context cancellation and configurable timeouts                            | VERIFIED   | `conn.go:Dial` uses `net.Dialer{}.DialContext`; `TestConnDial_CancelledContext` passes |
| 15 | Router never allocates reserved ITT 0xFFFFFFFF                                                   | VERIFIED   | `router.go:34` checks for reservedITT; `TestRouterRegister_Skips0xFFFFFFFF` passes |

**Score:** 15/15 truths verified

### Required Artifacts

| Artifact                                   | Expected                                          | Status     | Details                                               |
|--------------------------------------------|---------------------------------------------------|------------|-------------------------------------------------------|
| `go.mod`                                   | Module definition, go 1.25                        | VERIFIED   | module github.com/rkujawa/uiscsi; go 1.25            |
| `internal/serial/serial.go`                | RFC 1982 serial arithmetic                        | VERIFIED   | Exports LessThan, GreaterThan, InWindow, Incr         |
| `internal/serial/serial_test.go`           | Wrap-around tests, 99 lines                       | VERIFIED   | 99 lines; 0xFFFFFFFF cases at lines 14, 38, 85        |
| `internal/digest/crc32c.go`                | CRC32C with Castagnoli polynomial                 | VERIFIED   | Exports HeaderDigest, DataDigest; uses crc32.Castagnoli |
| `internal/digest/crc32c_test.go`           | RFC test vectors, 102 lines                       | VERIFIED   | 102 lines; 3 vector assertions (0xe3069283, 0x8a9136aa, 0x62a8ab43) |
| `internal/pdu/padding.go`                  | 4-byte boundary PadLen helper                     | VERIFIED   | Exports PadLen; uses double-modulo formula            |
| `internal/pdu/opcode.go`                   | All 18 opcode constants                           | VERIFIED   | 18 unique constants; String(), IsInitiator(), IsTarget() |
| `internal/pdu/bhs.go`                      | BHSLength=48, encode/decode helpers               | VERIFIED   | BHSLength=48; manual 3-byte DataSegmentLength encode  |
| `internal/pdu/header.go`                   | PDU interface, Header, DecodeBHS                  | VERIFIED   | PDU interface with Opcode/MarshalBHS/DataSegment; DecodeBHS dispatches all 18 opcodes |
| `internal/pdu/initiator.go`                | 8 initiator types, MarshalBHS/UnmarshalBHS, 297 lines | VERIFIED | 297 lines; all 8 initiator structs implemented       |
| `internal/pdu/target.go`                   | 10 target types, MarshalBHS/UnmarshalBHS, 444 lines | VERIFIED | 444 lines; all 10 target structs implemented         |
| `internal/pdu/ahs.go`                      | AHS struct, MarshalAHS, UnmarshalAHS              | VERIFIED   | Full implementation with padding and error handling   |
| `internal/pdu/pdu.go`                      | EncodePDU top-level function                      | VERIFIED   | EncodePDU assembles BHS + data + padding             |
| `internal/pdu/pdu_test.go`                 | 24+ round-trip tests, 770 lines                   | VERIFIED   | 770 lines; 19 opcode cases + 8 additional edge case test functions |
| `internal/transport/conn.go`               | Conn, Dial with context                           | VERIFIED   | Dial uses net.Dialer{}.DialContext; SetDeadline, Close, NetConn |
| `internal/transport/framer.go`             | ReadRawPDU, WriteRawPDU                           | VERIFIED   | io.ReadFull throughout; single-write WriteRawPDU      |
| `internal/transport/pump.go`               | WritePump, ReadPump goroutines                    | VERIFIED   | WritePump serializes writes; ReadPump dispatches by ITT |
| `internal/transport/router.go`             | Router with Register/Dispatch/Unregister          | VERIFIED   | Skips 0xFFFFFFFF; mutex-protected pending map        |
| `internal/transport/pool.go`               | sync.Pool buffer management                       | VERIFIED   | 4 pools (BHS + 3 size classes); GetBHS/PutBHS/GetBuffer/PutBuffer |
| `internal/transport/framer_test.go`        | Framing tests, net.Pipe, 304 lines                | VERIFIED   | 304 lines; back-to-back, AHS, digest, padding, truncated BHS tests |
| `internal/transport/pump_test.go`          | Concurrent pump tests, 253 lines                  | VERIFIED   | 253 lines; concurrent writers, full round-trip, unsolicited ITT tests |
| `internal/transport/router_test.go`        | ITT routing tests, 125 lines                      | VERIFIED   | 125 lines; 0xFFFFFFFF skip, concurrent register/dispatch tests |

### Key Link Verification

| From                               | To                          | Via                                      | Status  | Details                                         |
|------------------------------------|-----------------------------|------------------------------------------|---------|-------------------------------------------------|
| `internal/digest/crc32c.go`        | `hash/crc32`                | `crc32.MakeTable(crc32.Castagnoli)`      | WIRED   | Line 13 confirmed                               |
| `internal/serial/serial.go`        | RFC 1982                    | `int32(s1-s2)` signed subtraction        | WIRED   | Lines 16, 22 confirmed                          |
| `internal/pdu/pdu.go`              | `internal/pdu/initiator.go` | opcode dispatch in DecodeBHS for OpNOPOut/OpSCSICommand | WIRED | header.go:60,65 dispatch to NOPOut, SCSICommand |
| `internal/pdu/pdu.go`              | `internal/pdu/target.go`    | opcode dispatch for OpSCSIResponse/OpNOPIn | WIRED | header.go:92,97 dispatch to NOPIn, SCSIResponse |
| `internal/pdu/bhs.go`              | `encoding/binary`           | BigEndian for 16/32/64-bit BHS fields    | WIRED   | `binary.BigEndian` confirmed in bhs.go, header.go, initiator.go, target.go |
| `internal/transport/framer.go`     | `internal/pdu`              | `pdu.BHSLength`, `pdu.PadLen`            | WIRED   | framer.go:13 uses pdu.BHSLength; line 49 uses pdu.PadLen |
| `internal/transport/framer.go`     | `io`                        | `io.ReadFull` for exact-length reads     | WIRED   | Lines 33, 61 confirmed; no raw Read calls       |
| `internal/transport/pump.go`       | `internal/transport/router.go` | `router.Dispatch` in ReadPump         | WIRED   | pump.go:68 calls router.Dispatch                |
| `internal/transport/pump.go`       | `internal/transport/framer.go` | `ReadRawPDU`/`WriteRawPDU`            | WIRED   | pump.go:22,43 confirmed                         |
| `internal/transport/pool.go`       | `sync`                      | `sync.Pool` for buffer reuse             | WIRED   | Lines 13, 36, 39, 42 in pool.go confirmed       |

### Data-Flow Trace (Level 4)

Not applicable. All artifacts are pure protocol library code (serialization, framing, routing) with no database or external data sources. There are no rendering components. All data flows through the test harness inputs, and the test suite verifies correctness of encoding/decoding.

### Behavioral Spot-Checks

| Behavior                                       | Command                                               | Result  | Status  |
|------------------------------------------------|-------------------------------------------------------|---------|---------|
| Full internal test suite passes under -race    | `go test -race -count=1 ./internal/...`               | ok (all 4 packages) | PASS |
| Serial arithmetic handles 2^32 wrap            | `go test -race ./internal/serial/ -v` (TestLessThan/wrap max to zero) | PASS | PASS |
| CRC32C RFC test vectors pass                   | `go test -race ./internal/digest/ -v` (TestHeaderDigest) | PASS | PASS |
| Framer back-to-back PDU framing                | `go test -race ./internal/transport/ -run TestFramerReadRawPDU_BackToBack` | PASS | PASS |
| Concurrent pump writers under race detector    | `go test -race ./internal/transport/ -run TestPump_ConcurrentWriters` | PASS | PASS |
| Router skips reserved ITT 0xFFFFFFFF           | `go test -race ./internal/transport/ -run TestRouterRegister_Skips0xFFFFFFFF` | PASS | PASS |
| `go vet ./internal/...`                        | `go vet ./internal/...`                               | no output (clean) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description                                                              | Status    | Evidence                                                                   |
|-------------|-------------|--------------------------------------------------------------------------|-----------|----------------------------------------------------------------------------|
| PDU-01      | 01-02       | Binary PDU encoder/decoder for all iSCSI PDU types (BHS + AHS + data + padding) | SATISFIED | All 18 opcode types in initiator.go + target.go; EncodePDU/DecodeBHS in pdu.go + header.go |
| PDU-02      | 01-01       | RFC 1982 serial number arithmetic for sequence number comparisons         | SATISFIED | internal/serial/serial.go; LessThan/GreaterThan/InWindow/Incr all confirmed |
| PDU-03      | 01-01       | CRC32C (Castagnoli) computation for header and data digests               | SATISFIED | internal/digest/crc32c.go; HeaderDigest/DataDigest pass all RFC test vectors |
| PDU-04      | 01-01, 01-02 | PDU padding to 4-byte boundaries per RFC 7143                            | SATISFIED | PadLen in padding.go; used in pdu.go:EncodePDU and framer.go:WriteRawPDU  |
| XPORT-01    | 01-03       | TCP connection management with configurable timeouts and context cancellation | SATISFIED | internal/transport/conn.go; Dial uses DialContext; SetDeadline confirmed   |
| XPORT-02    | 01-03       | PDU framing over TCP (read full BHS, then AHS + data based on lengths)   | SATISFIED | internal/transport/framer.go; ReadRawPDU uses io.ReadFull in 2-stage read  |
| XPORT-03    | 01-03       | Dedicated read/write goroutine pumps per connection (no concurrent TCP writes) | SATISFIED | internal/transport/pump.go; WritePump serializes all writes to single goroutine |
| XPORT-04    | 01-03       | ITT-based PDU routing/correlation                                         | SATISFIED | internal/transport/router.go; Router.Register/Dispatch/Unregister; ReadPump extracts ITT from BHS[16:20] |
| TEST-03     | 01-02, 01-03 | Table-driven unit tests for PDU encoding/decoding                        | SATISFIED | pdu_test.go (770 lines, 19 opcode round-trip cases); transport test files (682 lines combined) |

No orphaned requirements. All 9 requirement IDs declared across the three plan files are accounted for and satisfied.

### Anti-Patterns Found

No blockers or warnings found.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/pdu/ahs.go` | 33 | `return nil` | Info | Legitimate guard clause: returns nil for empty input, not a stub — MarshalAHS fully implemented below |

No TODO/FIXME/PLACEHOLDER comments found in any production file. No empty handler stubs. No hardcoded empty data returns. All functions have substantive implementations.

### Human Verification Required

None. All behaviors are mechanically verifiable via the test suite and static inspection.

### Gaps Summary

No gaps. All 15 observable truths are verified, all 22 required artifacts exist at all three levels (exists, substantive, wired), all 10 key links are confirmed wired, all 9 requirement IDs are satisfied, no anti-pattern blockers found, and the full test suite passes under `go test -race`.

---

_Verified: 2026-03-31_
_Verifier: Claude (gsd-verifier)_
