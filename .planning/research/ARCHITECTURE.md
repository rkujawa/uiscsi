# Architecture Research

**Domain:** Pure-userspace iSCSI initiator library (RFC 7143)
**Researched:** 2026-03-31
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
+-----------------------------------------------------------------------+
|                         Public API Layer                               |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | High-Level API    |  | Low-Level API     |  | Discovery API     |  |
|  | (ReadBlocks,      |  | (Raw CDB          |  | (SendTargets,     |  |
|  |  WriteBlocks,     |  |  pass-through)    |  |  target enum)     |  |
|  |  Inquiry, etc.)   |  |                   |  |                   |  |
|  +--------+----------+  +--------+----------+  +--------+----------+  |
|           |                      |                      |             |
+-----------+----------------------+----------------------+-------------+
|                         SCSI Layer                                    |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | CDB Builder       |  | Response Parser   |  | Task Manager      |  |
|  | (SPC + SBC        |  | (sense data,      |  | (ABORT, LUN       |  |
|  |  commands)        |  |  status codes)    |  |  RESET, etc.)     |  |
|  +--------+----------+  +--------+----------+  +--------+----------+  |
|           |                      |                      |             |
+-----------+----------------------+----------------------+-------------+
|                       Session Layer                                   |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | Session Manager   |  | Command Sequencer |  | Error Recovery    |  |
|  | (TSIH, ISID,      |  | (CmdSN, ExpCmdSN, |  | (ERL 0/1/2,      |  |
|  |  state machine)   |  |  windowing)       |  |  task reassign)   |  |
|  +--------+----------+  +--------+----------+  +--------+----------+  |
|           |                      |                      |             |
+-----------+----------------------+----------------------+-------------+
|                      Connection Layer                                 |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | Connection FSM    |  | Login Negotiator  |  | Text Negotiator   |  |
|  | (state machine,   |  | (auth, params,    |  | (key=value        |  |
|  |  StatSN tracking) |  |  CHAP, phases)    |  |  exchange)        |  |
|  +--------+----------+  +--------+----------+  +--------+----------+  |
|           |                      |                      |             |
+-----------+----------------------+----------------------+-------------+
|                        PDU Layer                                      |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | PDU Codec         |  | Digest Engine     |  | PDU Router        |  |
|  | (BHS encode/      |  | (CRC32C header +  |  | (opcode dispatch, |  |
|  |  decode, AHS,     |  |  data digest)     |  |  ITT correlation) |  |
|  |  data segment)    |  |                   |  |                   |  |
|  +--------+----------+  +--------+----------+  +--------+----------+  |
|           |                      |                      |             |
+-----------+----------------------+----------------------+-------------+
|                      Transport Layer                                  |
|  +-------------------+  +-------------------+  +-------------------+  |
|  | TCP Transport     |  | Read Pump         |  | Write Pump        |  |
|  | (net.Conn,        |  | (goroutine:       |  | (goroutine:       |  |
|  |  dial, TLS)       |  |  read + frame)    |  |  serialize +      |  |
|  |                   |  |                   |  |  write)           |  |
|  +-------------------+  +-------------------+  +-------------------+  |
+-----------------------------------------------------------------------+
```

### Component Responsibilities

| Component | Responsibility | Boundary |
|-----------|----------------|----------|
| **High-Level API** | Typed Go functions for common SCSI operations (Inquiry, ReadCapacity, Read/Write blocks) | Accepts Go types, returns Go types. Translates to/from CDB bytes internally. |
| **Low-Level API** | Raw CDB pass-through for power users who build their own CDB bytes | Accepts `[]byte` CDB + data buffers, returns raw response bytes |
| **Discovery API** | SendTargets discovery, target/LUN enumeration | Manages short-lived discovery sessions, returns structured target info |
| **CDB Builder** | Constructs SCSI CDB bytes for SPC (Inquiry, TestUnitReady, ReportLUNs) and SBC (Read10/16, Write10/16, ReadCapacity) | Pure functions: command params in, `[]byte` CDB out |
| **Response Parser** | Parses SCSI status, sense data, mode pages, VPD pages | Pure functions: raw bytes in, structured Go types out |
| **Task Manager** | Issues iSCSI Task Management Function Requests (ABORT TASK, LUN RESET, etc.) | Coordinates with session layer for affected task cleanup |
| **Session Manager** | Owns session state (FREE, LOGGED_IN, FAILED), ISID/TSIH pair, session-wide CmdSN window | One goroutine owns session state; communicates via channels |
| **Command Sequencer** | Assigns CmdSN to outgoing commands, tracks ExpCmdSN/MaxCmdSN from target responses, enforces command window | Serializes command submission; blocks when window is full |
| **Error Recovery** | Implements ERL 0 (session drop+reconnect), ERL 1 (PDU retransmit on digest failure), ERL 2 (connection reassignment within session) | Monitors connection health, triggers appropriate recovery strategy |
| **Connection FSM** | Per-connection state machine per RFC 7143 Section 8 (S1-S8 states for initiator) | Owns one TCP connection's lifecycle; reports state changes to session |
| **Login Negotiator** | Drives the iSCSI login phase: SecurityNegotiation + OperationalNegotiation sub-phases | Handles CHAP authentication, parameter exchange, CSG/NSG transitions |
| **Text Negotiator** | Key=value text parameter exchange (RFC 7143 Section 6.2) | Reused by both login and full-feature-phase text exchanges |
| **PDU Codec** | Encodes/decodes the 48-byte BHS, optional AHS, data segment with padding | Pure encode/decode; no I/O. Operates on byte slices and structs. |
| **Digest Engine** | CRC32C computation for header digest (4 bytes after BHS+AHS) and data digest (4 bytes after data segment) | Negotiation-aware: only computes when negotiated. Uses `hash/crc32` with Castagnoli polynomial. |
| **PDU Router** | Dispatches received PDUs by opcode, correlates responses to pending commands via ITT | Central multiplexer between transport read pump and waiting command goroutines |
| **TCP Transport** | Manages `net.Conn`, TCP dial with timeouts, optional TLS wrapping | Provides `io.ReadWriteCloser` to read/write pumps |
| **Read Pump** | Dedicated goroutine: reads from TCP, frames complete PDUs (BHS length fields determine framing), passes to PDU router | Runs for connection lifetime; signals connection errors |
| **Write Pump** | Dedicated goroutine: serializes PDUs from a channel, writes to TCP in order | Ensures PDU-level write atomicity; no interleaved writes |

## Recommended Project Structure

```
iscsi/
    iscsi.go              # Package doc, top-level Session/Target types
    session.go            # Session state machine, CmdSN management
    connection.go         # Connection FSM, read/write pump orchestration
    login.go              # Login phase negotiation, CSG/NSG transitions
    auth.go               # CHAP and mutual CHAP authentication
    negotiate.go          # Text key=value parameter negotiation engine
    discovery.go          # SendTargets discovery sessions
    error_recovery.go     # ERL 0/1/2 implementation
    task.go               # Task management function requests
    pdu/
        pdu.go            # PDU struct, BHS field definitions, opcode constants
        encode.go         # PDU serialization (struct -> bytes)
        decode.go         # PDU deserialization (bytes -> struct)
        opcodes.go        # Opcode constants and names
        keys.go           # Negotiation key constants (RFC 7143 Section 13)
    digest/
        digest.go         # CRC32C header/data digest computation
    scsi/
        cdb.go            # CDB builder functions (Read10, Write10, Inquiry, etc.)
        sense.go          # Sense data parser
        status.go         # SCSI status codes
        types.go          # InquiryData, ReadCapacityData, etc.
    transport/
        tcp.go            # TCP connection management, dialing
        tls.go            # TLS wrapping for IPsec-less security
    internal/
        sequence.go       # CmdSN/StatSN/ExpStatSN sequence number helpers
        pool.go           # Buffer and PDU object pools
    highlevel/
        block.go          # ReadBlocks, WriteBlocks convenience functions
        inquiry.go        # Inquiry, TestUnitReady, ReadCapacity wrappers
        lun.go            # LUN discovery and management helpers
