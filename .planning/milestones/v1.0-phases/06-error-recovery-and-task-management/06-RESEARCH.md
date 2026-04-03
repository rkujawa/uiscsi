# Phase 6: Error Recovery and Task Management - Research

**Researched:** 2026-04-01
**Domain:** iSCSI error recovery (ERL 0/1/2) + task management functions per RFC 7143
**Confidence:** HIGH

## Summary

This phase implements all three iSCSI error recovery levels and six task management functions on top of an already mature session layer. The existing codebase provides all PDU types (`TaskMgmtReq`, `TaskMgmtResp`, `SNACKReq`), ITT-based routing, CmdSN windowing, and a per-task goroutine model that must be carefully extended for recovery and TMF operations. The primary challenge is session-layer state management during recovery transitions -- specifically: stopping background goroutines cleanly, snapshotting in-flight tasks, re-running login with ISID+TSIH for reinstatement, and resubmitting commands with new sequence numbers while remaining transparent to callers.

The login package currently lacks a `WithTSIH` option needed for session reinstatement (it always sends TSIH=0). This is a concrete gap that must be addressed in Plan 1. The `faultConn` wrapper for error injection testing is a new test utility (not production code) that wraps `net.Conn` with configurable fault hooks.

**Primary recommendation:** Structure into 3 plans: (1) TMF + faultConn infra, (2) ERL 0 session reinstatement with auto-reconnect, (3) ERL 1 SNACK + ERL 2 connection replacement. TMF first because it provides the task cleanup primitives that ERL 0 retry logic depends on.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Library auto-reconnects on connection loss. Session detects connection drop (via read pump error or AsyncEvent 2/3), automatically attempts reconnect + session reinstatement. Configurable via SessionOption.
- D-02: 3 reconnect attempts with exponential backoff (1s, 2s, 4s) by default. Configurable via `WithMaxReconnectAttempts()` and `WithReconnectBackoff()`. After exhaustion, all in-flight commands fail and `Session.Err()` returns the connection error.
- D-03: Reuse same ISID and present old TSIH for session reinstatement per RFC 7143 Section 6.3.5. Store ISID/TSIH in Session struct. Target recognizes as reinstatement and restores task allegiance.
- D-04: Retry all in-flight commands after successful session reinstatement. Commands that were in the `s.tasks` map when connection dropped are re-submitted with new CmdSN/ExpStatSN. Callers see transparent recovery.
- D-05: Full SNACK implementation for Data-In/Status retransmission without dropping connection. Uses existing `pdu.SNACKReq` PDU type.
- D-06: Dual detection: DataSN gap detection for fast mid-stream recovery (immediate SNACK when received DataSN > expected DataSN) plus per-task timeout as safety net for tail loss (final Data-In PDUs dropped). Timeout configurable via `WithSNACKTimeout()` SessionOption.
- D-07: DataSN gap detection extends existing `datain.go` reassembly tracking. When gap detected, SNACK Request sent with BegRun=expected DataSN and RunLength=gap size.
- D-08: Single-connection replacement within MaxConnections=1. Drop failed connection, establish new TCP connection, login with same ISID/TSIH + Logout for connection recovery, reassign tasks to new connection. Functional ERL 2 without requiring MC/S.
- D-09: Methods on Session: `sess.AbortTask(ctx, itt)`, `sess.AbortTaskSet(ctx, lun)`, `sess.LUNReset(ctx, lun)`, `sess.TargetWarmReset(ctx)`, `sess.TargetColdReset(ctx)`, `sess.ClearTaskSet(ctx, lun)`. Consistent with `sess.Submit()` pattern. Uses existing `pdu.TaskMgmtReq` / `pdu.TaskMgmtResp` PDU types.
- D-10: Dedicated `TMFResult` struct with Response code (function complete, not supported, task does not exist, rejected, etc.) and error. Separate from `session.Result` since TMF responses have different semantics (no sense data, no residuals).
- D-11: Successful ABORT TASK auto-resolves the aborted command's Result channel with `Result{Err: ErrTaskAborted}`. No dangling goroutines -- task goroutine is cleaned up.
- D-12: Successful ABORT TASK / ABORT TASK SET / LUN RESET / CLEAR TASK SET auto-cleans affected tasks: removes from `s.tasks` map, unregisters ITT from Router, resolves Result channels. One-stop cleanup for callers.
- D-13: `faultConn` wrapper around `net.Conn` with configurable faults: drop after N bytes, inject read/write errors at specific points, add latency. Deterministic and reproducible. Used with `net.Pipe()` for in-process testing.
- D-14: Dual test approach: gotgt + faultConn for connection-level faults (ERL 0, ERL 2). Synthetic PDU replay for protocol-level faults (missing DataSN sequences for SNACK testing, corrupt digests for ERL 1).

