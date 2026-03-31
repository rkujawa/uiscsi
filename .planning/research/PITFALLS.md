# Pitfalls Research

**Domain:** Pure-userspace iSCSI initiator library (RFC 7143) in Go
**Researched:** 2026-03-31
**Confidence:** HIGH (RFC spec + documented implementation bugs from UNH-IOL testing + real-world interop reports)

## Critical Pitfalls

### Pitfall 1: Command Sequence Number Arithmetic Done Wrong

**What goes wrong:**
CmdSN, ExpCmdSN, MaxCmdSN, and StatSN are 32-bit unsigned integers that use modular arithmetic (mod 2^32) with serial number comparison per RFC 1982. Implementations that use simple `<` / `>` comparisons instead of serial arithmetic break when sequence numbers wrap around 2^32. The command window (ExpCmdSN to MaxCmdSN) must be validated using serial arithmetic, not naive integer comparison. Additionally, immediate commands carry a CmdSN that may lie outside the ExpCmdSN-to-MaxCmdSN range -- this is legal and must not be rejected.

**Why it happens:**
Developers implement sequence number comparison as plain `uint32` comparison. This works for millions of commands but silently breaks at wrap-around. The wrap edge case is never hit in testing unless explicitly provoked. RFC 7143 Section 3.2.2.1 specifies serial arithmetic but it is easy to overlook.

**How to avoid:**
- Implement a dedicated `serialCmp(a, b uint32) int` function that performs RFC 1982 serial number arithmetic from day one.
- Use this function for ALL sequence number comparisons -- CmdSN window checks, StatSN tracking, ExpStatSN updates.
- Write explicit unit tests for wrap-around: CmdSN at 0xFFFFFFF0 wrapping through 0x00000010.
- Never use `>` or `<` directly on sequence numbers anywhere in the codebase.

**Warning signs:**
- Any use of `>`, `<`, `>=`, `<=` on raw uint32 sequence number variables without a wrapper function.
- Tests that only exercise low sequence number values.
- Session failures after extended operation (hours/days of sustained I/O).

**Phase to address:**
Phase 1 (PDU layer / core types). The serial arithmetic helper must exist before any PDU processing code is written.

---

### Pitfall 2: Incorrect R2T and Data-Out Sequencing

**What goes wrong:**
Write operations involve complex multi-PDU exchanges: the initiator sends a SCSI Command (possibly with immediate data up to FirstBurstLength), then the target sends R2T (Ready to Transfer) PDUs requesting specific data ranges, and the initiator responds with Data-Out PDUs. Getting the sequencing wrong causes data corruption or target disconnection. Specific failure modes documented in UNH-IOL testing:
- Sending more unsolicited data than FirstBurstLength (RFC 7143 Section 3.2.4.2 says this is an error).
- Not respecting MaxBurstLength in Data-Out sequences responding to a single R2T.
- Incorrect DataSN sequencing within a Data-Out sequence for a single R2T.
- Sending unsolicited data when InitialR2T=Yes and ImmediateData=No.
- Integer overflow in R2T offset/length calculations leading to buffer overruns (CVE in EDK2 iSCSI driver: GHSA-8522-69fh-w74x).

**Why it happens:**
The interaction between FirstBurstLength, MaxBurstLength, MaxRecvDataSegmentLength, ImmediateData, and InitialR2T creates a combinatorial matrix of behaviors. Developers implement the happy path (ImmediateData=Yes, InitialR2T=No with moderate burst lengths) and miss edge cases.

