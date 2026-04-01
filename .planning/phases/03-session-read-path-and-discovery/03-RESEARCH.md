# Phase 3: Session, Read Path, and Discovery - Research

**Researched:** 2026-03-31
**Domain:** iSCSI session layer, SCSI command dispatch, Data-In reassembly, NOP keepalive, SendTargets discovery, logout
**Confidence:** HIGH

## Summary

Phase 3 builds the session layer on top of the authenticated connection from Phase 2. The session manages CmdSN/MaxCmdSN command windowing, dispatches SCSI commands via an async Submit+Channel model, reassembles Data-In PDUs into io.Reader streams, runs NOP-Out/NOP-In keepalive, implements SendTargets discovery, and handles graceful logout plus async messages from the target.

All PDU types needed are already implemented (SCSICommand, SCSIResponse, DataIn, NOPOut, NOPIn, TextReq, TextResp, LogoutReq, LogoutResp, AsyncMsg). The transport layer (ReadPump, WritePump, Router) provides full-duplex PDU framing with ITT-based dispatch and an unsolicited PDU channel. The login package provides NegotiatedParams and text key-value encoding/decoding. Serial number arithmetic (serial.InWindow, serial.Incr) is ready for CmdSN/DataSN validation.

**Primary recommendation:** Create `internal/session/` package with Session struct wrapping transport.Conn. Session owns ReadPump/WritePump lifecycle, maintains CmdSN/StatSN counters, and provides Submit(ctx, cmd) returning a channel-based Result with io.Reader for data and SCSI status. Discovery and Logout are methods on Session (plus a standalone Discover convenience function).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Session wraps Conn -- `NewSession(conn, params, opts...)` takes ownership of the transport.Conn and NegotiatedParams. Caller does Dial -> Login -> NewSession as three distinct steps, consistent with D-02 from Phase 2.
- **D-02:** NegotiatedParams exposed via read-only accessor `session.Params()` -- callers can inspect negotiated values (MaxBurstLength, digests, etc.) but not modify them.
- **D-03:** NewSession auto-starts ReadPump/WritePump -- session is ready for commands immediately after creation. No separate Start() call needed.
- **D-04:** Async Submit+Channel model -- `session.Submit(ctx, cmd)` returns a `<-chan Result` (or similar). Callers can have multiple commands in flight up to the CmdSN window. CmdSN/MaxCmdSN windowing handled internally.
- **D-05:** Data-In delivered as `io.Reader` -- streaming assembly of multi-PDU reads. Higher-level APIs (ReadBlocks, Inquiry in future phases) consume the Reader internally and return typed results. Lower memory pressure for large transfers, Go-idiomatic.
- **D-06:** Both standalone function and Session method -- `Discover(ctx, addr, opts...)` convenience function (Dial+Login+SendTargets+Logout in one call) and `session.SendTargets(ctx)` for power users with existing discovery sessions.
- **D-07:** Structured `DiscoveryTarget` return type -- `type DiscoveryTarget struct { Name string; Portals []Portal }` with `Portal` containing Address and Port. Parsed from text key-value response.
- **D-08:** Automatic background keepalive -- Session runs a goroutine sending NOP-Out at configurable interval (default 30s). Timeout triggers session error. No caller action needed.
- **D-09:** Async events via callback -- `WithAsyncHandler(func(AsyncEvent))` functional option. Called on dedicated goroutine when target sends AsyncMsg PDU.
- **D-10:** Target-requested logout triggers auto-logout + notify -- Session initiates graceful logout automatically per RFC 7143 Time2Wait+Time2Retain, then calls async handler to inform caller.