### Claude's Discretion
- Internal state machine design for recovery levels (ERL 0 reconnect FSM, etc.)
- SNACK timeout default value and backoff strategy
- How task reassignment bookkeeping works during ERL 2 connection replacement
- TMFResult response code enum values and naming
- faultConn internal design (hook points, configuration API)
- How re-login is orchestrated during reinstatement (reuse login package or separate path)
- Test file organization for error injection scenarios

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ERL-01 | Error Recovery Level 0 -- session-level recovery (detect failure, reconnect, reinstate session, retry commands) | D-01 through D-04; login.WithTSIH gap identified; reconnect FSM design; task snapshot + retry pattern |
| ERL-02 | Error Recovery Level 1 -- within-connection recovery (SNACK for data/status retransmission) | D-05 through D-07; existing SNACKReq PDU; datain.go DataSN tracking extension; dual detection pattern |
| ERL-03 | Error Recovery Level 2 -- connection-level recovery (connection replacement, task reassignment) | D-08; logout reason 2 already implemented; TASK REASSIGN function code 7 |
| TMF-01 | ABORT TASK -- abort specific outstanding task by ITT | D-09, D-11; TaskMgmtReq Function=1, ReferencedTaskTag=target ITT |
| TMF-02 | ABORT TASK SET -- abort all tasks from this initiator on a LUN | D-09, D-12; TaskMgmtReq Function=2, LUN field |
| TMF-03 | LUN RESET -- reset a specific logical unit | D-09; TaskMgmtReq Function=5, LUN field |
| TMF-04 | TARGET WARM RESET -- reset target (sessions preserved) | D-09; TaskMgmtReq Function=6 |
| TMF-05 | TARGET COLD RESET -- reset target (sessions dropped) | D-09; TaskMgmtReq Function=7 |
| TMF-06 | CLEAR TASK SET -- clear all tasks on a LUN from all initiators | D-09, D-12; TaskMgmtReq Function=3, LUN field |
| TEST-05 | Error injection tests for recovery level verification | D-13, D-14; faultConn wrapper; synthetic PDU replay; gotgt integration |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25.5 | Runtime | Already installed on NetBSD 10.1 amd64 |
| `net` | stdlib | TCP reconnection | Dial for new connections during recovery |
| `sync` | stdlib | Mutex, Once, Cond | Recovery state synchronization |
| `context` | stdlib | Cancellation/timeout | TMF and recovery operations need context |
| `testing/synctest` | stdlib | Deterministic concurrent tests | Test reconnect FSM, timeout-based SNACK detection, task cleanup races |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/pdu` (existing) | project | TaskMgmtReq, TaskMgmtResp, SNACKReq | All TMF and SNACK operations -- PDUs already fully implemented |
| `internal/login` (existing) | project | Session reinstatement login | Reuse Login() with WithISID + new WithTSIH for reconnect |
| `internal/transport` (existing) | project | Router, ReadPump, WritePump, Conn | Reconnect rebuilds transport layer with same Router |

**No new external dependencies.** All implementations use stdlib + existing project packages.

## Architecture Patterns

### Recommended File Structure
```
internal/session/
    tmf.go            # TMF methods (AbortTask, LUNReset, etc.) + TMFResult type
    recovery.go       # ERL 0 reconnect FSM, session reinstatement, task retry
    snack.go          # ERL 1 SNACK sending, DataSN gap detection hook
    connreplace.go    # ERL 2 connection replacement + task reassignment
    types.go          # Extended with TMFResult, ErrTaskAborted, recovery config fields
internal/session/
    tmf_test.go       # TMF unit tests with mock target
    recovery_test.go  # ERL 0 reconnect tests with faultConn
    snack_test.go     # ERL 1 SNACK tests with synthetic PDU replay
    connreplace_test.go  # ERL 2 connection replacement tests
