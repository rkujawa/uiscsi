# Project Research Summary

**Project:** uiscsi
**Domain:** Pure-userspace iSCSI initiator library (RFC 7143) in Go
**Researched:** 2026-03-31
**Confidence:** HIGH

## Executive Summary

Building a pure-userspace iSCSI initiator library in Go is a well-scoped protocol implementation project. The iSCSI protocol (RFC 7143, 295 pages) is a mature, stable standard with clear PDU formats, state machines, and negotiation rules. The recommended approach is a zero-external-dependency library built entirely on Go 1.25 stdlib (encoding/binary, hash/crc32, net, crypto/md5, log/slog, context). No existing Go library implements iSCSI at the protocol level -- all current Go packages (dell/goiscsi, kubernetes-csi/csi-lib-iscsi) merely wrap the iscsiadm CLI. The closest reference implementation is libiscsi (C), which provides both architectural patterns to study and a conformance test suite to inform test design. The core value proposition is platform independence: no CGo, no kernel modules, runs anywhere Go compiles including NetBSD.

The architecture follows a clean layered design: Transport (TCP) -> PDU Codec -> Connection (read/write pumps, state machine) -> Session (command sequencing) -> SCSI (CDB builders, sense parsing) -> Public API (high-level typed + low-level raw CDB). This bottom-up layering maps directly to the build order. The most complex subsystems are the write path (R2T/Data-Out sequencing with burst length enforcement), login negotiation (multi-PDU state machine with CHAP), and error recovery (ERL 0 session reinstatement). These three areas account for the majority of documented implementation bugs across all iSCSI initiators.

The key risks are: (1) getting the write path wrong due to the combinatorial matrix of ImmediateData/InitialR2T/burst length parameters, (2) fragile text negotiation parsing that breaks against real targets with quirks, and (3) sequence number arithmetic bugs that only manifest after billions of commands. All three are mitigable through table-driven testing, explicit state machines, and dedicated serial arithmetic helpers built from day one. The project should use gostor/gotgt as an in-process test target for integration tests, with validation against LIO for conformance.

## Key Findings

### Recommended Stack

The entire runtime stack is Go 1.25 stdlib with zero external dependencies. This is both a technical choice and a library design principle -- a Go library should not force logging frameworks or serialization dependencies on consumers.