### Claude's Discretion
- Internal session state machine design
- CmdSN window tracking data structure (ring buffer, sync.Cond, semaphore, etc.)
- Data-In reassembly buffer management
- How the Result type is structured (struct with Reader + Status + SenseData, etc.)
- Internal package organization (internal/session/ vs extending internal/login/)
- NOP-Out TTT handling for unsolicited vs solicited NOP exchanges
- Logout PDU exchange sequencing
- SendTargets text response parsing implementation

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SESS-01 | Session state machine per RFC 7143 connection/session model | Architecture patterns: session states (Logged-In, LoggingOut, Closed), state transitions driven by PDU events |
| SESS-02 | CmdSN/ExpCmdSN/MaxCmdSN command windowing and flow control | CmdSN window section: semaphore-based throttling, serial number arithmetic for window bounds |
| SESS-03 | StatSN/ExpStatSN tracking per connection | StatSN tracking pattern: connection-scoped counter updated from every target response PDU |
| SESS-04 | SCSI Command PDU generation with proper CDB encapsulation | SCSICommand PDU already implemented; Submit builds it with allocated ITT and current CmdSN |
| SESS-05 | NOP-Out/NOP-In keepalive (initiator-originated and target-initiated response) | NOP semantics section: TTT rules, immediate bit, dual-direction handling |
| READ-01 | Data-In PDU handling with sequence number validation and data offset tracking | Data-In reassembly pattern: DataSN monotonic check, BufferOffset validation |
| READ-02 | Multi-PDU read reassembly (gathering Data-In PDUs into complete read response) | io.Reader streaming pattern: pipe-based assembly, goroutine writes data as PDUs arrive |
| READ-03 | Status delivery via Data-In with S-bit or separate SCSI Response PDU | Dual status delivery: S-bit on final Data-In vs separate SCSIResponse PDU |
| EVT-01 | Async message handling (SCSI async event, target-requested logout, connection/session drop notification, vendor-specific) | Async event codes 0-5, 255; callback dispatch on dedicated goroutine |
| EVT-02 | Logout (normal session/connection teardown) | Logout PDU exchange: reason code 0, Time2Wait/Time2Retain handling |
| EVT-03 | Logout for connection recovery (remove connection for recovery) | Logout reason code 2; deferred to Phase 6 ERL but PDU exchange implemented here |
| DISC-01 | SendTargets discovery (discovery session type, text request/response for target enumeration) | Discovery pattern: SessionType=Discovery login, SendTargets=All text request |
| DISC-02 | Target and LUN enumeration from discovery results | DiscoveryTarget struct parsing from TargetName + TargetAddress key-value pairs |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `sync` | 1.25 | Mutex, Cond, WaitGroup for session concurrency | CmdSN window gating, state machine protection |
| Go stdlib `context` | 1.25 | Cancellation/timeout for Submit, keepalive, logout | Already used throughout codebase |
| Go stdlib `io` | 1.25 | io.Pipe for Data-In streaming assembly | Connects PDU receiver goroutine to caller's io.Reader |
| Go stdlib `log/slog` | 1.25 | Session lifecycle logging | Established pattern from transport layer |
| Go stdlib `testing/synctest` | 1.25 | Deterministic concurrent tests for session goroutines | Virtualizes time for keepalive/timeout tests |

### Supporting (all internal)
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `internal/transport` | Conn, ReadPump, WritePump, Router, RawPDU | Session wraps Conn, starts pumps, uses Router for dispatch |
| `internal/pdu` | All PDU types, EncodePDU, DecodeBHS | Build/parse SCSICommand, DataIn, NOPOut, NOPIn, TextReq, etc. |
| `internal/serial` | LessThan, GreaterThan, InWindow, Incr | CmdSN window bounds, DataSN validation |
| `internal/login` | NegotiatedParams, EncodeTextKV, DecodeTextKV, Login, WithSessionType | Session config source, SendTargets text encoding, discovery login |

No external dependencies needed. This phase is 100% stdlib + internal packages.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  session/
    session.go       # Session struct, NewSession, Submit, Close
    cmdwindow.go     # CmdSN/MaxCmdSN windowing logic
    datain.go        # Data-In reassembly, io.Reader streaming
    keepalive.go     # NOP-Out/NOP-In background goroutine
    logout.go        # Logout PDU exchange, graceful teardown
    discovery.go     # SendTargets, Discover convenience function
    async.go         # AsyncMsg handling, event dispatch
    types.go         # Result, AsyncEvent, DiscoveryTarget, Portal types
    session_test.go  # Integration tests with mock target
    cmdwindow_test.go
    datain_test.go
    keepalive_test.go
    logout_test.go
    discovery_test.go