internal/transport/
    faultconn.go      # faultConn test utility (in transport package, test-only)
    faultconn_test.go # faultConn self-tests
internal/login/
    login.go          # Add WithTSIH option for session reinstatement
```

### Pattern 1: TMF Request/Response (follows existing logout pattern)
**What:** TMF methods build a TaskMgmtReq PDU, send via writeCh, wait for TaskMgmtResp via Router single-shot registration, then auto-cleanup affected tasks.
**When to use:** All six TMF operations (D-09).
**Example:**
```go
// Source: existing session/logout.go pattern, extended for TMF
func (s *Session) AbortTask(ctx context.Context, itt uint32) (*TMFResult, error) {
    // TMF uses immediate delivery -- set Immediate bit, use current CmdSN
    // (not acquired from window) per RFC 7143 Section 11.5
    tmfITT, respCh := s.router.Register()

    req := &pdu.TaskMgmtReq{
        Header: pdu.Header{
            Immediate:        true,
            Final:            true,
            InitiatorTaskTag: tmfITT,
        },
        Function:          1, // ABORT TASK
        ReferencedTaskTag: itt,
        CmdSN:            s.window.current(),
        ExpStatSN:        s.getExpStatSN(),
    }
    // ... marshal, send via writeCh, wait on respCh with timeout ...
    // On success: auto-resolve aborted task's Result with ErrTaskAborted
    // and clean up from s.tasks + Router
}
```

### Pattern 2: ERL 0 Reconnect FSM
**What:** State machine that detects connection loss, stops pumps, snapshots in-flight tasks, re-dials + re-logins, and resubmits snapshotted commands.
**When to use:** ERL 0 auto-reconnect (D-01 through D-04).
**Example:**
```go
// Source: RFC 7143 Section 7.1.4.1 + Section 6.3.5
func (s *Session) reconnect(ctx context.Context) error {
    // 1. Cancel old pumps (s.cancel() on old context)
    // 2. Snapshot s.tasks -- copy map, keep task structs alive
    // 3. Close old TCP connection
    // 4. Exponential backoff loop: Dial new TCP, Login with WithISID(s.isid) + WithTSIH(s.tsih)
    // 5. Replace s.conn, create new writeCh/unsolCh, start new pump goroutines
    // 6. For each snapshotted task: acquire new CmdSN, rebuild SCSICommand PDU, re-register ITT, re-send
    // 7. Restart taskLoops for each retried task
}
```

### Pattern 3: faultConn Wrapper (test utility)
**What:** `net.Conn` wrapper with hook functions for injecting faults at precise points.
**When to use:** All error injection tests (D-13).
**Example:**
```go
// Source: design decision D-13
type faultConn struct {
    net.Conn
    mu         sync.Mutex
    readBytes  int64
    writeBytes int64
    readFault  func(n int64) error  // called before each Read, n = cumulative bytes read
    writeFault func(n int64) error  // called before each Write, n = cumulative bytes written
}