```

### Structure Rationale

- **`iscsi/` (root package):** The primary package users import. Contains session/connection lifecycle — the core protocol engine. Keeps the import path clean: `import "github.com/.../iscsi"`.
- **`pdu/`:** Isolated PDU codec with zero I/O dependencies. Pure encode/decode makes it trivially testable with table-driven tests against known byte sequences from the RFC.
- **`digest/`:** Separated because digest computation is optional (negotiated) and has a clear single responsibility. Also allows benchmarking independently.
- **`scsi/`:** SCSI-layer concerns (CDB construction, sense parsing) are protocol-independent — they could theoretically work with any SCSI transport, not just iSCSI.
- **`transport/`:** Abstracts the TCP connection behind an interface, enabling mock transports for testing and potential future transports.
- **`internal/`:** Shared utilities not part of the public API. Sequence number arithmetic, buffer pooling.
- **`highlevel/`:** Convenience wrappers that compose the low-level API. Separate package keeps the core lean and prevents circular dependencies.

## Architectural Patterns

### Pattern 1: Goroutine-per-Connection with Channel Multiplexing

**What:** Each iSCSI connection runs two dedicated goroutines (read pump, write pump) plus the connection manager goroutine. Pending commands register with the PDU router and wait on per-command channels for their response PDUs.

**When to use:** Always — this is the fundamental concurrency model for the library.

**Trade-offs:** Clean separation of read/write paths; natural backpressure via channel capacity. Slight overhead per connection from goroutines, but iSCSI sessions typically have one connection (and this project explicitly scopes to single-connection sessions for v1).

**Example:**
```go
// Simplified connection run loop
func (c *Connection) run(ctx context.Context) error {
    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error { return c.readPump(ctx) })
    g.Go(func() error { return c.writePump(ctx) })

    return g.Wait() // first error cancels both pumps
}