```

### Pattern 1: Session Goroutine Architecture
**What:** Session owns three background goroutines: ReadPump (receives PDUs), WritePump (serializes outgoing PDUs), and keepalive (periodic NOP-Out). An additional dispatcher goroutine handles unsolicited PDUs from the ReadPump's unsolicitedCh.
**When to use:** Always -- this is the core session architecture.
**Example:**
```go
// Session manages an iSCSI Full Feature Phase session.
type Session struct {
    conn      *transport.Conn
    params    login.NegotiatedParams
    router    *transport.Router
    writeCh   chan *transport.RawPDU
    unsolCh   chan *transport.RawPDU // unsolicited PDU channel

    // Sequence numbers (protected by mu)
    mu        sync.Mutex
    cmdSN     uint32
    maxCmdSN  uint32
    expStatSN uint32

    // CmdSN window gating
    windowCond *sync.Cond // or use a semaphore channel

    // Task tracking: ITT -> *task for multi-PDU reassembly
    tasks     map[uint32]*task

    // Lifecycle
    cancel    context.CancelFunc
    done      chan struct{}
    err       error // first fatal error

    // Options
    asyncHandler func(AsyncEvent)
    keepaliveInterval time.Duration
}
```

### Pattern 2: CmdSN Command Window Gating
**What:** Before submitting a command, the initiator must verify CmdSN is within [ExpCmdSN, MaxCmdSN]. Use sync.Cond to block Submit() when the window is full, waking waiters when target responses update MaxCmdSN.
**When to use:** Every non-immediate command submission.
**RFC rule:** Queuing capacity = MaxCmdSN - ExpCmdSN + 1 (using serial number arithmetic). CmdSN advances by 1 for each non-immediate command sent. Immediate commands carry CmdSN but do NOT advance it.
**Example:**
```go
func (s *Session) acquireCmdSN(ctx context.Context) (uint32, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for !serial.InWindow(s.cmdSN, s.expCmdSN, s.maxCmdSN) {
        // Window full -- wait for target to advance MaxCmdSN
        // Use context-aware waiting
        done := make(chan struct{})
        go func() {
            s.windowCond.Wait()
            close(done)
        }()
        s.mu.Unlock()
        select {
        case <-ctx.Done():
            s.mu.Lock()
            return 0, ctx.Err()
        case <-done:
            s.mu.Lock()
        }
    }
    sn := s.cmdSN
    s.cmdSN = serial.Incr(s.cmdSN)
    return sn, nil
}

// Called when any target response PDU updates ExpCmdSN/MaxCmdSN
func (s *Session) updateWindow(expCmdSN, maxCmdSN uint32) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if serial.GreaterThan(expCmdSN, s.expCmdSN) {
        s.expCmdSN = expCmdSN
    }
    if serial.GreaterThan(maxCmdSN, s.maxCmdSN) {
        s.maxCmdSN = maxCmdSN
    }
    s.windowCond.Broadcast()
}
```

### Pattern 3: Data-In Streaming via io.Pipe
**What:** When a read command is submitted, create an io.Pipe. The reassembly goroutine writes Data-In PDU payloads to the PipeWriter as they arrive (validating DataSN and BufferOffset). The caller reads from the PipeReader. Status is delivered when S-bit Data-In or SCSIResponse arrives.
**When to use:** All SCSI read commands (READ-01, READ-02, READ-03).
**Example:**
```go
type task struct {
    itt        uint32
    pr         *io.PipeReader
    pw         *io.PipeWriter
    resultCh   chan Result
    nextDataSN uint32
    nextOffset uint32
}

