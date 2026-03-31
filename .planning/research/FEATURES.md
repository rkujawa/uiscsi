# Feature Research

**Domain:** Pure-userspace iSCSI initiator library (Go)
**Researched:** 2026-03-31
**Confidence:** HIGH (RFC 7143 is a stable, well-documented standard; libiscsi and open-iscsi provide clear competitive baselines)

## Feature Landscape

This analysis spans two layers: the iSCSI transport layer (RFC 7143) and the SCSI command layer (SPC/SBC) exposed through it. Features are categorized based on RFC mandate level, real-world deployment expectations, and competitive positioning against libiscsi (C, userspace) and open-iscsi (C, kernel-coupled).

### Table Stakes (Users Expect These)

Missing any of these means the library is either spec-noncompliant or unusable in real deployments.

#### iSCSI Protocol Layer (RFC 7143 Mandatory)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Login phase state machine | RFC 7143 MUST -- no login, no session | HIGH | Security, operational, and leading-connection negotiation phases; multi-PDU exchange with state tracking |
| Text negotiation (all Section 13 keys) | RFC 7143 MUST for all keys except X- extensions | MEDIUM | HeaderDigest, DataDigest, MaxConnections, InitialR2T, ImmediateData, MaxRecvDataSegmentLength, MaxBurstLength, FirstBurstLength, DefaultTime2Wait, DefaultTime2Retain, MaxOutstandingR2T, DataPDUInOrder, DataSequenceInOrder, ErrorRecoveryLevel, SessionType |
| CHAP authentication | RFC 7143 MUST -- "compliant initiators and targets MUST implement CHAP" | MEDIUM | Includes one-way CHAP; must handle challenge/response over login PDUs |
| Mutual CHAP | Expected in enterprise deployments; prevents rogue targets | MEDIUM | Bidirectional authentication; initiator also challenges target |
| AuthMethod=None | RFC 7143 MUST support negotiating no auth | LOW | Trivial but must be a negotiable option |
| SCSI Command PDU (request/response) | Core purpose of iSCSI -- carrying SCSI CDBs | HIGH | Includes CmdSN/ExpStatSN tracking, command windowing, bidirectional support |
| Data-In PDU handling | How read data arrives from target | MEDIUM | Sequence number tracking, data offset validation, F-bit handling |
| Data-Out PDU generation | How write data is sent to target | HIGH | Must handle solicited (R2T-prompted) and unsolicited data; FirstBurstLength/MaxBurstLength enforcement |
| R2T (Ready to Transfer) handling | RFC 7143 MUST for solicited writes | HIGH | R2T tracking, R2TSN sequencing, MaxOutstandingR2T compliance, buffer offset management |
| Immediate data | Negotiable but expected to work when enabled (default: Yes) | MEDIUM | Write data piggybacked on SCSI Command PDU; bounded by FirstBurstLength |
| Unsolicited data | Negotiable via InitialR2T=No; commonly used for performance | MEDIUM | Data-Out PDUs sent before first R2T; bounded by FirstBurstLength |
| Header digest (CRC32C) | RFC 7143 MUST support negotiation; expected in production | MEDIUM | CRC32C over 48-byte BHS; negotiated per-connection |
| Data digest (CRC32C) | RFC 7143 MUST support negotiation; often disabled for performance | MEDIUM | CRC32C over data segment; negotiated per-connection |
| Task management functions | RFC 7143 MUST -- ABORT TASK, ABORT TASK SET, LUN RESET, TARGET WARM RESET, TARGET COLD RESET, CLEAR TASK SET | HIGH | Each TMF has specific semantics, response codes, and effect on outstanding commands |
| NOP-Out / NOP-In | RFC 7143 MUST -- connection keepalive and ping | LOW | Initiator sends NOP-Out, target responds NOP-In; also target-initiated NOP-In requires NOP-Out response |
| Logout (normal and recovery) | RFC 7143 MUST | MEDIUM | Clean session/connection teardown; also used for connection recovery |
| Async message handling | RFC 7143 MUST handle target-initiated async events | MEDIUM | Includes SCSI async event, logout request, session/connection drop, vendor-specific |
| Error Recovery Level 0 | RFC 7143 MUST -- session-level recovery (reconnect) | HIGH | Session reinstatement, connection cleanup, command retry semantics |
| SendTargets discovery | Standard discovery mechanism; expected by all users | MEDIUM | Discovery session type; text request/response for target enumeration |
| Command numbering and windowing | RFC 7143 MUST -- CmdSN/ExpCmdSN/MaxCmdSN tracking | HIGH | Flow control, ordering guarantees, out-of-window detection |
| Connection multiplexing state machine | RFC 7143 connection state machine (even for single-conn sessions) | HIGH | Login, Full Feature, Logout, Cleanup states with defined transitions |