// Read pump: frame PDUs, dispatch to router
func (c *Connection) readPump(ctx context.Context) error {
    for {
        pdu, err := c.readPDU(ctx) // reads BHS, AHS, data segment
        if err != nil {
            return err
        }
        c.router.dispatch(pdu) // correlate ITT, deliver to waiting command
    }
}
```

### Pattern 2: Explicit State Machines for Protocol Phases

**What:** Connection and session lifecycle follow RFC 7143 Section 8 state machines implemented as explicit `state` types with transition tables. Not implicit if/else chains.

**When to use:** For connection states (S1-S8 per RFC) and login sub-phases (SecurityNegotiation, OperationalNegotiation, FullFeaturePhase).

**Trade-offs:** More upfront code than ad-hoc conditionals, but dramatically easier to test (assert state transitions), debug (log current state), and verify against the RFC (1:1 mapping of state names).

**Example:**
```go
type ConnState int

const (
    S1_Free ConnState = iota // no transport connection
    S2_XptWait               // waiting for transport connection
    S4_InLogin               // login phase in progress
    S5_LoggedIn              // full feature phase
    S6_InLogout              // logout requested
    S7_Cleanup               // connection cleanup in progress
    S8_CleanupWait           // waiting for cleanup completion
)

type transition struct {
    from  ConnState
    event Event
    to    ConnState
    action func() error
}

// Table-driven transitions, directly traceable to RFC 7143 Section 8.1
var initiatorTransitions = []transition{
    {S1_Free, EvtConnect, S2_XptWait, nil},
    {S2_XptWait, EvtXptReady, S4_InLogin, startLogin},
    {S4_InLogin, EvtLoginSuccess, S5_LoggedIn, nil},
    {S5_LoggedIn, EvtLogoutReq, S6_InLogout, sendLogout},
    // ... per RFC 7143 Section 8.1.3
}
```

### Pattern 3: ITT-Based Command Correlation

**What:** Each outgoing SCSI command or task management request is assigned a unique Initiator Task Tag (ITT). The PDU router maintains a `map[uint32]chan *PDU` to deliver response PDUs to the goroutine waiting on that specific command.

**When to use:** All command/response correlation in full feature phase.

**Trade-offs:** Simple, O(1) lookup. Must handle ITT exhaustion (32-bit space is huge but must handle wrap), and must clean up entries on timeout/abort. Lock contention on the map is the main concern — use `sync.Mutex`, not `sync.RWMutex` (writes are as frequent as reads).

**Example:**
```go
type Router struct {
    mu       sync.Mutex
    pending  map[uint32]chan<- *pdu.PDU // ITT -> response channel
    nextITT  uint32
}

func (r *Router) Register() (itt uint32, ch <-chan *pdu.PDU) {
    r.mu.Lock()
    defer r.mu.Unlock()
    itt = r.nextITT
    r.nextITT++
    resp := make(chan *pdu.PDU, 1) // buffered: router never blocks
    r.pending[itt] = resp
    return itt, resp
}