**Core technologies:**
- **Go 1.25 stdlib (encoding/binary, hash/crc32, net, crypto/*):** Complete coverage of iSCSI needs -- binary PDU encoding, CRC32C with hardware acceleration (SSE4.2), TCP networking, MD5-HMAC for CHAP
- **log/slog:** Structured logging with injectable Handler -- consumers plug their own logger
- **testing/synctest (Go 1.25):** Deterministic concurrent test framework for state machine and timeout testing
- **gostor/gotgt (test only):** Pure Go iSCSI target for integration tests -- embeddable in test processes

**Critical version note:** Go 1.25 required for testing/synctest graduation (no longer experimental). Available on NetBSD 10.1 via pkgsrc.

### Expected Features

**Must have (table stakes -- RFC 7143 mandatory or basic usability):**
- Login phase state machine with full Section 13 key negotiation
- CHAP and mutual CHAP authentication (plus AuthMethod=None)
- SCSI Command/Response PDU transport with CmdSN/ExpCmdSN windowing
- Data-In (read) and Data-Out with R2T/immediate/unsolicited data (write)
- Header and data digest (CRC32C) negotiation and computation
- NOP-Out/NOP-In keepalive, async message handling, logout
- Task management functions (ABORT TASK, LUN RESET, etc.)
- Error Recovery Level 0 (session reconnect)
- SendTargets discovery
- Core SCSI commands: TEST UNIT READY, INQUIRY, READ CAPACITY, READ/WRITE 10/16, REQUEST SENSE, REPORT LUNS, MODE SENSE
- Raw CDB pass-through and structured sense data parsing

**Should have (v1.x differentiators):**
- Error Recovery Levels 1 and 2
- SYNCHRONIZE CACHE, WRITE SAME, UNMAP (thin provisioning)
- PERSISTENT RESERVE IN/OUT (clustering)
- Extended VPD page parsing, connection statistics

**Defer (v2+):**
- Multiple connections per session (MC/S) -- rarely used, massive complexity
- iSNS discovery, iSER (RDMA), kernel block device emulation
- SANITIZE, EXTENDED COPY, legacy CDB formats

**Anti-features (explicitly avoid):**
- Kernel block device emulation (defeats userspace purpose)
- Built-in opinionated retry policies (application-specific)
- Automatic LUN scanning (policy belongs in application)

### Architecture Approach

The architecture is a six-layer stack (Transport -> PDU -> Connection -> Session -> SCSI -> API) with clear boundaries. Each layer communicates via well-defined interfaces: channels between session and connection, function calls for codec operations, io.ReadWriteCloser for transport abstraction.

**Major components:**
1. **PDU Codec + Digest Engine** -- Pure encode/decode of 48-byte BHS, AHS, data segments with CRC32C; zero I/O dependencies, trivially testable
2. **Connection Layer (FSM + Read/Write Pumps + PDU Router)** -- Goroutine-per-connection model with channel-based write serialization and ITT-correlated response dispatch
3. **Login Negotiator + Auth (CHAP)** -- Multi-PDU login state machine with typed negotiation engine (boolean AND/OR, numerical min/max, string list semantics per key)
4. **Session Manager + Command Sequencer** -- Session state machine, CmdSN/MaxCmdSN windowing, ERL 0 reconnection
5. **SCSI Layer (CDB Builder + Response Parser)** -- Protocol-independent CDB construction and sense data parsing; can be built in parallel with iSCSI layers
6. **Public API (High-Level + Low-Level + Discovery)** -- Go-idiomatic typed functions (ReadBlocks, Inquiry) built on raw CDB pass-through

**Key architectural pattern:** Explicit table-driven state machines for connection FSM and login phases, directly traceable to RFC 7143 Section 8 state diagrams.

### Critical Pitfalls

1. **Sequence number wrap-around** -- CmdSN/StatSN use RFC 1982 serial arithmetic, not simple integer comparison. Build a `serialCmp()` helper before writing any PDU processing code. Test at 0xFFFFFFF0 wrap boundary.
2. **R2T/Data-Out sequencing errors** -- The ImmediateData x InitialR2T x burst length matrix creates many write path variants. Build a truth table for all combinations; parameterized tests covering boundary cases are essential.
3. **Text negotiation fragility** -- Null-byte separators, C-bit continuation across PDUs, per-key type semantics (AND/OR/min/max), MaxRecvDataSegmentLength directionality. Implement a typed negotiation engine, not string splitting.
4. **PDU framing on TCP streams** -- Always use io.ReadFull; compute total PDU length including AHS, padding, and digests. Test with back-to-back PDUs and 1-byte-at-a-time delivery.
5. **Concurrent TCP write corruption** -- Single writer goroutine with channel-based send path from day one. Run all tests with -race flag in CI.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: PDU Foundation
**Rationale:** Everything depends on correct PDU encoding/decoding. This layer has zero dependencies on higher layers and is the natural starting point. SCSI CDB builders can be built in parallel since they also have no iSCSI dependencies.
**Delivers:** PDU codec (BHS encode/decode, all opcodes), CRC32C digest engine, serial number arithmetic helpers, transport abstraction interface, SCSI CDB builder/response parser library
**Addresses:** PDU framing, header/data digest computation, CDB construction for all core SCSI commands, sense data parsing
**Avoids:** Pitfall 1 (sequence number arithmetic), Pitfall 5 (CRC32C errors), Pitfall 11 (PDU framing)

### Phase 2: Connection Management
**Rationale:** The connection layer is the next dependency -- session and login both need read/write pumps, PDU routing, and the connection FSM. The goroutine architecture must be established here.
**Delivers:** Read pump, write pump (channel-based), PDU router with ITT correlation, connection FSM (S1-S8 states)
**Addresses:** Goroutine-per-connection concurrency model, ITT allocation and tracking
**Avoids:** Pitfall 7 (ITT reuse), Pitfall 10 (concurrent write corruption)

### Phase 3: Login and Authentication
**Rationale:** No session can be established without login. Login depends on the connection layer and PDU codec. This is the most interop-sensitive code -- real targets have quirks.
**Delivers:** Login state machine (SecurityNegotiation, OperationalNegotiation, FullFeaturePhase transitions), typed text negotiation engine, CHAP and mutual CHAP authentication, AuthMethod=None
**Addresses:** Full Section 13 key negotiation, multi-PDU login exchanges, login redirect handling
**Avoids:** Pitfall 3 (negotiation fragility), Pitfall 4 (login shortcuts), Pitfall 9 (CHAP ordering), Pitfall 12 (MaxRecvDataSegmentLength directionality)

### Phase 4: Session and Read Path
**Rationale:** With login working, establish the session layer and implement the simpler I/O direction first. Reads are target-driven (Data-In PDUs) and involve less initiator-side complexity than writes.
**Delivers:** Session state machine, CmdSN/ExpCmdSN/MaxCmdSN windowing, SCSI Command PDU submission, Data-In accumulation, NOP-Out/NOP-In keepalive, async message handling, basic high-level read API
**Addresses:** Command numbering and flow control, read I/O path, connection health monitoring
**Avoids:** Pitfall 1 (sequence number windowing in practice)

### Phase 5: Write Path
**Rationale:** The write path is the most complex subsystem due to R2T handling, immediate/unsolicited data, and burst length enforcement. Isolating it in its own phase allows focused testing of all parameter combinations.
**Delivers:** Data-Out PDU generation, R2T handling, immediate data, unsolicited data, FirstBurstLength/MaxBurstLength enforcement, high-level write API
**Addresses:** Complete bidirectional I/O, all write path variants
**Avoids:** Pitfall 2 (R2T/Data-Out sequencing -- the highest-impact pitfall for data integrity)

### Phase 6: Error Recovery and Task Management
**Rationale:** ERL 0 is the minimum viable error handling and should be solid before the library is usable. Task management functions are closely related (abort/reset during recovery). ERL 1/2 can follow later.
**Delivers:** ERL 0 session reinstatement, task management functions (all six TMFs), logout (normal + recovery), proper ISID management for reinstatement
**Addresses:** Connection failure recovery, command cleanup, session lifecycle
**Avoids:** Pitfall 6 (ERL complexity -- by focusing on ERL 0 first), Pitfall 8 (TMF response misinterpretation)

### Phase 7: Discovery and Public API Polish
**Rationale:** With the core protocol working end-to-end, add discovery sessions and polish the public API surface for library consumers.
**Delivers:** SendTargets discovery, complete high-level typed API (ReadBlocks, WriteBlocks, Inquiry, ReadCapacity, etc.), raw CDB pass-through API, discovery API, documentation
**Addresses:** Target enumeration, Go-idiomatic API design, library usability

### Phase 8: Hardening and Advanced Features
**Rationale:** Once the core is validated against real targets, add ERL 1/2, advanced SCSI commands, and performance optimizations.
**Delivers:** ERL 1 (SNACK), ERL 2 (connection recovery), SYNC CACHE, WRITE SAME, UNMAP, PERSISTENT RESERVE, connection statistics, buffer pooling optimization
**Addresses:** v1.x differentiator features, production readiness

### Phase Ordering Rationale

- **Bottom-up by dependency:** Each phase produces artifacts the next phase consumes. PDU codec before connection, connection before login, login before session.
- **Read before write:** The read path is simpler (target-driven Data-In) and validates the session/command infrastructure before tackling the complex write path.
- **ERL 0 before ERL 1/2:** Session-level recovery is the foundation. Higher ERLs extend it, not replace it. Attempting all three simultaneously is a documented failure pattern.
- **SCSI layer parallel-buildable:** CDB builders and sense parsers have zero iSCSI dependency and can be built alongside Phase 1-2, providing early testable artifacts.
- **Integration testing throughout:** Each phase should include integration tests against gotgt to catch interop issues early, not just unit tests.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Login/Auth):** Complex interop surface with real-world target quirks documented in UNH-IOL testing. Study libiscsi's login implementation and Microsoft/LIO known issues before implementing.
- **Phase 5 (Write Path):** The ImmediateData x InitialR2T x burst length matrix needs a comprehensive test plan derived from UNH-IOL FFP test suite structure.
- **Phase 6 (Error Recovery):** ERL 0 session reinstatement has subtle ISID reuse and command retry semantics. Study open-iscsi's recovery implementation.

Phases with standard patterns (skip research-phase):
- **Phase 1 (PDU Foundation):** Well-defined binary format from RFC. Table-driven encode/decode is standard Go practice.
- **Phase 2 (Connection Management):** Goroutine-per-connection with channel multiplexing is a well-documented Go networking pattern.
- **Phase 4 (Session/Read Path):** Read path is straightforward Data-In accumulation. Command windowing follows RFC precisely.
- **Phase 7 (Discovery/API):** SendTargets is a simple text exchange. API design follows Go conventions.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Zero external dependencies; entirely Go stdlib. Go 1.25 features verified. gostor/gotgt is the only uncertainty (no stable releases, but active development). |
| Features | HIGH | RFC 7143 is the authoritative source for mandatory features. Competitive analysis against libiscsi and open-iscsi provides clear baselines. Feature prioritization is unambiguous. |
| Architecture | HIGH | Layered protocol stack is the standard pattern for iSCSI implementations (libiscsi, open-iscsi both follow it). Goroutine-per-connection is idiomatic Go. Build order follows natural dependency chain. |
| Pitfalls | HIGH | Pitfalls sourced from RFC implementation notes, UNH-IOL conformance testing reports, documented CVEs (EDK2), and real bug reports (Red Hat, Microsoft). These are battle-tested failure modes, not speculation. |

**Overall confidence:** HIGH

### Gaps to Address

- **gotgt reliability as test target:** gotgt has no stable releases and may not implement all RFC features. Plan to fall back to a minimal custom test target or LIO for conformance validation. Verify gotgt compatibility with Go 1.25 early.
- **NetBSD-specific testing:** Go on NetBSD is supported but less heavily tested than Linux/macOS. Verify that TCP networking, CRC32C hardware acceleration, and test infrastructure work correctly on NetBSD 10.1 early in Phase 1.
- **Performance baselines:** No research was done on expected throughput or latency targets. Establish benchmarks during Phase 4 (first I/O) to inform buffer pooling and optimization decisions in Phase 8.
- **TLS support:** The architecture mentions optional TLS wrapping but neither STACK.md nor FEATURES.md addresses it in detail. Determine if TLS is a v1 requirement or can be deferred.

## Sources

### Primary (HIGH confidence)
- [RFC 7143 - iSCSI Protocol (Consolidated)](https://datatracker.ietf.org/doc/html/rfc7143) -- Protocol specification, PDU formats, state machines, negotiation rules, error recovery
- [RFC 7144 - iSCSI SCSI Features Update](https://www.rfc-editor.org/rfc/rfc7144.html) -- SCSI feature clarifications
- [RFC 3385 - iSCSI CRC/Checksum Considerations](https://www.rfc-editor.org/rfc/rfc3385) -- CRC32C rationale
- [Go 1.25 Release Notes](https://go.dev/doc/go1.25) -- testing/synctest, runtime features
- [Go stdlib package documentation](https://pkg.go.dev/) -- encoding/binary, hash/crc32, log/slog, net
- [UNH-IOL iSCSI Test Suites](https://www.iol.unh.edu/testing/storage/iscsi/test-plans) -- Conformance test structure
- [sahlberg/libiscsi](https://github.com/sahlberg/libiscsi) -- Reference C userspace initiator, conformance test suite

### Secondary (MEDIUM confidence)
- [gostor/gotgt](https://github.com/gostor/gotgt) -- Go iSCSI target for testing (no stable releases)
- [open-iscsi](https://github.com/open-iscsi/open-iscsi) -- Linux kernel initiator (architecture reference, not dependency)
- [Go on NetBSD wiki](https://go.dev/wiki/NetBSD) -- Platform support status
- [EDK2 iSCSI CVE (GHSA-8522-69fh-w74x)](https://github.com/tianocore/edk2/security/advisories/GHSA-8522-69fh-w74x) -- R2T overflow vulnerability

### Tertiary (LOW confidence)
- [Black Hat 2005: iSCSI Security](https://blackhat.com/presentations/bh-usa-05/bh-us-05-Dwivedi-update.pdf) -- CHAP vulnerability analysis (dated but still relevant for reflection attack awareness)

---
*Research completed: 2026-03-31*
*Ready for roadmap: yes*