#### SCSI Command Layer (Minimum Viable)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| TEST UNIT READY | Basic device health check; first thing any initiator does | LOW | 6-byte CDB, no data transfer |
| INQUIRY (standard + VPD pages) | Device identification; required to know what you are talking to | MEDIUM | Standard inquiry data + VPD page 0x00 (supported pages), 0x80 (serial), 0x83 (device ID) |
| READ CAPACITY (10 and 16) | Must know device size before I/O | LOW | RC10 for devices under 2TB; RC16 mandatory for large devices |
| READ (10 and 16) | Core block read operation | MEDIUM | 10-byte for compatibility; 16-byte for large LBAs and protection info |
| WRITE (10 and 16) | Core block write operation | HIGH | Involves Data-Out PDUs, R2T handling, unsolicited data paths |
| REQUEST SENSE | Error detail retrieval | LOW | Returns sense data with error codes |
| REPORT LUNS | LUN discovery; SPC mandatory | LOW | Returns list of addressable LUNs on the target |
| MODE SENSE (6 and 10) | Device parameter queries (caching, geometry, etc.) | MEDIUM | Multiple mode pages; 6-byte for legacy, 10-byte for modern |
| Raw CDB pass-through | Power users build arbitrary CDBs; library just transports | MEDIUM | The escape hatch -- any SCSI command the library does not wrap |

### Differentiators (Competitive Advantage)

These are not required for spec compliance or basic operation, but set this library apart from alternatives.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Pure Go, zero C dependencies | No CGo, no libiscsi linking, no kernel modules; runs anywhere Go compiles including containers and constrained environments | N/A (architectural) | This IS the core value proposition; not a feature to add but a constraint to maintain |
| Error Recovery Level 1 (within-connection) | PDU retransmission without dropping connection; libiscsi does not expose this cleanly | HIGH | SNACK mechanism for data/status recovery; most initiators only do ERL 0 |
| Error Recovery Level 2 (connection-level) | Connection recovery within session without losing session state; rare in userspace libs | VERY HIGH | Connection allegiance reassignment, task reassignment across connections; very few implementations do this well |
| Typed high-level SCSI API | Go-idiomatic ReadBlocks/WriteBlocks/Inquiry/ReadCapacity with structured returns | MEDIUM | libiscsi has this in C; no Go equivalent exists. Proper Go types, error handling, context.Context |
| Async/event-driven architecture | Full async command pipeline with Go channels/contexts | MEDIUM | libiscsi is async but C callback-based; Go channels + context.Context is naturally better DX |
| WRITE SAME (10, 16) | Efficient zeroing/pattern-fill of large ranges; thin provisioning prep | LOW | Single CDB writes same data to range of LBAs |
| UNMAP | Thin provisioning / TRIM support; increasingly expected | LOW | Tells target that LBA ranges are no longer in use |
| SYNCHRONIZE CACHE (10, 16) | Flush write cache to persistent media; data integrity | LOW | Important for correctness but often overlooked |
| PERSISTENT RESERVE IN/OUT | SCSI-3 persistent reservations for clustering/fencing | HIGH | Required for shared-storage clusters (WSFC, Pacemaker); complex state machine |
| COMPARE AND WRITE | Atomic compare-and-swap at block level | MEDIUM | Used for distributed locking and cluster fencing |
| Structured sense data parsing | Decode sense data into Go types with descriptor/fixed format support | MEDIUM | Most C libs return raw bytes; Go structs with proper error classification |
| VERIFY (10, 16) | Data verification without read-back; integrity checks | LOW | Useful for scrubbing and validation workflows |
| START STOP UNIT | Spin-up/spin-down; eject; power management | LOW | Needed for proper device lifecycle management |
| READ DEFECT DATA (10, 12) | Defect list retrieval for health monitoring | LOW | Niche but valuable for storage management tools |
| Comprehensive VPD page parsing | Block limits (0xB0), block characteristics (0xB1), logical block provisioning (0xB2) | MEDIUM | Essential for understanding device capabilities (max transfer length, thin provisioning support, etc.) |
| IOL-inspired conformance test suite | Built-in conformance validation against RFC 7143 | HIGH | No other Go library has this; increases trust for production use |
| Connection-level statistics | Latency, throughput, error counts, retry counts per connection | LOW | Observable library; critical for production debugging |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Multiple connections per session (MC/S) | RFC allows it; seems like "more connections = more performance" | Massively increases state machine complexity (connection allegiance, task routing, failover); almost never used in practice; most targets limit to 1 connection; performance gain is negligible with modern TCP | Single connection per session with reconnection. If users need more throughput, use multiple sessions to different portal groups. Defer MC/S to v2+ if ever. |
| iSER (RDMA transport) | High-performance environments want RDMA | Completely different transport layer; requires RDMA hardware and drivers; niche deployment; undermines pure-userspace value | Out of scope. Users who need iSER already have infrastructure-specific solutions. |
| iSNS discovery | Enterprise SANs use iSNS for centralized discovery | Adds an entire additional protocol dependency (iSNS client); SendTargets handles 95%+ of use cases | SendTargets discovery. If iSNS is truly needed, it can be added as an optional module later. |
| Kernel block device emulation | "I want /dev/sdX from your library" | Defeats the entire purpose of a userspace library; requires kernel modules (NBD, TCMU, FUSE); platform-specific; fragile | Expose io.ReadWriteSeeker interface. If users need a block device, they can use NBD themselves with the library's I/O interface. |
| Boot from iSCSI | Diskless boot over iSCSI | Requires kernel/firmware involvement by nature; not a library concern | Out of scope. This is a firmware/bootloader feature. |
| IPsec integration | RFC 7143 references IPsec as a security mechanism | IPsec is a network-layer concern, not an application library concern; adding IPsec into a userspace library is wrong layering | Document that IPsec should be configured at the OS/network level. The library operates over whatever TCP connection it is given. |
| Automatic LUN scanning and device management | "Just give me all the disks" | Opinionated device management policy does not belong in a library; different users want different behaviors | Provide building blocks (SendTargets, REPORT LUNS, INQUIRY) and let the application implement its own discovery/management policy. |
| Built-in retry/reconnection policy | "Handle all errors for me" | Retry policies are application-specific; wrong retry logic causes data corruption or hangs | Provide ERL 0/1/2 mechanisms and hooks/callbacks for the application to implement its own policy. Offer a simple default policy as an optional helper. |