func (r *Router) Dispatch(p *pdu.PDU) {
    r.mu.Lock()
    ch, ok := r.pending[p.ITT()]
    if ok {
        delete(r.pending, p.ITT())
    }
    r.mu.Unlock()
    if ok {
        ch <- p
    }
}
```

### Pattern 4: Layered Negotiation Engine

**What:** Text key=value negotiation (RFC 7143 Section 6.2) is implemented as a reusable engine used by both login negotiation and full-feature-phase text exchanges. The engine handles multi-PDU exchanges (C-bit continuation), declarative vs negotiated keys, and irrelevant/reject responses.

**When to use:** Login phase parameter negotiation and any full-feature-phase text operations.

**Trade-offs:** Slight over-engineering if text exchanges are rare in practice, but login always uses it and the RFC mandates the capability.

## Data Flow

### SCSI Read Command Flow

```
Application
    |
    | ReadBlocks(ctx, lun, lba, count)
    v
High-Level API
    |
    | builds Read10/Read16 CDB, calculates transfer length
    v
Session Layer
    |
    | assigns CmdSN, selects connection, registers ITT
    v
Connection Layer
    |
    | creates SCSI Command PDU (opcode 0x01), sets R bit (read)
    v
PDU Codec
    |
    | encodes BHS (48 bytes) + optional header digest
    v
Write Pump -----> TCP -----> iSCSI Target
                                  |
                                  | processes command, reads from storage
                                  v
Read Pump <----- TCP <----- SCSI Data-In PDU(s) (opcode 0x25)
    |                        (one or more, F-bit on final)
    v
PDU Router
    |
    | correlates ITT, delivers to waiting goroutine
    v
PDU Codec
    |
    | decodes, verifies digests, extracts data segment
    v
Command Goroutine
    |
    | accumulates data segments, checks status
    v
High-Level API
    |
    | returns []byte data to application
    v
Application
```

### SCSI Write Command Flow

```
Application
    |
    | WriteBlocks(ctx, lun, lba, data)
    v
High-Level API
    |
    | builds Write10/Write16 CDB
    v
Session Layer
    |
    | assigns CmdSN, registers ITT
    v
Connection Layer
    |
    | creates SCSI Command PDU (opcode 0x01), sets W bit (write)
    | if ImmediateData=Yes: attaches first chunk to command PDU
    | if InitialR2T=No: sends unsolicited Data-Out PDUs
    v
Write Pump -----> TCP -----> iSCSI Target
                                  |
Read Pump <----- TCP <----- R2T PDU (opcode 0x31)
    |                        (target requests more data)
    v
PDU Router
    |
    | delivers R2T to waiting command goroutine
    v
Command Goroutine
    |
    | sends solicited Data-Out PDUs (opcode 0x05)
    | for the requested offset+length
    v
Write Pump -----> TCP -----> iSCSI Target
                                  |
Read Pump <----- TCP <----- SCSI Response PDU (opcode 0x21)
    |                        (status = GOOD)
    v
Application
```

### Login Phase Flow

```
Initiator                                      Target
    |                                              |
    |--- Login Request (CSG=Security, NSG=Op) ---->|
    |    InitiatorName, AuthMethod=CHAP,None        |
    |                                              |
    |<-- Login Response (CSG=Security) ------------|
    |    AuthMethod=CHAP                           |
    |    CHAP_A=5 (MD5)                            |
    |                                              |
    |--- Login Request (CSG=Security) ------------>|
    |    CHAP_N=initiator, CHAP_R=<response>       |
    |    (+ mutual CHAP_I, CHAP_C if mutual)       |
    |                                              |
    |<-- Login Response (T=1, CSG=Sec, NSG=Op) ---|
    |    (auth success, transition to Op phase)    |
    |                                              |
    |--- Login Request (CSG=Op, NSG=FFP) -------->|
    |    MaxRecvDataSegmentLength, etc.            |
    |                                              |
    |<-- Login Response (T=1, CSG=Op, NSG=FFP) ---|
    |    negotiated parameters                     |
    |                                              |
    |=== Full Feature Phase established ===========|