// Called by dispatcher for each Data-In PDU matching this task's ITT
func (t *task) handleDataIn(din *pdu.DataIn) {
    if din.DataSN != t.nextDataSN {
        t.pw.CloseWithError(fmt.Errorf("DataSN gap: got %d want %d", din.DataSN, t.nextDataSN))
        return
    }
    if din.BufferOffset != t.nextOffset {
        t.pw.CloseWithError(fmt.Errorf("offset mismatch: got %d want %d", din.BufferOffset, t.nextOffset))
        return
    }
    t.pw.Write(din.Data)
    t.nextDataSN++
    t.nextOffset += uint32(len(din.Data))

    if din.HasStatus {
        t.pw.Close() // EOF signals reader that data is complete
        t.resultCh <- Result{Status: din.Status, Data: t.pr}
    }
}
```

### Pattern 4: NOP-Out/NOP-In Dual-Direction Keepalive
**What:** Two separate flows. (1) Initiator sends NOP-Out with TTT=0xFFFFFFFF as ping, expects NOP-In response matching ITT. (2) Target sends unsolicited NOP-In with TTT!=0xFFFFFFFF, initiator responds with NOP-Out echoing the TTT.
**RFC rule:** NOP-Out marked Immediate does NOT advance CmdSN. Unsolicited target NOP-In has ITT=0xFFFFFFFF (routed to unsolicitedCh). Initiator-ping NOP-Out uses Router.Register for response correlation.
**Example:**
```go
// Initiator ping (direction 1)
func (s *Session) sendPing(ctx context.Context) error {
    itt, ch := s.router.Register()
    nop := &pdu.NOPOut{
        Header: pdu.Header{
            Immediate:        true,
            Final:            true,
            InitiatorTaskTag: itt,
        },
        TargetTransferTag: 0xFFFFFFFF, // initiator-originated ping
        CmdSN:             s.getCmdSN(), // carry but do NOT advance
        ExpStatSN:         s.getExpStatSN(),
    }
    // ... encode and send via writeCh, wait on ch with timeout
}

// Target ping response (direction 2) -- in unsolicited handler
func (s *Session) handleUnsolicitedNOPIn(nopin *pdu.NOPIn) {
    if nopin.TargetTransferTag == 0xFFFFFFFF {
        return // response to our ping, handled by Router
    }
    // Target-initiated: echo back with NOP-Out
    resp := &pdu.NOPOut{
        Header: pdu.Header{
            Immediate:        true,
            Final:            true,
            InitiatorTaskTag: 0xFFFFFFFF, // response, not a new task
        },
        TargetTransferTag: nopin.TargetTransferTag,
        CmdSN:             s.getCmdSN(),
        ExpStatSN:         s.getExpStatSN(),
    }
    // ... encode and send via writeCh
}
```

### Pattern 5: SendTargets Discovery Response Parsing
**What:** The SendTargets response is a text key-value payload where each target begins with `TargetName=<iqn>` followed by zero or more `TargetAddress=<addr>:<port>,<tpgt>` entries.
**When to use:** DISC-01, DISC-02.
**Example:**
```go
// Response text format:
// TargetName=iqn.2001-04.com.example:storage1\x00
// TargetAddress=10.0.0.1:3260,1\x00
// TargetAddress=10.0.0.2:3260,2\x00
// TargetName=iqn.2001-04.com.example:storage2\x00
// TargetAddress=10.0.0.3:3260,1\x00