## Feature Dependencies

```
[Login Phase State Machine]
    |--requires--> [Text Negotiation (Section 13 Keys)]
    |--requires--> [AuthMethod Negotiation]
    |                  |--requires--> [CHAP Implementation]
    |                  |--requires--> [Mutual CHAP]
    |                  \--requires--> [AuthMethod=None]
    \--enables--> [Full Feature Phase]

[Full Feature Phase]
    |--requires--> [Command Numbering / Windowing]
    |--requires--> [SCSI Command PDU]
    |                  |--enables--> [All SCSI Commands]
    |                  |--requires--> [Data-In PDU Handling] (for reads)
    |                  \--requires--> [Data-Out PDU Generation] (for writes)
    |                                     |--requires--> [R2T Handling]
    |                                     |--requires--> [Immediate Data]
    |                                     \--requires--> [Unsolicited Data]
    |--requires--> [NOP-Out/NOP-In]
    |--requires--> [Async Message Handling]
    |--enables--> [Task Management Functions]
    \--enables--> [Logout]

[Header/Data Digest]
    \--requires--> [CRC32C Implementation]

[Error Recovery Level 0]
    \--requires--> [Login Phase] + [Connection State Machine]

[Error Recovery Level 1]
    |--requires--> [Error Recovery Level 0]
    \--requires--> [SNACK PDU] (for data/status retransmission)

[Error Recovery Level 2]
    |--requires--> [Error Recovery Level 1]
    \--requires--> [Connection Allegiance Reassignment]

[SendTargets Discovery]
    |--requires--> [Login Phase] (discovery session)
    \--requires--> [Text Request/Response PDUs]

[High-Level SCSI API]
    |--requires--> [Raw CDB Pass-Through]
    |--requires--> [Sense Data Parsing]
    \--requires--> [INQUIRY, READ CAPACITY, READ, WRITE, etc.]

[PERSISTENT RESERVE IN/OUT]
    \--requires--> [Full Feature Phase] + [Raw CDB or typed wrapper]

[UNMAP / WRITE SAME]
    \--requires--> [VPD Page Parsing (Block Limits 0xB0)]
```

### Dependency Notes