```

### Key Data Flows

1. **Command submission:** Application -> High-Level API -> SCSI Layer (CDB build) -> Session (CmdSN assign) -> Connection (PDU create) -> PDU Codec (serialize) -> Write Pump (TCP send). Response flows in reverse via Read Pump -> PDU Router -> waiting goroutine.

2. **Error recovery (ERL 0):** Connection detects TCP error -> Connection FSM transitions to cleanup state -> Session Manager notified -> Session drops to FAILED -> reconnects new TCP connection -> re-runs login -> re-issues any commands that were in flight (their ITT registrations were preserved with timeout tracking).

3. **NOP ping/pong:** Session layer sends periodic NOP-Out PDUs (with ITT=0xFFFFFFFF for unsolicited, or real ITT for solicited) -> target responds with NOP-In. Failure to receive NOP-In within timeout triggers connection error -> error recovery.

## Scaling Considerations

| Concern | Library Context |
|---------|-----------------|
| Concurrent commands | CmdSN window limits outstanding commands (typically 32-128). Command Sequencer handles backpressure naturally. |
| Large transfers | MaxRecvDataSegmentLength negotiation (typically 8KB-1MB). Data-In/Out segmented into PDUs. Use buffer pooling to avoid GC pressure. |
| Multiple targets | Each target = separate Session. Sessions are independent. No shared state except optional connection pooling at transport layer. |
| Multiple LUNs | Handled within a single session. LUN is a field in the SCSI Command PDU. No architectural impact. |
| Throughput | Read/write pump goroutines are the bottleneck. Minimize allocations in hot path. Pre-allocate BHS buffers. Use `io.ReadFull` for exact reads. |

### Performance-Critical Decisions

1. **Buffer pooling:** `sync.Pool` for PDU buffers and data segment buffers. The read pump allocates on every PDU receive — pooling is essential.
2. **Zero-copy where possible:** Data segments can be sliced from larger buffers rather than copied. The PDU codec should accept `io.Reader`/`io.Writer` to avoid intermediate allocations.
3. **CRC32C:** Go's `hash/crc32` package with `crc32.MakeTable(crc32.Castagnoli)` uses hardware acceleration (SSE4.2 on amd64) when available. Do not hand-roll.

## Anti-Patterns

### Anti-Pattern 1: Monolithic Connection Handler

**What people do:** Put login, full-feature-phase, error recovery, and negotiation all in one giant function with phase tracking via booleans.
**Why it's wrong:** iSCSI has well-defined state machines (RFC 7143 Section 8). Monolithic handlers make it impossible to verify state transition correctness, and bugs manifest as protocol violations that are hard to reproduce.
**Do this instead:** Explicit state machine with transition table. Each state has a handler function. Transitions are logged and testable.

### Anti-Pattern 2: Synchronous PDU Read/Write in Command Goroutine

**What people do:** Each SCSI command goroutine directly reads/writes the TCP connection.
**Why it's wrong:** Interleaved writes corrupt PDU framing. Interleaved reads steal PDUs meant for other commands. TCP is a byte stream, not a message stream.
**Do this instead:** Dedicated read pump and write pump goroutines per connection. Commands send PDUs via a write channel and receive responses via ITT-correlated channels.

### Anti-Pattern 3: Ignoring Command Window

**What people do:** Fire SCSI commands without tracking CmdSN/ExpCmdSN/MaxCmdSN.
**Why it's wrong:** Target will reject commands outside the CmdSN window. Worse, some targets silently drop them, causing mysterious timeouts.
**Do this instead:** Command Sequencer tracks the window. Block or return an error when the window is full. Update window on every received PDU (ExpCmdSN and MaxCmdSN are in every target PDU).

### Anti-Pattern 4: String-Based Negotiation Without Validation

**What people do:** Build login key=value pairs with string concatenation, parse responses with string splitting.
**Why it's wrong:** RFC 7143 Section 13 has strict rules: some keys are declarative, some are negotiated (minimum, maximum, OR, AND semantics), some have restricted values. String manipulation misses validation.
**Do this instead:** Typed negotiation engine where each key has a defined type (boolean AND/OR, numerical min/max, string list), and the engine validates both proposals and responses.

### Anti-Pattern 5: Treating Error Recovery as an Afterthought

**What people do:** Build the happy path first, plan to "add error recovery later."
**Why it's wrong:** Error recovery (especially ERL 1 and 2) requires deep integration with the command pipeline, PDU sequencing, and session state. Retrofitting it means rewriting the core.
**Do this instead:** Design the command tracking, ITT management, and session state from the start with error recovery in mind. ERL 0 (session reconnect) should work from day one. ERL 1/2 add to existing infrastructure rather than replacing it.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| iSCSI Target (SAN) | TCP connection on port 3260 (default) | Standard iSCSI port. Must handle targets on non-standard ports. |
| DNS | Target portal resolution | Resolve target portal hostnames before connecting. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Application <-> Session | Method calls + context.Context | Session.Execute(ctx, cdb, data) is the main entry point. Context carries deadlines and cancellation. |
| Session <-> Connection | Channels | Session sends command PDUs via channel; connection delivers responses via ITT channels. |
| Connection <-> PDU Codec | Function calls | Synchronous encode/decode. No goroutines in the codec. |
| Connection <-> Transport | io.ReadWriteCloser | Transport abstracted behind interface for testability. |
| PDU Router <-> Command goroutines | Per-ITT channels | Each command gets a buffered channel; router delivers exactly one response (or timeout). |

### Testability Boundaries

| Layer | Test Strategy |
|-------|---------------|
| PDU Codec | Table-driven: known byte sequences from RFC examples -> struct, and back |
| Digest Engine | Table-driven: known inputs -> known CRC32C outputs |
| CDB Builder | Table-driven: command params -> expected CDB bytes |
| State Machines | Transition tests: event sequences -> expected state sequences |
| Login Negotiator | Mock transport: scripted PDU exchanges from RFC examples |
| Session/Connection | Mock transport + mock target: full command flow integration tests |
| High-Level API | Integration tests against gotgt or embedded minimal target |

## Build Order (Dependency Chain)

The layers must be built bottom-up because upper layers depend on lower layers:

```
Phase 1: PDU Codec + Digest Engine + Transport Abstraction
         (no dependencies on each other; can be built in parallel)
              |