func parseSendTargetsResponse(data []byte) []DiscoveryTarget {
    kvs := login.DecodeTextKV(data)
    var targets []DiscoveryTarget
    var current *DiscoveryTarget
    for _, kv := range kvs {
        switch kv.Key {
        case "TargetName":
            if current != nil {
                targets = append(targets, *current)
            }
            current = &DiscoveryTarget{Name: kv.Value}
        case "TargetAddress":
            if current != nil {
                portal := parsePortal(kv.Value) // "addr:port,tpgt"
                current.Portals = append(current.Portals, portal)
            }
        }
    }
    if current != nil {
        targets = append(targets, *current)
    }
    return targets
}
```

### Pattern 6: Multi-PDU Response Dispatch (Router Enhancement)
**What:** The current Router.Dispatch delivers one PDU per ITT and removes the entry. For SCSI read commands, multiple Data-In PDUs share the same ITT. The session layer must NOT use Router directly for Data-In; instead, register a task map that receives all PDUs for an ITT until the command completes.
**When to use:** All SCSI commands that receive Data-In PDUs.
**Implementation:** The ReadPump already dispatches by ITT to Router. For multi-PDU commands, use a channel with sufficient capacity (or unbounded) so the ReadPump never blocks. The task goroutine drains and processes PDUs in order.

### Anti-Patterns to Avoid
- **Blocking ReadPump on task processing:** Never do synchronous work in the ReadPump dispatcher. Always send to channels and process asynchronously.
- **Incrementing CmdSN for immediate commands:** NOP-Out marked Immediate carries CmdSN but MUST NOT advance it.
- **Single PDU per ITT assumption:** Router.Dispatch currently deletes the entry after one PDU. For multi-PDU SCSI reads, keep the entry alive until F-bit or S-bit.
- **Ignoring MaxCmdSN/ExpCmdSN validation:** Per RFC, if MaxCmdSN < ExpCmdSN - 1, BOTH must be ignored (stale/invalid update).
- **StatSN tracking across sessions:** StatSN is connection-scoped, not session-scoped. Reset on new connection.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Serial number comparison | Custom uint32 comparison | `serial.LessThan/GreaterThan/InWindow` | RFC 1982 wrap-around arithmetic already implemented and tested |
| Text key-value encoding | Custom SendTargets parser | `login.EncodeTextKV/DecodeTextKV` | Same null-delimited format used in login; already tested |
| PDU framing | Custom TCP read/write | `transport.ReadRawPDU/WriteRawPDU` | Handles digest, padding, buffer pooling |
| ITT allocation | Manual counter | `transport.Router.Register` | Handles wrap-around, reserved ITT avoidance |
| CRC32C digests | Custom digest code | `digest.CRC32C` + transport layer | Already negotiated and applied at transport level |

**Key insight:** Phase 1-2 built substantial infrastructure. The session layer orchestrates existing components, it does not reimplement them.

## Common Pitfalls

### Pitfall 1: Router Single-Dispatch vs Multi-PDU Commands
**What goes wrong:** Router.Dispatch deletes the pending entry after delivering one PDU. A SCSI read returning 5 Data-In PDUs would lose PDUs 2-5.
**Why it happens:** Router was designed for request/response (one PDU out, one PDU back). Multi-PDU responses are a session-layer concern.
**How to avoid:** Either (a) modify Router to support "persistent" registrations that survive multiple dispatches, or (b) have the session layer intercept all PDUs from ReadPump before they reach Router, using its own ITT->task map. Option (b) is cleaner -- the session layer maintains a task map and the ReadPump sends ALL PDUs to a session dispatcher channel, not directly to Router.
**Warning signs:** Dropped Data-In PDUs, incomplete reads, "no pending entry for ITT" warnings in logs.

### Pitfall 2: CmdSN Window Deadlock
**What goes wrong:** Submit() blocks on full CmdSN window. If the goroutine handling target responses (which would update MaxCmdSN and unblock Submit) is also blocked on something Submit holds, deadlock.
**Why it happens:** Shared mutex between command submission and response processing.
**How to avoid:** Response processing (updateWindow) must never be called while holding the Submit lock in a blocking way. Use sync.Cond properly -- the Cond.Wait atomically unlocks the mutex and suspends.
**Warning signs:** All goroutines blocked, no progress under load.

### Pitfall 3: NOP-Out CmdSN Handling
**What goes wrong:** NOP-Out marked Immediate still carries CmdSN but MUST NOT advance it. If CmdSN is incremented, the window drifts and commands get rejected by the target.
**Why it happens:** Treating NOP-Out like a regular command.
**How to avoid:** NOP-Out uses a separate path that reads CmdSN atomically but does not call Incr. Only non-immediate commands advance CmdSN.
**Warning signs:** Target rejects commands with "CmdSN out of range" after NOP exchanges.

### Pitfall 4: Data-In S-bit vs Separate SCSI Response
**What goes wrong:** Code only handles status from S-bit Data-In, missing the case where status arrives in a separate SCSIResponse PDU (no S-bit on any Data-In).
**Why it happens:** Assuming all read commands deliver status inline with data.
**How to avoid:** Task must handle both: (1) Data-In with S=1 as final PDU, (2) Data-In with F=1 but S=0 followed by a separate SCSIResponse PDU. Both paths must close the pipe and deliver Result.
**Warning signs:** Reads that transfer all data correctly but Result channel never receives status.

### Pitfall 5: Unsolicited NOP-In vs Response NOP-In
**What goes wrong:** Treating all NOP-In PDUs the same. Unsolicited NOP-In (from target, ITT=0xFFFFFFFF) arrives on unsolicitedCh and needs a NOP-Out response echoing the TTT. Response NOP-In (to our ping, ITT matches our registered ITT) arrives via Router.
**Why it happens:** Not distinguishing the two flows.
**How to avoid:** ReadPump already separates: ITT=0xFFFFFFFF goes to unsolicitedCh, others go to Router. The unsolicited handler must check: if opcode is NOP-In AND TTT != 0xFFFFFFFF, respond with NOP-Out echoing TTT. If TTT = 0xFFFFFFFF, it's an informational update (update ExpCmdSN/MaxCmdSN).
**Warning signs:** Target times out and drops connection because initiator never responds to its NOP-In pings.

### Pitfall 6: SendTargets Continuation (C-bit)
**What goes wrong:** Large discovery responses may span multiple TextResp PDUs with C=1 (continue). If code only processes the first PDU, targets with many LUNs return incomplete results.
**Why it happens:** Assuming response fits in one PDU.
**How to avoid:** Loop reading TextResp PDUs while C=1, concatenating data segments. The final PDU has C=0. Each continuation uses the TTT from the previous response.
**Warning signs:** Truncated target list when target has many portal entries.

### Pitfall 7: Logout During Active Commands
**What goes wrong:** Calling Logout while commands are still in flight causes the target to reject the logout or abort commands unexpectedly.
**Why it happens:** No drain phase before logout.
**How to avoid:** Session.Close() should: (1) stop accepting new commands, (2) wait for in-flight commands to complete or timeout, (3) send Logout PDU, (4) wait for LogoutResp, (5) close connection.
**Warning signs:** Commands returning errors during shutdown, target sending AsyncMsg event=3 (session drop).

### Pitfall 8: ExpCmdSN/MaxCmdSN Update Validation
**What goes wrong:** Blindly accepting any ExpCmdSN/MaxCmdSN from target response PDUs, including stale values from reordered or delayed PDUs.
**Why it happens:** Not implementing the RFC validation rule.
**How to avoid:** Per RFC 7143: if MaxCmdSN < ExpCmdSN - 1 (serial comparison), BOTH values must be ignored. Only update local values when they advance (serial.GreaterThan).
**Warning signs:** Command window shrinks unexpectedly, Submit blocks indefinitely.

## Code Examples

### Result Type Structure
```go
// Result holds the outcome of a SCSI command submitted via Session.Submit.
type Result struct {
    Status    uint8      // SCSI status byte (0x00=GOOD, 0x02=CHECK CONDITION, etc.)
    SenseData []byte     // Sense data from SCSI Response or S-bit Data-In
    Data      io.Reader  // Data payload (nil for non-read commands)

    // Residual information
    Overflow      bool
    Underflow     bool
    ResidualCount uint32
}
```

### AsyncEvent Type
```go
// AsyncEvent represents an asynchronous event from the target.
type AsyncEvent struct {
    EventCode  uint8  // 0=SCSI event, 1=logout request, 2=conn drop, 3=session drop, 4=negotiation, 5=task termination, 255=vendor
    VendorCode uint8  // Only meaningful when EventCode=255
    Parameter1 uint16
    Parameter2 uint16
    Parameter3 uint16
    Data       []byte // Event-specific data (e.g., sense data for EventCode=0)
}
```

### DiscoveryTarget and Portal Types
```go
// DiscoveryTarget represents a target discovered via SendTargets.
type DiscoveryTarget struct {
    Name    string   // IQN of the target
    Portals []Portal // Network portals for this target
}