**How to avoid:**
- Build a truth table for all combinations of ImmediateData (Yes/No) x InitialR2T (Yes/No) and implement each path explicitly.
- Validate that total unsolicited data never exceeds FirstBurstLength.
- Validate that each Data-Out sequence responding to an R2T never exceeds MaxBurstLength.
- Validate that each Data-Out PDU data segment never exceeds MaxRecvDataSegmentLength (target's declared value).
- Check for integer overflow on R2T Desired Data Transfer Length + Buffer Offset before computing buffer slices.
- Write parameterized tests covering: zero-length writes, writes exactly at burst boundaries, writes that require multiple R2Ts, writes where MaxRecvDataSegmentLength forces PDU splitting within a burst.

**Warning signs:**
- Write tests only use small payloads that fit in a single PDU.
- No test coverage for InitialR2T=Yes configurations.
- R2T response code calculates buffer offsets without overflow checks.

**Phase to address:**
Phase covering full feature phase / SCSI command transport. Must be addressed before any write path is considered functional.

---

### Pitfall 3: Text Negotiation Key-Value Parsing Fragility

**What goes wrong:**
iSCSI login and text negotiation use a key=value text format in the data segment, but the implementation details are treacherous:
- Keys can be split across multiple PDUs (C bit continuation). A target sending a text response with C=1 means "more data follows" -- the initiator must buffer and reassemble before parsing.
- The null byte (0x00) separates key=value pairs, not newlines.
- Keys are case-sensitive per RFC 7143 Section 6.1.
- Unknown keys must be responded to with `key=NotUnderstood`, not silently dropped.
- Some keys allow multiple values (e.g., AuthMethod can list "CHAP,None"). The comma-separated parsing must handle whitespace correctly.
- MaxRecvDataSegmentLength is per-direction and per-connection -- it is NOT a session-wide parameter. Confusing the initiator's declared value with the target's declared value causes PDU size violations.
- Renegotiation of keys during login that do not allow renegotiation must be detected and rejected (RFC 7143 Section 6.3).

UNH-IOL testing of Microsoft's iSCSI Target Server found it disconnecting sessions when receiving split text requests (C bit set) with MaxRecvDataSegmentLength keys, and when receiving unknown keys without C bit. Real targets have these bugs -- the initiator must be robust.

**Why it happens:**
Developers treat negotiation as a simple string split exercise. The continuation mechanism, per-key semantics (declarative vs. negotiated, boolean vs. numerical range vs. list), and error handling for malformed responses all add complexity that is easy to underestimate.

**How to avoid:**
- Implement a proper negotiation state machine that tracks: pending keys (sent but not responded), agreed values, and negotiation phase (SecurityNegotiation vs. OperationalNegotiation).
- Handle C bit continuation by buffering partial data segments until F bit is set.
- Implement per-key type handlers: boolean keys use "Yes"/"No" with OR/AND semantics, numerical keys use min/max range negotiation, list keys use intersection of offered values.
- Store MaxRecvDataSegmentLength separately for initiator-to-target and target-to-initiator directions.
- Build a test corpus of malformed/edge-case negotiation responses: empty values, unknown keys, split keys, duplicate keys, keys with trailing whitespace.

**Warning signs:**
- Negotiation code that splits on "=" and "\\0" without handling continuation.
- A single MaxRecvDataSegmentLength variable shared for both directions.
- No handling of `NotUnderstood` responses from target.
- Tests only negotiate with a cooperative target that sends all keys in one PDU.

**Phase to address:**
Phase covering login negotiation. This is foundational -- every subsequent feature depends on correct negotiation.

---

### Pitfall 4: Login Phase State Machine Shortcuts

**What goes wrong:**
The iSCSI login is not a simple request-response. It is a multi-step state machine with two Connection State Groups (CSG): SecurityNegotiation (CSG=0) and LoginOperationalNegotiation (CSG=1). The initiator proposes a CSG and a Next Stage Group (NSG) via T bit (transit) and NSG field. Failures documented in real implementations:
- Skipping SecurityNegotiation when the target requires authentication (Red Hat bug 1624678: initiator requests CSG:1 without completing authentication).
- Not handling Login Redirect responses (status class 0x01) which require reconnecting to a different address.
- Not handling partial login responses (T=0, meaning target wants more negotiation before transitioning).
- Sending login PDUs with incorrect ISID format -- the ISID has a structured format (type qualifier + qualifier fields) not just an arbitrary 6-byte value.

**Why it happens:**
Developers implement the happy path: send login request with CSG=0/NSG=1/T=1 (transit from security to operational in one step), then CSG=1/NSG=3/T=1 (transit from operational to full feature). This works with permissive targets but fails when targets require multi-step negotiation or enforce authentication.

**How to avoid:**
- Implement login as an explicit state machine with states: SECURITY_NEGOTIATION, OPERATIONAL_NEGOTIATION, FULL_FEATURE_PHASE, and transitions driven by the target's response (T bit, NSG, status class/detail).
- Handle all status classes: 0x00 (success), 0x01 (redirect), 0x02 (initiator error), 0x03 (target error).
- Support multi-PDU login exchanges within each CSG (T=0 responses from target).
- Validate ISID structure per RFC 7143 Section 11.12.5.

**Warning signs:**
- Login code that sends exactly two PDUs and expects exactly two responses.
- No handling for status class 0x01 (redirect).
- No loop for T=0 responses within a CSG.
- ISID generated as random bytes without respecting the type field structure.

**Phase to address:**
Login phase implementation. Must be rock-solid before any full feature phase work begins.

---

### Pitfall 5: CRC32C Digest Computation With Wrong Bit Ordering or Scope

**What goes wrong:**
iSCSI uses CRC32C (Castagnoli) for header and data digests, not the more common CRC32 (IEEE). Specific errors:
- Using CRC32 (polynomial 0x04C11DB7) instead of CRC32C (polynomial 0x1EDC6F41). Go's `hash/crc32` package provides both via `crc32.MakeTable(crc32.Castagnoli)` -- using `crc32.IEEETable` is wrong.
- Computing the digest over the wrong bytes. Header digest covers the 48-byte BHS (Basic Header Segment) plus any AHS (Additional Header Segments), but NOT the header digest field itself and NOT the data segment. Data digest covers only the data segment, padded to 4-byte boundary with zeros.
- Not padding the data segment to a 4-byte boundary before computing the data digest.
- Byte order of the digest in the PDU: the 32-bit CRC is stored in the PDU in network byte order (big-endian) per RFC 7143.
- Negotiation mismatch: digest negotiation uses "CRC32C" or "None" (note the exact string). Both HeaderDigest and DataDigest are negotiated independently and can have different values.

**Why it happens:**
CRC32C is easily confused with CRC32. The padding requirement for data digests is buried in the spec. The scope of what bytes are included in each digest requires careful reading of RFC 7143 Section 11.

**How to avoid:**
- Use `crc32.MakeTable(crc32.Castagnoli)` explicitly and name it clearly (e.g., `crc32cTable`).
- Write a test that computes the digest of a known PDU from the RFC examples or from a packet capture against a known-good target (e.g., LIO).
- Implement digest computation as a single function that takes the exact byte slice it should cover -- do not scatter digest logic across PDU serialization code.
- Pad data segments to 4-byte boundary before digest computation. Assert that padding bytes are zero.
- Store the negotiated digest mode (None vs. CRC32C) per-connection, not per-session.

**Warning signs:**
- Import of `crc32.IEEETable` anywhere in the codebase.
- Digest tests that only verify against self-computed values (circular validation).
- Data digest code that does not handle the 4-byte padding.
- Digest negotiation result stored as a session-level variable.

**Phase to address:**
PDU serialization/deserialization phase. Digest support should be implemented alongside PDU encoding, even if negotiation defaults to None initially.

---

### Pitfall 6: Error Recovery Level Complexity Underestimation

**What goes wrong:**
RFC 7143 defines three error recovery levels, and each level subsumes the previous:
- **ERL 0 (session recovery):** On any error, tear down the entire session and re-establish. Sounds simple but requires correct cleanup of all pending tasks, proper ISID reuse for session reinstatement, and awareness that the target may have partially completed commands.
- **ERL 1 (digest error recovery / within-connection):** On a digest error, request retransmission via SNACK. Requires maintaining enough state to identify which PDU failed, issue a SNACK with correct BegRun/RunLength, and handle the retransmitted PDUs. The initiator must also handle the case where the target rejects SNACK.
- **ERL 2 (connection recovery):** On a connection failure within a multi-connection session, re-establish the failed connection and reassign pending tasks. For single-connection sessions (this project's v1 scope), ERL 2 degrades to session recovery but the negotiation and state machine still need to handle it.

The critical mistake is implementing ERL 1 and ERL 2 without a solid ERL 0 foundation. ERL 0 is where most of the real-world error recovery happens.

**Why it happens:**
ERL 0 seems too simple (just reconnect), so developers rush to ERL 1/2. But ERL 0 session reinstatement has subtle requirements: the initiator MUST use the same ISID, the target uses the ISID to identify the old session and clean it up, pending commands are lost and must be re-issued by the SCSI layer, and the command numbering restarts.

**How to avoid:**
- Implement ERL 0 first and make it bulletproof: clean session teardown, proper ISID management for reinstatement, notification to callers that pending commands were lost.
- For ERL 1, implement SNACK only after full feature phase works correctly without it. SNACK is a retransmission request mechanism -- it requires tracking which StatSN/DataSN values have been received.
- For ERL 2, since the project is single-connection-per-session, ERL 2 negotiation can be supported but actual recovery degrades to ERL 0. Document this clearly.
- Test error recovery by injecting faults: drop TCP connections mid-transfer, corrupt digests, delay responses past timeout.

**Warning signs:**
- Error recovery code that does not track pending commands for retry/notification after session reinstatement.
- No ISID reuse logic for session reinstatement.
- SNACK implementation without StatSN/DataSN gap tracking.
- No fault injection in test suite.

**Phase to address:**
Error recovery should be phased: ERL 0 in the phase immediately after basic full feature phase works. ERL 1 and ERL 2 in a subsequent phase. Do not attempt all three simultaneously.

---

### Pitfall 7: Initiator Task Tag (ITT) Reuse Collision

**What goes wrong:**
Every outstanding SCSI command and task management function gets a unique 32-bit Initiator Task Tag (ITT). The ITT is how the initiator correlates responses to requests. If an ITT is reused while a previous command with that tag is still outstanding (even in error recovery), the initiator will misroute the response. This causes silent data corruption (response for command A delivered to waiter for command B) or panics from unexpected PDU types.

**Why it happens:**
Simple ITT allocation (incrementing counter) works until wrap-around or until error recovery creates "zombie" commands that are still tracked by the target but the initiator has forgotten about. After session reinstatement (ERL 0), the target may still send responses for old commands with old ITTs if cleanup races occur.

**How to avoid:**
- Use a 32-bit allocator that tracks outstanding ITTs and never reuses one until the previous command with that tag has been fully resolved (response received or session reset).
- After session reinstatement, clear the ITT allocation map completely -- the new session has no outstanding commands.
- Reserve ITT 0xFFFFFFFF as the "unassigned" / "reserved" value per RFC 7143.
- Consider using a generation counter in upper bits of ITT to make collisions detectable (e.g., upper 8 bits = session generation, lower 24 bits = command index).

**Warning signs:**
- ITT allocated as a simple incrementing `uint32` with no tracking of outstanding tags.
- No special handling of ITT 0xFFFFFFFF.
- No ITT cleanup on session reinstatement.

**Phase to address:**
Core types / session management phase -- ITT allocation is needed as soon as commands are sent.

---

### Pitfall 8: Task Management Response Misinterpretation

**What goes wrong:**
Task management functions (ABORT TASK, LUN RESET, etc.) have specific response codes that require different handling:
- Response 0 (Function Complete): task was aborted/reset successfully.
- Response 1 (Task Does Not Exist): the referenced task was not found -- initiator must not treat this as a fatal error (the task may have already completed).
- Response 3 (Task Still Allegiant): task is allegiant to another connection -- relevant for multi-connection sessions.
- Response 5 (Function Rejected): target refused the operation.

Real targets (documented in UNH-IOL testing of Windows iSCSI Target) return incorrect response codes: "Function Complete" when the task does not exist. The initiator must be defensive.

**Why it happens:**
Developers treat task management as fire-and-forget or only check for response 0 vs. "everything else is an error." The nuanced response codes are missed.

**How to avoid:**
- Handle all five response codes explicitly in a switch statement.
- Treat "Task Does Not Exist" as a non-error for ABORT TASK (the task likely completed before the abort arrived).
- Log but do not fail on unexpected response codes from buggy targets.
- For ABORT TASK SET and LUN RESET, the initiator must clean up ALL local state for affected tasks, regardless of the target's response code.

**Warning signs:**
- Task management response handling with only two branches (success/failure).
- No local state cleanup after task management operations.
- ABORT TASK treated as mandatory-success.

**Phase to address:**
Task management phase, after basic full feature phase.

---

### Pitfall 9: CHAP Authentication Challenge-Response Ordering

**What goes wrong:**
CHAP authentication in iSCSI follows a specific exchange:
1. Target sends CHAP_A (algorithm list) -- initiator selects one.
2. Target sends CHAP_I (identifier), CHAP_C (challenge) -- initiator computes response.
3. Initiator sends CHAP_N (name), CHAP_R (response).
4. For mutual CHAP, initiator also sends CHAP_I, CHAP_C in step 3, and target responds with CHAP_N, CHAP_R.

Mistakes:
- Using the same CHAP secret for forward and reverse authentication (RFC 7143 requires distinct secrets to prevent reflection attacks).
- Accepting MD5 (CHAP_A=5) as the only algorithm without warning that it is cryptographically weak.
- Not validating the target's CHAP response in mutual authentication (defeats the purpose).
- CHAP_C challenge too short -- RFC recommends at least 16 bytes of random data.
- Identifier (CHAP_I) is a single byte (0-255) -- must be unique per challenge within a session.

**Why it happens:**
CHAP is conceptually simple but the ordering of key-value pairs within login PDUs matters, and mutual CHAP doubles the complexity. Developers test against targets with authentication disabled and add CHAP as an afterthought.

**How to avoid:**
- Implement CHAP as a dedicated authentication module, separate from login state machine.
- Enforce minimum challenge length of 16 bytes in generated challenges.
- Enforce distinct secrets for forward and reverse CHAP at the API level.
- Test against a real target with CHAP enabled (LIO targetcli supports CHAP configuration).
- Log all CHAP exchanges at debug level for troubleshooting.

**Warning signs:**
- CHAP implementation that only tests forward authentication, not mutual.
- API that accepts a single "password" field instead of separate initiator/target secrets.
- Challenge generation using `math/rand` instead of `crypto/rand`.

**Phase to address:**
Authentication phase, during or immediately after login negotiation.

---

### Pitfall 10: Concurrent PDU Read/Write on TCP Connection Without Proper Synchronization

**What goes wrong:**
An iSCSI connection multiplexes multiple outstanding commands over a single TCP connection. The initiator must be able to send command PDUs while simultaneously receiving responses, Data-In PDUs, and asynchronous events. In Go, the natural pattern is separate goroutines for reading and writing. The pitfalls:
- `net.Conn.Write` is not safe for concurrent use -- two goroutines writing PDUs simultaneously can interleave bytes, producing corrupt PDUs on the wire.
- `net.Conn.Read` is safe for a single reader, but the reader goroutine must correctly frame PDUs (read exactly 48 bytes BHS, parse DataSegmentLength, read data segment + padding + digests) without leaving partial reads that corrupt the next PDU.
- Shared session state (CmdSN, ExpStatSN, outstanding command map) accessed from both reader and writer goroutines creates data races.

**Why it happens:**
Go makes concurrency easy to write and hard to get right. The TCP connection looks like a simple `io.ReadWriteCloser`, so developers assume concurrent Read/Write is fine. It is for Read and Write separately (one reader, one writer) but not for multiple writers.

**How to avoid:**
- Use a single writer goroutine that receives PDUs to send via a channel. All command-sending code puts PDUs on the channel; the writer goroutine serializes them to the TCP connection.
- Use a single reader goroutine that reads PDUs from the TCP connection and dispatches them (to response channels, to async event handlers).
- Protect shared state (sequence numbers, command map) with appropriate synchronization. Prefer channel-based designs over mutexes where natural. Where mutexes are needed, protect the minimal critical section.
- Run all tests with `-race` flag. Integrate `go test -race` into CI from day one.

**Warning signs:**
- Multiple goroutines calling `conn.Write()` directly.
- Sequence number variables accessed without synchronization.
- Tests that pass without `-race` but fail with it.
- No channel-based PDU send path.

**Phase to address:**
Connection management phase -- the reader/writer goroutine architecture must be established before any command sending.

---

### Pitfall 11: PDU Framing Errors on Partial TCP Reads

**What goes wrong:**
TCP is a stream protocol with no message boundaries. A single `conn.Read()` call may return a partial PDU, multiple PDUs, or a PDU split across reads. iSCSI PDUs have a specific structure: 48-byte BHS, optional AHS (length from AHSLength field), optional header digest (4 bytes if negotiated), data segment (length from DataSegmentLength field, padded to 4-byte boundary), optional data digest (4 bytes if negotiated). Getting the framing wrong means every subsequent PDU read is corrupted.

**Why it happens:**
Developers use `conn.Read(buf)` and assume it fills the buffer. In Go, `io.ReadFull` exists for this purpose but developers sometimes use raw `Read`. Even with `io.ReadFull`, the total PDU length calculation is error-prone: forgetting AHS, forgetting padding, forgetting digests all cause frame desynchronization.

**How to avoid:**
- Always use `io.ReadFull` (or equivalent) to read exact byte counts.
- Implement PDU reading as a pipeline: read 48-byte BHS, parse AHSLength and DataSegmentLength, compute total remaining bytes (AHS + header digest + data segment padded to 4 bytes + data digest), read remaining bytes.
- Compute padding as `(4 - (dataSegmentLength % 4)) % 4`.
- Write a test that sends two back-to-back PDUs in a single TCP write and verifies the reader correctly separates them.
- Write a test that sends a PDU in 1-byte increments and verifies the reader still works.

**Warning signs:**
- Use of `conn.Read()` instead of `io.ReadFull()`.
- Padding calculation that does not handle DataSegmentLength already divisible by 4 (producing padding of 4 instead of 0).
- No test for back-to-back PDU reading.

**Phase to address:**
PDU layer -- the very first phase of implementation.

---

### Pitfall 12: MaxRecvDataSegmentLength Directionality Confusion

**What goes wrong:**
MaxRecvDataSegmentLength is declared by each side during login and specifies the maximum data segment length that side is willing to RECEIVE. This means:
- The initiator's declared MaxRecvDataSegmentLength limits what the target sends TO the initiator (Data-In PDU sizes).
- The target's declared MaxRecvDataSegmentLength limits what the initiator sends TO the target (Data-Out PDU sizes).

Confusing these causes the initiator to send PDUs larger than the target can receive, or to allocate receive buffers based on its own declared value instead of the target's.

**Why it happens:**
The name "MaxRecvDataSegmentLength" is ambiguous -- "recv" from whose perspective? Developers store one value and use it for both send and receive paths.

**How to avoid:**
- Store two separate values: `maxRecvFromTarget` (what the initiator declared) and `maxSendToTarget` (what the target declared).
- Name variables clearly to indicate direction.
- Use `maxSendToTarget` when splitting Data-Out PDUs. Use `maxRecvFromTarget` when allocating receive buffers.
- Validate in tests: declare initiator MRDSL=512, target MRDSL=1024, verify Data-Out PDUs are at most 1024 bytes and Data-In PDUs are at most 512 bytes.

**Warning signs:**
- A single `maxDataSegmentLength` variable used for both directions.
- Data-Out PDU splitting that uses the initiator's own declared value.

**Phase to address:**
Login negotiation and full feature phase -- affects both phases.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding negotiation defaults instead of implementing full negotiation | Faster first login | Cannot interoperate with targets that require specific values; breaks with strict targets | Never for a library claiming RFC compliance |
| Using `encoding/binary.Read` for all PDU parsing | Clean, simple code | ~3-5x slower than direct byte manipulation; allocates per call; causes GC pressure under load | Acceptable for initial implementation; optimize hot paths later |
| Single global mutex for all session state | Eliminates race conditions trivially | Serializes all PDU processing; throughput ceiling under concurrent I/O | Acceptable for Phase 1; refactor to fine-grained locking or channels before performance phase |
| Skipping AHS (Additional Header Segments) parsing | 99% of PDUs have no AHS | Breaks extended CDB support (CDBs > 16 bytes); fails UNH-IOL conformance tests | Never -- parse the AHS length field even if the handler is a no-op |
| Not implementing SNACK (ERL 1) | Simpler error recovery | Cannot recover from single-PDU digest errors without full session reset; wastes bandwidth on lossy networks | Acceptable for v1 if ERL 0 is solid |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| LIO (Linux kernel target) | Assuming LIO implements every RFC feature perfectly -- LIO has bugs in text negotiation, SCSI command support (e.g., "Unsupported SA: 0x12" for GET_LBA_STATUS) | Test against LIO but do not use LIO behavior as the spec reference. Validate against RFC text, not target behavior. |
| Microsoft iSCSI Target | Trusting that "Function Complete" response to task management means the task was actually aborted (documented to return success for non-existent tasks) | Treat task management responses defensively; always clean up local state regardless of target response. |
| TrueNAS / FreeNAS | NOP timeout and reconnection issues during sustained I/O; "BUS_RESET" sense key errors during connection recovery | Implement NOP-In/NOP-Out keepalive handling robustly; handle unexpected SCSI sense data gracefully. |
| gotgt (Go iSCSI target for testing) | gotgt is under heavy development, may not implement all RFC features; behavior may differ from production targets | Use gotgt for development convenience but validate against LIO or another mature target for conformance. |
| Targets with digest enforcement | Connecting with DataDigest=None when target requires CRC32C causes immediate disconnect | Always attempt to negotiate CRC32C and fall back to None, not the other way around. Let the user configure preference. |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Allocating new byte slices for every PDU | High GC pause times, throughput drops under load | Use `sync.Pool` for PDU buffers; pre-allocate buffers matching MaxRecvDataSegmentLength | > 1000 IOPS; immediately visible with pprof heap profile |
| Using `encoding/binary.Read/Write` in hot path | CPU bottleneck in PDU serialization; high allocation rate | Use direct byte manipulation (`binary.BigEndian.PutUint32` etc.) for BHS encoding/decoding | > 5000 IOPS; profile shows time spent in reflection |
| Unbounded outstanding command map | Memory growth proportional to un-reaped commands; map never shrinks | Enforce MaxCmdSN window; clean up command map entries promptly on response receipt | Sustained load with slow target responses |
| Channel-based PDU dispatch with unbuffered channels | Writer goroutine blocks when reader is slow; deadlock potential | Use buffered channels sized to command window; implement backpressure via CmdSN window | Concurrent I/O with > 32 outstanding commands |
| String operations in negotiation hot path | Login takes unexpectedly long; GC pressure from string concatenation | Pre-allocate negotiation buffers; avoid `fmt.Sprintf` in PDU encoding paths | Not a throughput issue (login is infrequent), but affects connection establishment latency |
| Computing CRC32C without hardware acceleration | CPU-bound digest computation limiting throughput | Go's `crc32.MakeTable(crc32.Castagnoli)` auto-detects SSE 4.2; verify with benchmarks that hardware path is used | > 100 MB/s throughput with digests enabled |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Using `math/rand` for CHAP challenge generation | Predictable challenges enable authentication bypass; attacker can precompute CHAP response | Use `crypto/rand.Read()` exclusively for all security-relevant random values (CHAP challenges, ISIDs in security contexts) |
| Same secret for forward and reverse CHAP | Enables reflection attack: attacker captures target's challenge/response and replays it back | Enforce at API level that initiator secret != target secret; document this requirement clearly |
| No mutual CHAP validation | Man-in-the-middle can impersonate the target; initiator connects to rogue target and sends data | When mutual CHAP is configured, ALWAYS validate the target's CHAP response; abort login if validation fails |
| Storing CHAP secrets in plaintext in configuration | Credential exposure if config files are leaked | Accept secrets via environment variables or callback functions, not static config; never log CHAP secrets even at debug level |
| No IPsec guidance in documentation | Users assume CHAP protects data in transit (it does not -- data phase is unencrypted) | Document clearly that CHAP only authenticates the login; data is sent in cleartext; recommend IPsec for sensitive environments |
| Accepting arbitrary SCSI commands without validation | Library user can issue destructive commands (FORMAT UNIT, WRITE SAME with UNMAP) accidentally | Provide clear API documentation; consider optional command allow-listing for high-level API |

## "Looks Done But Isn't" Checklist

- [ ] **Login negotiation:** Often missing handling of T=0 (non-transit) responses from target -- verify multi-step negotiation within a single CSG works
- [ ] **Login negotiation:** Often missing Login Redirect (status class 0x01) -- verify redirect handling including DNS resolution of new target address
- [ ] **Full feature phase:** Often missing NOP-In handling when InitiatorTaskTag=0xFFFFFFFF (target-initiated NOP ping) -- verify NOP-Out response is sent
- [ ] **Full feature phase:** Often missing Async Message (opcode 0x32) handling -- verify at minimum graceful handling of target requesting logout or connection drop
- [ ] **Write path:** Often missing the case where target sends R2T with DesiredDataTransferLength=0 (illegal per RFC but some targets do it) -- verify graceful error handling
- [ ] **Error recovery:** Often missing ExpCmdSN/MaxCmdSN update processing on EVERY response PDU (not just SCSI Response) -- verify sequence numbers are updated from all PDU types that carry them
- [ ] **Digest computation:** Often missing data segment padding to 4-byte boundary before computing data digest -- verify with odd-length data segments
- [ ] **Task management:** Often missing local cleanup of affected tasks after LUN RESET or TARGET RESET -- verify all pending commands for the LUN/target are failed to callers
- [ ] **Session reinstatement:** Often missing cleanup of old session state when re-logging with same ISID -- verify no stale ITTs or sequence numbers survive reinstatement
- [ ] **Text negotiation:** Often missing response to keys sent by target during operational negotiation (target-initiated key declarations) -- verify bidirectional negotiation

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Sequence number arithmetic wrong | MEDIUM | Replace all raw comparisons with serial arithmetic function; grep for raw uint32 comparisons on SN variables; comprehensive test at wrap boundary |
| R2T/Data-Out sequencing incorrect | HIGH | Requires rewriting write path; extract burst/PDU splitting into pure functions for testability; build parameterized test matrix |
| Key-value parsing fragility | MEDIUM | Refactor negotiation into state machine with per-key handlers; add fuzz testing for malformed responses |
| Login state machine shortcuts | MEDIUM | Add missing states and transitions; test each transition independently; add integration test with CHAP-enforcing target |
| CRC32C computation wrong | LOW | Fix polynomial table; validate against known-good packet capture; isolated change with no architectural impact |
| Concurrent write corruption | HIGH | Requires architectural change to channel-based writer; all callers must change from direct write to channel send |
| PDU framing errors | HIGH | Replace Read with ReadFull; recalculate all length computations; add framing tests with pathological TCP segmentation |
| MaxRecvDataSegmentLength confusion | MEDIUM | Split into two variables; grep for all uses; correct send/receive paths independently |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Sequence number arithmetic | Phase 1: PDU types and core helpers | Unit tests with wrap-around values pass |
| PDU framing errors | Phase 1: PDU serialization | Back-to-back and 1-byte-at-a-time read tests pass |
| CRC32C digest computation | Phase 1: PDU serialization | Digest matches known-good reference values from packet capture |
| Concurrent read/write | Phase 2: Connection management | All tests pass with `-race`; channel-based writer architecture in place |
| ITT reuse collision | Phase 2: Connection/session management | Allocator test shows no ITT reuse while commands outstanding |
| Login state machine | Phase 3: Login negotiation | Multi-step login with CHAP target succeeds; redirect handling tested |
| Text negotiation parsing | Phase 3: Login negotiation | Fuzz test corpus of malformed negotiations; C-bit continuation tested |
| MaxRecvDataSegmentLength directionality | Phase 3: Login negotiation | Asymmetric MRDSL test (different values each direction) |
| CHAP authentication | Phase 3: Authentication | Mutual CHAP against LIO target succeeds; challenge uses crypto/rand |
| R2T/Data-Out sequencing | Phase 4: Full feature phase (writes) | Parameterized test matrix: all ImmediateData x InitialR2T x burst size combinations |
| Error recovery (ERL 0) | Phase 5: Error recovery | Fault injection: kill TCP mid-transfer, verify session reinstatement and command retry |
| Task management responses | Phase 6: Task management | Test all five response codes; verify local state cleanup after LUN RESET |

## Sources

- [RFC 7143: Internet Small Computer System Interface (iSCSI) Protocol (Consolidated)](https://www.rfc-editor.org/rfc/rfc7143.html) -- Section 10 "Notes to Implementers", Section 8 "Error Recovery"
- [RFC 3385: iSCSI CRC/Checksum Considerations](https://www.rfc-editor.org/rfc/rfc3385)
- [Microsoft iSCSI Target Server Implementation Notes (Windows Server 2012)](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-r2-and-2012/jj863561(v=ws.11)) -- Comprehensive list of UNH-IOL compliance test failures
- [Microsoft iSCSI Software Target 3.3 Implementation Notes](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-storage-solutions/gg983494(v=ws.10)) -- R2T and burst length violation documentation
- [UNH-IOL iSCSI Target Full Feature Phase Test Suite v3.0](https://www.iol.unh.edu/sites/default/files/testsuites/iscsi/target_ffp_v3.0.pdf)
- [EDK2 iSCSI DXE Remote Memory Exposure (GHSA-8522-69fh-w74x)](https://github.com/tianocore/edk2/security/advisories/GHSA-8522-69fh-w74x) -- Integer overflow in R2T handling
- [Red Hat Bug 1624678: Login negotiation failed due to authentication enforcement](https://bugzilla.redhat.com/show_bug.cgi?id=1624678)
- [Linux iSCSI Target Error Recovery Level documentation](https://linux-iscsi.org/wiki/error_recovery_level)
- [iSCSI abort task discussion (CMU IPS mailing list)](https://www.pdl.cmu.edu/mailinglists/ips/mail/msg06309.html)
- [Black Hat 2005: iSCSI Security (Insecure SCSI)](https://blackhat.com/presentations/bh-usa-05/bh-us-05-Dwivedi-update.pdf) -- CHAP vulnerability analysis
- [Go Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Go zero-copy techniques for network programming](https://goperf.dev/01-common-patterns/zero-copy/)
- [uber-go/goleak: Goroutine leak detector](https://github.com/uber-go/goleak)
- [gostor/gotgt: Go iSCSI Target framework](https://github.com/gostor/gotgt)

---
*Pitfalls research for: pure-userspace iSCSI initiator (RFC 7143) in Go*
*Researched: 2026-03-31*