- **Login phase is the critical path:** Nothing works without a successful login. This must be rock-solid first.
- **Data-Out path is the most complex:** Writes involve R2T tracking, unsolicited data, immediate data, and buffer management. Reads are simpler (Data-In is target-driven).
- **ERL levels are strictly layered:** ERL 1 requires ERL 0 infrastructure; ERL 2 requires ERL 1. Cannot skip levels.
- **High-level API requires raw CDB:** The typed API is built on top of CDB pass-through, not alongside it.
- **VPD pages gate advanced features:** UNMAP and WRITE SAME availability depends on querying block limits VPD page first.

## MVP Definition

### Launch With (v1.0)

Minimum to be a usable, spec-compliant iSCSI initiator library.

- [ ] Login phase with full text negotiation (all Section 13 keys) -- nothing works without this
- [ ] AuthMethod=None and CHAP (mutual CHAP included) -- spec compliance and real-world auth
- [ ] SCSI Command/Response PDU transport -- core purpose of the library
- [ ] Data-In handling (reads) -- half of all I/O
- [ ] Data-Out with R2T, immediate data, unsolicited data (writes) -- other half of all I/O
- [ ] Header and data digest (CRC32C) -- spec mandatory negotiation support
- [ ] NOP-Out/NOP-In keepalive -- connection health
- [ ] Task management (ABORT TASK, ABORT TASK SET, LUN RESET, TARGET WARM/COLD RESET, CLEAR TASK SET) -- spec mandatory
- [ ] Async message handling -- spec mandatory
- [ ] Logout (normal + recovery) -- clean teardown
- [ ] Error Recovery Level 0 -- session-level recovery; minimum viable error handling
- [ ] SendTargets discovery -- target enumeration
- [ ] Command numbering and windowing -- flow control correctness
- [ ] Raw CDB pass-through API -- escape hatch for any SCSI command
- [ ] Core SCSI commands: TEST UNIT READY, INQUIRY (std + VPD 0x00/0x80/0x83), READ CAPACITY 10/16, READ 10/16, WRITE 10/16, REQUEST SENSE, REPORT LUNS, MODE SENSE 6/10
- [ ] Structured sense data parsing -- usable error reporting

### Add After Validation (v1.x)

Features to add once core is proven working against real targets.

- [ ] Error Recovery Level 1 -- when users report PDU loss scenarios in production
- [ ] Error Recovery Level 2 -- when users need connection-level recovery without session loss
- [ ] SYNCHRONIZE CACHE 10/16 -- data integrity for write-heavy workloads
- [ ] WRITE SAME 10/16 -- efficient zeroing and provisioning
- [ ] UNMAP -- thin provisioning support (increasingly expected)
- [ ] VERIFY 10/16 -- integrity checking workflows
- [ ] START STOP UNIT -- device lifecycle
- [ ] PERSISTENT RESERVE IN/OUT -- clustering/shared-storage use cases
- [ ] COMPARE AND WRITE -- distributed locking
- [ ] Extended VPD page parsing (0xB0 block limits, 0xB1 characteristics, 0xB2 provisioning)
- [ ] Connection-level statistics and observability hooks
- [ ] PREVENT ALLOW MEDIUM REMOVAL -- media management

### Future Consideration (v2+)