// Portal represents a network endpoint for an iSCSI target.
type Portal struct {
    Address string // IP address or hostname
    Port    int    // TCP port (default 3260)
    GroupTag int   // Target Portal Group Tag
}
```

### Session Functional Options
```go
// SessionOption configures a Session via functional options.
type SessionOption func(*sessionConfig)

type sessionConfig struct {
    keepaliveInterval time.Duration  // default 30s
    keepaliveTimeout  time.Duration  // default 5s (time to wait for NOP-In response)
    asyncHandler      func(AsyncEvent)
    logger            *slog.Logger
}

func WithKeepaliveInterval(d time.Duration) SessionOption {
    return func(c *sessionConfig) { c.keepaliveInterval = d }
}

func WithAsyncHandler(h func(AsyncEvent)) SessionOption {
    return func(c *sessionConfig) { c.asyncHandler = h }
}
```

### Session Dispatcher Pattern (handling multi-PDU responses)
```go
// dispatchLoop runs in a goroutine, receiving all PDUs from ReadPump
// and routing them to the appropriate task or handler.
func (s *Session) dispatchLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case raw, ok := <-s.pduCh:
            if !ok {
                return
            }
            p, err := pdu.DecodeBHS(raw.BHS)
            if err != nil {
                continue
            }
            switch v := p.(type) {
            case *pdu.DataIn:
                s.handleDataIn(v, raw.DataSegment)
            case *pdu.SCSIResponse:
                s.handleSCSIResponse(v, raw.DataSegment)
            case *pdu.NOPIn:
                s.handleNOPIn(v)
            case *pdu.AsyncMsg:
                s.handleAsyncMsg(v, raw.DataSegment)
            case *pdu.TextResp:
                s.handleTextResp(v, raw.DataSegment)
            case *pdu.LogoutResp:
                s.handleLogoutResp(v)
            case *pdu.Reject:
                s.handleReject(v, raw.DataSegment)
            }
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Mutex + manual counter for CmdSN window | sync.Cond or buffered semaphore channel | N/A (Go patterns) | Cond is cleaner for variable-size windows; channel works for fixed-size |
| Buffer all Data-In then return | io.Pipe streaming | D-05 decision | Lower memory, suits tape devices, Go-idiomatic |
| Polling-based keepalive | time.Ticker + NOP-Out | Standard iSCSI initiator pattern | Deterministic interval, testable with synctest |