func (fc *faultConn) Read(p []byte) (int, error) {
    fc.mu.Lock()
    if fc.readFault != nil {
        if err := fc.readFault(fc.readBytes); err != nil {
            fc.mu.Unlock()
            return 0, err
        }
    }
    fc.mu.Unlock()
    n, err := fc.Conn.Read(p)
    fc.mu.Lock()
    fc.readBytes += int64(n)
    fc.mu.Unlock()
    return n, err
}
```

### Pattern 4: SNACK DataSN Gap Detection (extends existing datain.go)
**What:** Modify `task.handleDataIn()` to detect DataSN gaps and send SNACK instead of immediately failing.
**When to use:** ERL 1 within-connection recovery (D-05 through D-07).
**Example:**
```go
// Source: RFC 7143 Section 11.16, extends existing datain.go
func (t *task) handleDataIn(din *pdu.DataIn) {
    if din.DataSN != t.nextDataSN {
        if s.params.ErrorRecoveryLevel >= 1 {
            // Gap detected: send SNACK for missing range
            gap := din.DataSN - t.nextDataSN
            t.sendSNACK(t.nextDataSN, gap)  // BegRun, RunLength
            // Buffer this PDU for later reassembly after retransmission
            t.pendingDataIn[din.DataSN] = din
            return
        }
        // ERL 0: gap is fatal
        t.resultCh <- Result{Err: fmt.Errorf("session: DataSN gap")}
        return
    }
    // ... existing reassembly logic ...
}
```

### Anti-Patterns to Avoid
- **Blocking TMF on in-flight task completion:** TMF is immediate -- do not wait for the aborted task's goroutine to drain before returning TMFResult. Cancel the task goroutine asynchronously after TMF response confirms success.
- **Re-using old writeCh after reconnect:** The old writeCh is drained by the old WritePump goroutine. Reconnect must create a fresh writeCh and new pump goroutines to avoid races.
- **Incrementing CmdSN for TMF immediate commands:** TMF with Immediate bit set carries current CmdSN but does NOT advance it. Use `s.window.current()`, not `s.window.acquire()`.
- **Retrying write commands with consumed io.Reader:** After reconnect, the original io.Reader for write commands may be partially consumed. Either buffer write data or fail write commands during retry. Read commands can be retried cleanly since they have no request body.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Login for session reinstatement | Custom login exchange | Existing `login.Login()` with new `WithTSIH()` option | Login state machine is complex (CHAP, operational negotiation); reinstatement only differs in TSIH value |
| PDU encoding for TMF/SNACK | Manual byte packing | Existing `pdu.TaskMgmtReq.MarshalBHS()`, `pdu.SNACKReq.MarshalBHS()` | Already implemented and tested in Phase 1 |
| ITT allocation + response correlation | Custom tracking map | Existing `transport.Router.Register()` / `RegisterPersistent()` | Router handles all the concurrent dispatch safely |
| CmdSN windowing for retried commands | Manual sequence tracking | Existing `cmdWindow.acquire()` | Window handles serial arithmetic, blocking, and cancellation |
| Exponential backoff | Custom timer logic | Simple `time.Duration` multiplication in a loop | Backoff is 3 attempts with fixed multiplier -- too simple for a library |

## Common Pitfalls

### Pitfall 1: Write Command Retry After Reconnect
**What goes wrong:** Write commands have an `io.Reader` for data. After the first attempt partially reads it, retry cannot re-read from the beginning.
**Why it happens:** `io.Reader` is forward-only; `bytes.NewReader` supports `Seek` but `io.Reader` does not guarantee it.
**How to avoid:** For write commands in the retry snapshot, check if the Reader implements `io.Seeker`. If yes, seek to start. If no, fail the write command with `ErrRetryNotPossible` -- callers must resubmit with a fresh Reader. Document this limitation.
**Warning signs:** Retried write commands produce corrupt data or short writes on target.

### Pitfall 2: Race Between TMF Cleanup and taskLoop
**What goes wrong:** TMF auto-cleanup calls `task.cancel()` and removes from `s.tasks` while `taskLoop` goroutine is still processing PDUs for that ITT.
**Why it happens:** `taskLoop` runs in its own goroutine draining `pduCh`. Router may deliver a PDU to `pduCh` after TMF cleanup has already run.
**How to avoid:** Unregister ITT from Router FIRST (stops new PDU delivery), THEN cancel the task, THEN remove from s.tasks. Close the persistent channel to terminate the taskLoop goroutine.
**Warning signs:** Panic on send to closed channel, or goroutine leak where taskLoop blocks forever.

### Pitfall 3: Reconnect During Active Submit
**What goes wrong:** A Submit() call is in progress (acquired CmdSN, building PDU) when connection drops. The Submit sends to the old writeCh which is being drained/closed.
**Why it happens:** No coordination between reconnect FSM and new Submit calls.
**How to avoid:** Use a session-level "recovering" state that makes Submit() block or return error until recovery completes. Protect with `s.mu` or a dedicated `sync.RWMutex`.
**Warning signs:** Deadlock or panic when writeCh send races with channel close.

### Pitfall 4: SNACK with Wrong ITT
**What goes wrong:** SNACK Request must carry the ITT of the command whose Data-In PDUs are missing.
**Why it happens:** Using the wrong ITT field (e.g., allocating a new ITT for the SNACK itself).
**How to avoid:** Per RFC 7143 Section 11.16: "The Initiator Task Tag field in the SNACK MUST be the same as the Initiator Task Tag used for the referenced command." SNACK is NOT registered with the Router -- it piggybacks on the existing task's ITT.
**Warning signs:** Target rejects SNACK or retransmits wrong data.

### Pitfall 5: CmdSN for Retried Commands After Reinstatement
**What goes wrong:** Retried commands use old CmdSN values that are outside the new session's window.
**Why it happens:** After reinstatement, the target resets its ExpCmdSN/MaxCmdSN. Old CmdSN values from before the drop are stale.
**How to avoid:** Per D-04: "re-submitted with new CmdSN/ExpStatSN." Each retried command must acquire a fresh CmdSN from the new window. The new window is initialized from the login response's post-login CmdSN.
**Warning signs:** Target silently drops retried commands (CmdSN outside window).

### Pitfall 6: Deadlock in Reconnect When Pumps Are Blocked
**What goes wrong:** readPumpLoop blocks on `io.ReadFull` from a dead connection. writePumpLoop blocks on `io.Write` to a dead connection. Reconnect needs these to stop.
**Why it happens:** TCP connections without keepalive can hang indefinitely on read/write.
**How to avoid:** Close the underlying `net.Conn` first -- this unblocks any pending Read/Write with an error. Then cancel the context. The pump goroutines will exit on the I/O error.
**Warning signs:** Reconnect hangs waiting for old goroutines to exit.

### Pitfall 7: TMF Immediate Bit and CmdSN
**What goes wrong:** TMF sent without Immediate bit uses acquire() and advances CmdSN. Or TMF sent with Immediate bit incorrectly advances CmdSN.
**Why it happens:** Confusion about RFC 7143 Section 11.5: "If the Immediate bit is set to 1, the CmdSN is not advanced."
**How to avoid:** TMF always set Immediate=true and use `s.window.current()` for CmdSN. Do NOT call `s.window.acquire()`.
**Warning signs:** CmdSN desynchronization after TMF, subsequent commands rejected.

### Pitfall 8: ABORT TASK SET / LUN RESET Must Clean ALL Matching Tasks
**What goes wrong:** Only the first matching task is cleaned up, leaving others dangling.
**Why it happens:** Iterating `s.tasks` and deleting during iteration, or forgetting to match by LUN.
**How to avoid:** Snapshot matching ITTs first (iterate s.tasks under lock, collect ITTs where LUN matches), then clean each one outside the lock. Use the same cleanup pattern for ABORT TASK SET, LUN RESET, and CLEAR TASK SET.
**Warning signs:** Goroutine leaks, dangling channels, stale Router entries after reset.

## Code Examples

### TMF Function Codes (from RFC 7143 Section 11.5.1)
```go
// TMF function codes per RFC 7143 Section 11.5.1
const (
    TMFAbortTask       uint8 = 1
    TMFAbortTaskSet    uint8 = 2
    TMFClearTaskSet    uint8 = 3
    TMFLogicalUnitReset uint8 = 5
    TMFTargetWarmReset uint8 = 6
    TMFTargetColdReset uint8 = 7
    TMFTaskReassign    uint8 = 14
)
```

### TMF Response Codes (from RFC 7143 Section 11.6.1)
```go
// TMF response codes per RFC 7143 Section 11.6.1
const (
    TMFRespComplete           uint8 = 0
    TMFRespTaskNotExist       uint8 = 1
    TMFRespLUNNotExist        uint8 = 2
    TMFRespTaskAllegiant      uint8 = 3
    TMFRespReassignNotSupport uint8 = 4
    TMFRespNotSupported       uint8 = 5
    TMFRespAuthFailed         uint8 = 6
    TMFRespRejected           uint8 = 255
)
```

### SNACK Type Values (from RFC 7143 Section 11.16.1)
```go
// SNACK type values per RFC 7143 Section 11.16.1
const (
    SNACKTypeDataR2T    uint8 = 0 // Data/R2T SNACK
    SNACKTypeStatus     uint8 = 1 // Status SNACK
    SNACKTypeDataACK    uint8 = 2 // Data ACK
    SNACKTypeRDataSNACK uint8 = 3 // R-Data SNACK (for R-Data recovery)
)
```

### TMFResult Type
```go
// TMFResult carries the outcome of a task management function request.
type TMFResult struct {
    Response uint8 // TMF response code from target
    Err      error // transport-level error, if any
}