- [ ] Multiple connections per session (MC/S) -- if real demand materializes
- [ ] RFC 7144 features (QUERY TASK, QUERY TASK SET, I_T NEXUS RESET, QUERY ASYNC EVENT, iSCSIProtocolLevel negotiation) -- SAM-4/5 alignment
- [ ] MODE SELECT 6/10 -- device parameter modification (rarely needed by initiators)
- [ ] READ/WRITE 6 -- legacy CDB format (12-byte LBA limit); include only if ancient target compat needed
- [ ] PREFETCH 10/16 -- cache hints
- [ ] SANITIZE -- secure erase
- [ ] EXTENDED COPY / RECEIVE COPY RESULTS -- third-party copy (offloaded data transfer)
- [ ] READ DEFECT DATA -- health monitoring
- [ ] iSNS discovery -- if enterprise deployment feedback demands it

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Login phase + text negotiation | HIGH | HIGH | P1 |
| CHAP / Mutual CHAP / None auth | HIGH | MEDIUM | P1 |
| SCSI Command PDU transport | HIGH | HIGH | P1 |
| Data-In (read path) | HIGH | MEDIUM | P1 |
| Data-Out + R2T (write path) | HIGH | HIGH | P1 |
| Header/Data digest CRC32C | HIGH | MEDIUM | P1 |
| NOP-Out/NOP-In | HIGH | LOW | P1 |
| Task management | HIGH | HIGH | P1 |
| Async message handling | HIGH | MEDIUM | P1 |
| Logout | HIGH | MEDIUM | P1 |
| ERL 0 (session recovery) | HIGH | HIGH | P1 |
| SendTargets discovery | HIGH | MEDIUM | P1 |
| Command numbering/windowing | HIGH | HIGH | P1 |
| Raw CDB pass-through | HIGH | MEDIUM | P1 |
| Core SCSI commands (inquiry, read, write, etc.) | HIGH | MEDIUM | P1 |
| Sense data parsing | HIGH | MEDIUM | P1 |
| Typed high-level Go API | HIGH | MEDIUM | P1 |
| ERL 1 (PDU retransmission) | MEDIUM | HIGH | P2 |
| ERL 2 (connection recovery) | MEDIUM | VERY HIGH | P2 |
| SYNC CACHE / WRITE SAME / UNMAP | MEDIUM | LOW | P2 |
| PERSISTENT RESERVE IN/OUT | MEDIUM | HIGH | P2 |
| COMPARE AND WRITE | MEDIUM | MEDIUM | P2 |
| VPD page parsing (0xB0/B1/B2) | MEDIUM | MEDIUM | P2 |
| Connection statistics | MEDIUM | LOW | P2 |
| MC/S | LOW | VERY HIGH | P3 |
| RFC 7144 TMFs | LOW | MEDIUM | P3 |
| SANITIZE / EXTENDED COPY | LOW | HIGH | P3 |
| iSNS discovery | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch -- spec compliance or basic usability
- P2: Should have, add after core is validated
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | libiscsi (C, userspace) | open-iscsi (C, kernel) | go wrappers (goiscsi, csi-lib-iscsi) | uiscsi (our approach) |
|---------|------------------------|------------------------|--------------------------------------|-----------------------|
| Language / runtime | C, any platform with libc | C, Linux kernel module | Go, but shell out to iscsiadm | Pure Go, no C, no kernel |
| API style | Sync + async (callbacks) | CLI tools (iscsiadm) | Go wrappers around CLI | Native Go (channels, context, io interfaces) |
| Auth (CHAP) | Yes | Yes | Delegates to open-iscsi | Yes, native implementation |
| Error recovery levels | ERL 0 primarily | ERL 0-2 via kernel | Delegates to open-iscsi | ERL 0 (v1), 1-2 (v1.x) |
| SCSI command library | Extensive (50+ commands) | N/A (kernel handles SCSI) | None (raw block device) | Core commands (v1) + extensible via raw CDB |
| Digests (CRC32C) | Yes | Yes | Delegates | Yes |
| Discovery | SendTargets | SendTargets + iSNS | Delegates | SendTargets |
| MC/S | No | Yes | Delegates | No (v1), maybe v2 |
| Platform | Cross-platform | Linux only | Linux only (needs iscsiadm) | Anywhere Go compiles |
| Embeddable in Go apps | Via CGo (pain) | No (kernel dependency) | Sort of (shell exec) | Native import |
| Async I/O model | C callbacks | Kernel async | Synchronous CLI calls | Go goroutines + channels |

## Sources

- [RFC 7143 - iSCSI Protocol (Consolidated)](https://datatracker.ietf.org/doc/html/rfc7143) -- PRIMARY source for all MUST/SHOULD/MAY requirements
- [RFC 7144 - iSCSI SCSI Features Update](https://www.rfc-editor.org/rfc/rfc7144.html) -- Additional TMFs and features for SAM-4/5
- [libiscsi - GitHub](https://github.com/sahlberg/libiscsi) -- Primary userspace competitor; SCSI command coverage baseline
- [open-iscsi - GitHub](https://github.com/open-iscsi/open-iscsi) -- Kernel-based initiator; feature coverage baseline
- [UNH IOL iSCSI Initiator FFP Test Suite](https://www.iol.unh.edu/sites/default/files/testsuites/iscsi/initiator_ffp_v1.3.pdf) -- Conformance test structure reference
- [UNH IOL iSCSI Test Plans](https://www.iol.unh.edu/testing/storage/iscsi/test-plans) -- Full test plan listing
- [goiscsi - Dell](https://github.com/dell/goiscsi) -- Go wrapper (not pure userspace)
- [csi-lib-iscsi - Kubernetes](https://pkg.go.dev/github.com/kubernetes-csi/csi-lib-iscsi) -- CSI helper (wraps iscsiadm)
- [gotgt - Go iSCSI target](https://github.com/gostor/gotgt) -- Potential test target for integration testing

---
*Feature research for: Pure-userspace iSCSI initiator library in Go*
*Researched: 2026-03-31*