## Open Questions

1. **Router modification vs session-level dispatch**
   - What we know: Router deletes ITT entry after one Dispatch. Multi-PDU commands need persistent entries.
   - What's unclear: Whether to modify Router (add DispatchPersistent) or bypass it entirely at session level.
   - Recommendation: Bypass Router for session-layer dispatch. Have ReadPump send ALL PDUs to a single session channel. The session dispatcher routes by ITT using its own task map. Router remains useful for simple request/response exchanges (NOP-Out ping, Text Request, Logout).

2. **io.Pipe backpressure on slow consumers**
   - What we know: If the caller reads slowly from the io.Reader, the PipeWriter blocks, which blocks the dispatcher goroutine.
   - What's unclear: Whether this blocks other tasks' PDU processing.
   - Recommendation: Each task should have its own PDU channel. The dispatcher fans out PDUs by ITT, so one slow consumer does not block others. The per-task channel provides buffering.

3. **Initial CmdSN and ExpStatSN after login**
   - What we know: Login sets CmdSN and ExpStatSN during login exchange. These values must be carried into the session.
   - What's unclear: Exact handoff mechanism.
   - Recommendation: Login already returns NegotiatedParams. Add CmdSN and ExpStatSN to NegotiatedParams (or return them separately from Login). The session initializes its counters from these values.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testing/synctest` (Go 1.25) |
| Config file | None needed (go test discovers tests automatically) |
| Quick run command | `go test ./internal/session/ -count=1 -race` |
| Full suite command | `go test ./... -count=1 -race` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SESS-01 | Session state machine transitions | unit | `go test ./internal/session/ -run TestSessionState -race -count=1` | Wave 0 |
| SESS-02 | CmdSN window throttles when full, unblocks on MaxCmdSN update | unit | `go test ./internal/session/ -run TestCmdWindow -race -count=1` | Wave 0 |
| SESS-03 | StatSN/ExpStatSN tracking from response PDUs | unit | `go test ./internal/session/ -run TestStatSN -race -count=1` | Wave 0 |
| SESS-04 | SCSICommand PDU built with correct ITT, CmdSN, CDB | unit | `go test ./internal/session/ -run TestSubmitCommand -race -count=1` | Wave 0 |
| SESS-05 | NOP-Out ping and NOP-In response in both directions | integration | `go test ./internal/session/ -run TestKeepalive -race -count=1` | Wave 0 |
| READ-01 | Data-In DataSN validation, gap detection | unit | `go test ./internal/session/ -run TestDataInSequence -race -count=1` | Wave 0 |
| READ-02 | Multi-PDU read assembles complete data via io.Reader | integration | `go test ./internal/session/ -run TestMultiPDURead -race -count=1` | Wave 0 |
| READ-03 | Status from S-bit Data-In AND separate SCSIResponse | unit | `go test ./internal/session/ -run TestStatusDelivery -race -count=1` | Wave 0 |
| EVT-01 | AsyncMsg dispatched to callback with correct event code | unit | `go test ./internal/session/ -run TestAsyncEvent -race -count=1` | Wave 0 |
| EVT-02 | Graceful logout PDU exchange | integration | `go test ./internal/session/ -run TestLogout -race -count=1` | Wave 0 |
| EVT-03 | Logout reason code 2 for recovery | unit | `go test ./internal/session/ -run TestLogoutRecovery -race -count=1` | Wave 0 |
| DISC-01 | SendTargets=All request and response parsing | integration | `go test ./internal/session/ -run TestSendTargets -race -count=1` | Wave 0 |
| DISC-02 | DiscoveryTarget struct populated with Name and Portals | unit | `go test ./internal/session/ -run TestParseDiscovery -race -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/session/ -count=1 -race`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/session/session_test.go` -- mock target for session-level integration tests
- [ ] `internal/session/cmdwindow_test.go` -- CmdSN window unit tests
- [ ] `internal/session/datain_test.go` -- Data-In reassembly tests
- [ ] `internal/session/keepalive_test.go` -- NOP-Out/NOP-In tests with synctest
- [ ] `internal/session/logout_test.go` -- Logout exchange tests
- [ ] `internal/session/discovery_test.go` -- SendTargets parsing and integration tests