Phase 2: Connection Layer (read/write pumps, PDU router, connection FSM)
         (depends on: PDU codec, transport)
              |
Phase 3: Login Negotiator + Text Negotiation Engine + Auth (CHAP)
         (depends on: connection layer, PDU codec)
              |
Phase 4: Session Layer (session state machine, CmdSN sequencing)
         (depends on: connection layer, login negotiator)
              |
Phase 5: SCSI Layer (CDB builders, response parsers)
         (independent of session layer; can be built earlier)
              |
Phase 6: Low-Level API (raw CDB pass-through)
         (depends on: session layer, SCSI layer)
              |
Phase 7: High-Level API (typed convenience functions)
         (depends on: low-level API, SCSI layer)
              |
Phase 8: Discovery, Task Management, Error Recovery (ERL 1/2)
         (depends on: session layer, can be added incrementally)
```

**Key insight:** The SCSI layer (CDB builders, sense parsers) has zero dependency on the iSCSI session/connection layers. It can be built and tested independently at any point, even in Phase 1. This is a natural parallelization opportunity.

**ERL 0 should be part of Phase 4**, not Phase 8 — session-level reconnection is the baseline error handling and must be present from the start. ERL 1 and 2 build on top of the existing infrastructure and can be added later.

## Sources

- [RFC 7143 - iSCSI Protocol (Consolidated)](https://www.rfc-editor.org/rfc/rfc7143.html) - Authoritative specification. Sections 8 (state machines), 11 (PDU formats), 6 (login/negotiation), 7 (error recovery), 13 (negotiation keys).
- [RFC 7144 - iSCSI SCSI Features Update](https://www.rfc-editor.org/rfc/rfc7144.html) - SCSI feature clarifications for iSCSI.
- [RFC 3385 - iSCSI CRC/Checksum Considerations](https://www.rfc-editor.org/rfc/rfc3385) - CRC32C rationale and implementation guidance.
- [libiscsi (sahlberg)](https://github.com/sahlberg/libiscsi) - Reference C implementation: async/sync layered design, conformance test suite structure.
- [gotgt](https://github.com/gostor/gotgt) - Go iSCSI target: demonstrates Go iSCSI PDU handling, SCSI layer separation, object pooling patterns.
- [open-iscsi](https://github.com/open-iscsi/open-iscsi) - Linux iSCSI: control plane (userspace) / data plane (kernel) separation; login/negotiation in userspace.
- [Go Networking Internals](https://goperf.dev/02-networking/networking-internals/) - Go netpoller, goroutine-per-connection patterns.
- [Go State Machine Patterns](https://medium.com/@johnsiilver/go-state-machine-patterns-3b667f345b5e) - Table-driven state machine implementation in Go.

---
*Architecture research for: pure-userspace iSCSI initiator library (RFC 7143)*
*Researched: 2026-03-31*