// Sentinel errors for task management outcomes
var (
    ErrTaskAborted = errors.New("session: task aborted")
)
```

### login.WithTSIH (new -- needed for session reinstatement)
```go
// WithTSIH sets the TSIH for session reinstatement. Non-zero TSIH
// tells the target to reinstate an existing session rather than
// creating a new one. Per RFC 7143 Section 6.3.5.
func WithTSIH(tsih uint16) LoginOption {
    return func(c *loginConfig) {
        c.tsih = tsih
    }
}
```
Note: `loginConfig` already has a `tsih` field (line 166 of login.go, `loginState.tsih`), but it needs to be exposed through `loginConfig` and wired into `loginState.tsih`. Currently `loginState.tsih` is hardcoded to 0.

### Session Reconnect Config Options
```go
func WithMaxReconnectAttempts(n int) SessionOption {
    return func(c *sessionConfig) { c.maxReconnectAttempts = n }
}

func WithReconnectBackoff(base time.Duration) SessionOption {
    return func(c *sessionConfig) { c.reconnectBackoff = base }
}

func WithSNACKTimeout(d time.Duration) SessionOption {
    return func(c *sessionConfig) { c.snackTimeout = d }
}
```

### faultConn Configuration
```go
// FaultConn wraps net.Conn with injectable faults for testing.
type FaultConn struct {
    net.Conn
    mu          sync.Mutex
    readCount   int64
    writeCount  int64
    readFault   func(bytesRead int64) error
    writeFault  func(bytesWritten int64) error
}