## Project Constraints (from CLAUDE.md)

- **Language:** Go 1.25 -- use modern features (range-over-func, enhanced generics, testing/synctest)
- **Dependencies:** Zero external dependencies; stdlib + internal packages only
- **Standard:** RFC 7143 compliance drives implementation
- **Testing:** Fully testable without manual infrastructure -- mock target goroutine pattern from Phase 2
- **API style:** context.Context for cancellation, io.Reader/Writer where natural, structured errors
- **Quality:** High test coverage, clean interfaces, no dead code, no speculative abstractions
- **Avoid:** testify, protobuf, any external logging library, any wrapper around system iSCSI tools
- **Test pattern:** Table-driven tests with t.Run subtests, stdlib testing only
- **Logging:** log/slog with injectable slog.Handler

## Sources

### Primary (HIGH confidence)
- [RFC 7143](https://www.rfc-editor.org/rfc/rfc7143) -- iSCSI protocol specification: CmdSN/MaxCmdSN windowing (Section 3.2.2, 4.2), Data-In format (Section 11.7), NOP-Out/NOP-In (Section 11.18/11.19), AsyncMsg event codes (Section 11.9), Logout (Section 11.14/11.15), SendTargets (Section 4.3, 10.13)
- Existing codebase (`internal/transport/`, `internal/pdu/`, `internal/login/`, `internal/serial/`) -- verified by reading source files
- [Go 1.25 testing/synctest](https://pkg.go.dev/testing/synctest) -- deterministic concurrent test framework

### Secondary (MEDIUM confidence)
- [IANA iSCSI Parameters](https://www.iana.org/assignments/iscsi-parameters/iscsi-parameters.xhtml) -- async event codes, logout reason codes registry

### Tertiary (LOW confidence)
- [libiscsi](https://github.com/sahlberg/libiscsi) -- referenced for API design patterns (async core + sync wrapper), not directly verified for implementation details

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, all verified in existing codebase
- Architecture: HIGH -- patterns derived from RFC 7143 rules and existing codebase structure
- Pitfalls: HIGH -- derived from RFC protocol rules and real implementation concerns (Router single-dispatch, CmdSN immediate handling, dual NOP flows)

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable -- RFC 7143 is a stable standard, Go 1.25 is released)