// WithReadFaultAfter returns a fault function that errors after N bytes.
func WithReadFaultAfter(n int64, err error) func(int64) error {
    return func(bytesRead int64) error {
        if bytesRead >= n { return err }
        return nil
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Open-iscsi kernel+userspace recovery | Pure userspace recovery in library | This project | No kernel dependencies for error recovery |
| ERL negotiation as single value | Hierarchical: ERL 2 implies ERL 1 implies ERL 0 | RFC 7143 | Implementation must support all lower levels when higher is negotiated |

## Open Questions

1. **Write command retry seekability**
   - What we know: `io.Reader` cannot rewind; `bytes.NewReader` and `os.File` implement `io.Seeker`
   - What's unclear: Whether to attempt `io.Seeker` type assertion at retry time or require all write Readers to be seekable
   - Recommendation: Attempt `io.Seeker` assertion. If seekable, rewind and retry. If not, fail with descriptive error. This is the pragmatic approach -- most callers use `bytes.NewReader`.

2. **SNACK timeout default value**
   - What we know: Must be long enough to avoid false positives, short enough to detect tail loss
   - What's unclear: No standard default in RFC 7143
   - Recommendation: Default 5 seconds. Configurable via `WithSNACKTimeout()`. This matches typical iSCSI target response times for retransmission.

3. **Session reinstatement and task allegiance timing**
   - What we know: RFC 7143 Section 13.8 defines DefaultTime2Wait (default 2s) and Section 13.7 defines DefaultTime2Retain (default 20s) for the window during which tasks remain allegiant
   - What's unclear: Whether gotgt correctly implements these timers for integration testing
   - Recommendation: Use negotiated DefaultTime2Wait before reconnect attempt. Test with synthetic PDU replay if gotgt timing is unreliable.

4. **Router state during reconnect**
   - What we know: Router tracks ITTs. During reconnect, old ITTs must be preserved for retry.
   - What's unclear: Whether to reuse the same Router instance or create a new one
   - Recommendation: Reuse same Router. During reconnect: unregister all old entries, re-register with same ITTs for retried tasks. This preserves ITT allocation state and avoids ITT reuse conflicts.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.25.5) |
| Config file | None needed -- `go test` auto-discovers |
| Quick run command | `go test ./internal/session/ -run TestTMF -count=1` |
| Full suite command | `go test -race ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TMF-01 | ABORT TASK sends correct PDU, processes response, cleans up aborted task | unit | `go test ./internal/session/ -run TestAbortTask -count=1` | Wave 0 |
| TMF-02 | ABORT TASK SET aborts all tasks on LUN | unit | `go test ./internal/session/ -run TestAbortTaskSet -count=1` | Wave 0 |
| TMF-03 | LUN RESET sends correct PDU, cleans tasks on LUN | unit | `go test ./internal/session/ -run TestLUNReset -count=1` | Wave 0 |
| TMF-04 | TARGET WARM RESET sends correct PDU | unit | `go test ./internal/session/ -run TestTargetWarmReset -count=1` | Wave 0 |
| TMF-05 | TARGET COLD RESET sends correct PDU | unit | `go test ./internal/session/ -run TestTargetColdReset -count=1` | Wave 0 |
| TMF-06 | CLEAR TASK SET clears all tasks on LUN | unit | `go test ./internal/session/ -run TestClearTaskSet -count=1` | Wave 0 |
| ERL-01 | Connection drop triggers reconnect, session reinstated, in-flight commands retried | integration | `go test ./internal/session/ -run TestERL0Reconnect -count=1 -race` | Wave 0 |
| ERL-02 | DataSN gap triggers SNACK, retransmitted PDU reassembled correctly | unit | `go test ./internal/session/ -run TestSNACK -count=1` | Wave 0 |
| ERL-03 | Failed connection replaced, tasks reassigned to new connection | integration | `go test ./internal/session/ -run TestERL2ConnReplace -count=1 -race` | Wave 0 |
| TEST-05 | faultConn injects errors deterministically; error injection tests cover all ERLs | unit+integration | `go test ./internal/transport/ -run TestFaultConn -count=1 && go test ./internal/session/ -run TestErrorInjection -count=1 -race` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/session/ -count=1 -race`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/session/tmf_test.go` -- covers TMF-01 through TMF-06
- [ ] `internal/session/recovery_test.go` -- covers ERL-01
- [ ] `internal/session/snack_test.go` -- covers ERL-02
- [ ] `internal/session/connreplace_test.go` -- covers ERL-03
- [ ] `internal/transport/faultconn.go` -- faultConn test utility
- [ ] `internal/transport/faultconn_test.go` -- faultConn self-tests, covers TEST-05

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All code | Yes | 1.25.5 | -- |
| testing/synctest | Concurrent recovery tests | Yes | stdlib (Go 1.25) | -- |
| net.Pipe() | In-process testing | Yes | stdlib | -- |
| golangci-lint | Linting | N/A for phase | -- | go vet |

No external dependencies needed. Pure stdlib implementation.

## Sources

### Primary (HIGH confidence)
- RFC 7143 (https://www.rfc-editor.org/rfc/rfc7143.html) -- Sections 6.3.5 (session reinstatement), 7 (error recovery), 11.5/11.6 (TMF), 11.16 (SNACK), 13.19 (ErrorRecoveryLevel)
- Existing codebase: `internal/pdu/initiator.go` (TaskMgmtReq lines 79-110, SNACKReq lines 266-295), `internal/pdu/target.go` (TaskMgmtResp lines 102-130) -- PDU types verified as fully implemented
- Existing codebase: `internal/session/session.go`, `internal/session/async.go`, `internal/session/datain.go`, `internal/session/logout.go` -- integration points verified by code review
- Existing codebase: `internal/login/login.go` -- WithISID exists, WithTSIH gap identified (loginState.tsih always 0)

### Secondary (MEDIUM confidence)
- RFC 7143 text format (https://www.rfc-editor.org/rfc/rfc7143.txt) -- TMF function codes (1,2,3,5,6,7,14), response codes (0-6, 255), SNACK types (0-3) -- values cross-verified with web fetch summaries

### Tertiary (LOW confidence)
- None -- all findings verified against RFC and codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- stdlib only, all packages verified present in Go 1.25.5
- Architecture: HIGH -- extends well-understood existing patterns (logout, submit, datain)
- Pitfalls: HIGH -- derived from code review of actual session.go, datain.go, transport/pump.go
- TMF protocol details: HIGH -- PDU types already implemented, function codes from RFC
- ERL protocol details: HIGH -- RFC 7143 Section 7 thoroughly analyzed
- Recovery state management: MEDIUM -- FSM design is discretionary, multiple valid approaches

**Research date:** 2026-04-01
**Valid until:** 2026-05-01 (stable domain -- RFC 7143 is fixed, codebase is under our control)
