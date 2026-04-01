# Phase 4: Write Path - Research

**Researched:** 2026-04-01
**Domain:** iSCSI write path - R2T handling, Data-Out generation, immediate/unsolicited data, burst length enforcement
**Confidence:** HIGH

## Summary

The iSCSI write path is the inverse of the read path implemented in Phase 3. Where reads receive Data-In PDUs and reassemble them, writes generate Data-Out PDUs in response to R2T (Ready To Transfer) PDUs from the target. The complexity comes from four distinct write modes created by the ImmediateData x InitialR2T parameter matrix, and from strict burst length enforcement (FirstBurstLength for unsolicited data, MaxBurstLength for solicited data).

The existing codebase already has all the building blocks: `pdu.DataOut` and `pdu.R2T` types with full MarshalBHS/UnmarshalBHS (Phase 1), the `task` per-goroutine pattern with Router channel (Phase 3), `NegotiatedParams` with all write-relevant fields, and the `writeCh` serialization channel. The work is extending `Submit` to detect writes (via `cmd.Data != nil` per D-03), modifying the Command type to accept `io.Reader` for write data (per D-01), and implementing the write task loop that handles R2T PDUs and generates Data-Out PDU sequences.

**Primary recommendation:** Implement writes as a new `dataout.go` file in the session package, extending the existing `taskLoop` to dispatch R2T PDUs to write handling methods. Use the existing `writeCh` for all outgoing Data-Out PDUs. Test with the established `net.Pipe()` mock target pattern from session_test.go, using parameterized tests for the 2x2 ImmediateData x InitialR2T matrix.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Write data provided as `io.Reader` on the Command struct (`cmd.Data`). Callers with `[]byte` use `bytes.NewReader()`. Symmetric with the read path's `io.Reader` output.
- D-02: Session auto-detects write behavior from NegotiatedParams -- callers just Submit with data. Session handles ImmediateData piggybacking, unsolicited Data-Out, and R2T-solicited writes automatically based on negotiated InitialR2T, ImmediateData, FirstBurstLength, MaxBurstLength.
- D-03: `cmd.Data != nil` means write. If Command.Data is non-nil, it's a write command. If nil, it's a read or non-data command. No explicit Direction field needed.
- D-04: Sequential R2T processing per task. Even when MaxOutstandingR2T > 1, process R2Ts one at a time within a single task goroutine. Most targets use MaxOutstandingR2T=1. Can be optimized later if benchmarking shows a bottleneck.
- D-05: Extend existing per-task goroutine pattern. The per-task goroutine from the read path (drains Router channel for Data-In) also handles R2T PDUs for writes: receives R2T, reads chunk from io.Reader, sends Data-Out. Same goroutine, same lifetime.
- D-06: Read-on-demand from io.Reader. When R2T arrives, read exactly the needed chunk (up to MaxRecvDataSegmentLength per PDU, up to MaxBurstLength per R2T sequence). For immediate/unsolicited data, read FirstBurstLength upfront. No pre-buffering of entire write data.
- D-07: Reuse transport layer's buffer pool for Data-Out PDU data segments. Consistent with Phase 1's size-class buffer pooling (4KB/64KB/16MB) and copy-out ownership model.
- D-08: Write commands return same Result type as reads, with Data=nil. Status, SenseData, Err fields carry write outcome. Residual counts indicate how much data the target didn't accept. Uniform error handling for callers.
- D-09: On io.Reader error mid-write, abort the task and return the Reader error in Result.Err. No Data-Out PDUs sent after Reader failure. No iSCSI-level task abort (TMF is Phase 6). Target will time out the incomplete R2T sequence.

### Claude's Discretion
- DataSN numbering within Data-Out sequences
- How immediate data is attached to the SCSI Command PDU (inline in BHS data segment vs separate)
- Internal write task state machine design
- Buffer offset tracking across R2T sequences
- How ExpectedDataTransferLength is set on the SCSI Command PDU for writes

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WRITE-01 | R2T handling with R2TSN tracking and MaxOutstandingR2T compliance | R2T PDU type exists, taskLoop extension pattern documented, sequential processing per D-04 |
| WRITE-02 | Solicited Data-Out PDU generation in response to R2T | DataOut PDU type exists, read-on-demand from io.Reader per D-06, buffer pool reuse per D-07 |
| WRITE-03 | Immediate data support (piggybacked on SCSI Command PDU, bounded by FirstBurstLength) | SCSICommand.ImmediateData field exists, Submit already handles ImmediateData on wire |
| WRITE-04 | Unsolicited Data-Out when InitialR2T=No (before first R2T, bounded by FirstBurstLength) | DataOut with TTT=0xFFFFFFFF for unsolicited, FirstBurstLength includes immediate data |
| WRITE-05 | MaxBurstLength enforcement for solicited data sequences | NegotiatedParams.MaxBurstLength available, cap each R2T response sequence |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Language:** Go 1.25 -- use modern features where they improve clarity
- **Dependencies:** Minimal external dependencies (stdlib only for production code)
- **Standard:** RFC 7143 compliance -- the spec drives implementation
- **Testing:** stdlib `testing` with table-driven tests, `testing/synctest` for concurrent tests, no testify
- **API style:** Go idiomatic -- context.Context, io.Reader/Writer, structured errors
- **Buffer pool:** Use transport.GetBuffer/PutBuffer for Data-Out data segments
- **Test target:** net.Pipe() mock target for unit tests, gostor/gotgt for integration
- **No hand-rolling:** Use existing pdu.DataOut, pdu.R2T, pdu.SCSICommand types
- **GSD workflow:** Follow GSD workflow for all changes

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/binary` | stdlib | Data-Out PDU construction | BigEndian for network byte order fields in BHS |
| `io` | stdlib | io.Reader for write data source | D-01: write data provided as io.Reader |
| `bytes` | stdlib | io.Reader construction in tests | bytes.NewReader for test data |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/pdu` | project | DataOut, R2T, SCSICommand types | All PDU construction/parsing -- already implemented in Phase 1 |
| `internal/transport` | project | Buffer pool, WritePump, Router | Buffer allocation for Data-Out segments, PDU dispatch |
| `internal/login` | project | NegotiatedParams | Access ImmediateData, InitialR2T, burst length params |

### Alternatives Considered
None -- all building blocks exist in the project already. No new dependencies needed.

## Architecture Patterns

### Recommended Project Structure
```
internal/session/
    dataout.go          # NEW: Data-Out generation, R2T handling, write task logic
    dataout_test.go     # NEW: Write path unit tests (parameterized 2x2 matrix)
    types.go            # MODIFY: Add Data io.Reader field to Command
    session.go          # MODIFY: Submit detects writes, sends immediate/unsolicited data
    datain.go           # EXISTING: task type extended for write state
```

### Pattern 1: Write Task State Machine
**What:** The per-task goroutine (taskLoop) is extended to handle R2T PDUs for write tasks. When a write task receives an R2T via the Router channel, it reads data from the io.Reader, constructs Data-Out PDUs, and sends them via writeCh.
**When to use:** Every write command uses this pattern.
**Example:**
```go
// In taskLoop, after DecodeBHS:
case *pdu.R2T:
    s.window.update(p.ExpCmdSN, p.MaxCmdSN)
    s.updateStatSN(p.StatSN)
    if err := tk.handleR2T(p, s.writeCh, s.getExpStatSN(), s.params); err != nil {
        tk.cancel(err)
        s.cleanupTask(tk.itt)
        return
    }
```

### Pattern 2: Immediate Data at Submit Time
**What:** When ImmediateData=Yes and cmd.Data is non-nil, Submit reads up to min(FirstBurstLength, MaxRecvDataSegmentLength) bytes from cmd.Data and attaches them as the SCSICommand PDU's data segment.
**When to use:** Submit time, before the command is sent on the wire.
**Example:**
```go
// In Submit, after building SCSICommand:
if cmd.Data != nil && s.params.ImmediateData {
    immLen := min(s.params.FirstBurstLength, s.params.MaxRecvDataSegmentLength)
    immBuf := make([]byte, immLen)
    n, _ := io.ReadFull(cmd.Data, immBuf)
    scsiCmd.ImmediateData = immBuf[:n]
    scsiCmd.Header.DataSegmentLen = uint32(n)
    tk.bytesSent = uint32(n) // Track for unsolicited/R2T offset
}
```

### Pattern 3: Unsolicited Data-Out After Command
**What:** When InitialR2T=No and cmd.Data is non-nil, after sending the SCSI Command PDU, the initiator sends additional Data-Out PDUs (with TTT=0xFFFFFFFF) up to FirstBurstLength total (minus any immediate data already sent).
**When to use:** Immediately after sending the SCSI Command PDU, before waiting for R2T.
**Example:**
```go
// After sending SCSICommand via writeCh:
if cmd.Data != nil && !s.params.InitialR2T {
    remaining := s.params.FirstBurstLength - tk.bytesSent
    if remaining > 0 {
        tk.sendUnsolicitedDataOut(cmd.Data, remaining, s.writeCh, s.params, s.getExpStatSN)
    }
}
```

### Pattern 4: Solicited Data-Out in Response to R2T
**What:** When the task goroutine receives an R2T, it reads DesiredDataTransferLength bytes from the io.Reader and sends them as Data-Out PDUs, each bounded by MaxRecvDataSegmentLength, with total bounded by MaxBurstLength.
**When to use:** Every R2T received by the task goroutine.
**Example:**
```go
func (t *writeTask) handleR2T(r2t *pdu.R2T, writeCh chan<- *transport.RawPDU,
    expStatSN uint32, params login.NegotiatedParams) error {

    desired := r2t.DesiredDataTransferLength
    if desired > params.MaxBurstLength {
        desired = params.MaxBurstLength
    }
    offset := r2t.BufferOffset
    dataSN := uint32(0)
    sent := uint32(0)

    for sent < desired {
        chunkSize := min(params.MaxRecvDataSegmentLength, desired-sent)
        buf := transport.GetBuffer(int(chunkSize))
        n, err := io.ReadFull(t.reader, buf[:chunkSize])
        if err != nil && err != io.ErrUnexpectedEOF {
            transport.PutBuffer(buf)
            return fmt.Errorf("session: read write data: %w", err)
        }

        dout := &pdu.DataOut{
            Header: pdu.Header{
                InitiatorTaskTag: t.itt,
                DataSegmentLen:   uint32(n),
                Final:            sent+uint32(n) >= desired,
            },
            TargetTransferTag: r2t.TargetTransferTag,
            ExpStatSN:         expStatSN,
            DataSN:            dataSN,
            BufferOffset:      offset,
            Data:              buf[:n],
        }
        // ... marshal and send via writeCh
        dataSN++
        offset += uint32(n)
        sent += uint32(n)
    }
    return nil
}
```

### Anti-Patterns to Avoid
- **Pre-buffering entire write data:** Do NOT read the entire io.Reader into memory upfront. Read on-demand per R2T chunk (D-06). MaxBurstLength can be 16MB+.
- **Shared DataSN across R2T sequences:** DataSN resets to 0 for each R2T response and for the unsolicited sequence. Do NOT maintain a single DataSN counter across the entire task.
- **Sending Data-Out without TTT correlation:** Every solicited Data-Out MUST echo the TTT from its R2T. Unsolicited Data-Out uses TTT=0xFFFFFFFF.
- **Ignoring FirstBurstLength for immediate data:** Immediate data counts toward FirstBurstLength. If ImmediateData=Yes and InitialR2T=No, remaining unsolicited = FirstBurstLength - len(immediateData).
- **Setting Final bit incorrectly:** Final bit marks the last Data-Out in a burst (per R2T or per unsolicited sequence), NOT the last PDU for the entire write.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DataOut PDU encoding | Custom binary encoding | `pdu.DataOut.MarshalBHS()` | Already implemented in Phase 1 with correct field layout |
| R2T PDU decoding | Custom binary parsing | `pdu.R2T.UnmarshalBHS()` | Already implemented in Phase 1 |
| Buffer allocation | `make([]byte, ...)` | `transport.GetBuffer()` / `transport.PutBuffer()` | Pool reduces GC pressure for Data-Out segments |
| PDU wire serialization | Direct TCP writes | `writeCh` -> WritePump | Prevents TCP byte interleaving (single writer goroutine) |
| ITT correlation | Manual map tracking | `transport.Router.RegisterPersistent()` | Already handles R2T routing to task goroutine |
| CmdSN windowing | Custom sequence tracking | `cmdWindow.acquire()` | Existing flow control works for writes same as reads |

**Key insight:** The write path reuses nearly all Phase 1 and Phase 3 infrastructure. The new code is primarily the write-side logic in the task goroutine and the Submit modifications.

## Common Pitfalls

### Pitfall 1: Immediate Data Counting Toward FirstBurstLength
**What goes wrong:** Sending FirstBurstLength bytes of unsolicited Data-Out PLUS FirstBurstLength bytes of immediate data, exceeding the total first burst limit.
**Why it happens:** Misunderstanding that immediate data and unsolicited Data-Out are both part of the "first burst" governed by FirstBurstLength.
**How to avoid:** Track `bytesSent` starting from immediate data length. Unsolicited Data-Out remaining = FirstBurstLength - immediateDataLength.
**Warning signs:** Target rejects Data-Out PDUs or drops connection when both ImmediateData=Yes and InitialR2T=No.

### Pitfall 2: DataSN Scope Confusion
**What goes wrong:** Using a single DataSN counter for the entire write task, causing DataSN values to start at non-zero for solicited sequences after unsolicited data.
**Why it happens:** Assuming DataSN is per-task like DataSN in Data-In is per-task.
**How to avoid:** Reset DataSN to 0 for each new R2T response sequence and for the initial unsolicited sequence. Each burst has its own DataSN counter.
**Warning signs:** Target reports DataSN errors for solicited Data-Out PDUs.

### Pitfall 3: Missing W-bit on SCSI Command PDU
**What goes wrong:** Write commands sent without the W-bit (byte 1 bit 5) set in the SCSI Command PDU, causing target to treat them as non-data commands.
**Why it happens:** Command struct has `Write bool` field but Submit doesn't auto-set it from cmd.Data presence.
**How to avoid:** In Submit, if `cmd.Data != nil`, force `cmd.Write = true` and set ExpectedDataTransferLength appropriately.
**Warning signs:** Target never sends R2T; write appears to succeed with no data transferred.

### Pitfall 4: Final Bit on Wrong Data-Out PDU
**What goes wrong:** Setting Final=true on the last Data-Out PDU of the entire transfer instead of on the last PDU of each burst.
**Why it happens:** Confusion between "final PDU in burst" and "final PDU in transfer."
**How to avoid:** Final=true when the current burst (R2T response or unsolicited sequence) is complete, NOT when the entire write is complete.
**Warning signs:** Target fails to send subsequent R2Ts or times out waiting for more data.

### Pitfall 5: io.Reader Partial Read Handling
**What goes wrong:** `io.Read()` returns fewer bytes than requested (valid per io.Reader contract), causing undersized Data-Out PDUs or incorrect offset tracking.
**Why it happens:** io.Reader is allowed to return short reads. If the write data comes from a network source or channel, this is common.
**How to avoid:** Use `io.ReadFull()` or `io.ReadAtLeast()` for each Data-Out chunk. Handle io.ErrUnexpectedEOF for the last chunk where the data source may have less than requested.
**Warning signs:** Data corruption at the target, BufferOffset mismatches.

### Pitfall 6: Command.Data io.Reader Consumed Across Goroutine Boundary
**What goes wrong:** Submit reads immediate data from cmd.Data in the calling goroutine, then the task goroutine also reads from the same io.Reader for unsolicited/solicited data. If the io.Reader is not safe for sequential access across goroutines, data corruption occurs.
**Why it happens:** io.Reader implementations are generally not thread-safe.
**How to avoid:** Submit reads immediate data synchronously before starting the task goroutine. The io.Reader is then exclusively owned by the task goroutine. No concurrent reads.
**Warning signs:** Race detector fires, garbled write data, duplicate or missing chunks.

### Pitfall 7: Not Setting ExpectedDataTransferLength on Write Commands
**What goes wrong:** Target does not know how much data to expect, sends incorrect R2Ts, or rejects the command.
**Why it happens:** Read path does not require knowing total size upfront in some implementations, but write path always does -- the target uses ExpectedDataTransferLength to calculate R2T parameters.
**How to avoid:** Require callers to set ExpectedDataTransferLen on the Command struct for writes. This is standard SCSI: the CDB encodes the transfer length, and iSCSI's ExpectedDataTransferLength must match.
**Warning signs:** Target sends R2T with DesiredDataTransferLength=0 or refuses command.

## Code Examples

### Data-Out PDU Construction (verified from existing pdu.DataOut)
```go
// Source: internal/pdu/initiator.go lines 206-234
dout := &pdu.DataOut{
    Header: pdu.Header{
        Final:            isFinalInBurst,
        InitiatorTaskTag: itt,
        DataSegmentLen:   uint32(len(data)),
    },
    TargetTransferTag: r2t.TargetTransferTag, // Echo TTT from R2T
    ExpStatSN:         expStatSN,
    DataSN:            dataSN,     // Per-burst counter, starts at 0
    BufferOffset:      offset,     // Byte offset in total write buffer
    Data:              data,
}
bhs, _ := dout.MarshalBHS()
raw := &transport.RawPDU{BHS: bhs, DataSegment: data}
```

### R2T PDU Fields (verified from existing pdu.R2T)
```go
// Source: internal/pdu/target.go lines 332-367
// R2T fields used by write path:
//   r2t.TargetTransferTag          - echo in Data-Out TTT
//   r2t.R2TSN                      - for R2TSN tracking (WRITE-01)
//   r2t.BufferOffset               - starting offset for this burst
//   r2t.DesiredDataTransferLength  - bytes requested (cap by MaxBurstLength)
//   r2t.StatSN                     - update session ExpStatSN
//   r2t.ExpCmdSN / r2t.MaxCmdSN   - update CmdSN window
```

### Command Type Modification (based on types.go)
```go
// Modified Command struct for write support:
type Command struct {
    CDB                    [16]byte
    Read                   bool
    Write                  bool
    ExpectedDataTransferLen uint32
    LUN                    uint64
    Data                   io.Reader // D-01: write data source (nil = non-write)
    TaskAttributes         uint8
}
// Note: Remove existing ImmediateData []byte field.
// Submit reads immediate data from Data when ImmediateData=Yes.
```

### Four ImmediateData x InitialR2T Combinations
```go
// Combination 1: ImmediateData=Yes, InitialR2T=Yes (RFC default)
// - Submit reads min(FirstBurstLength, MaxRecvDSL) from cmd.Data
// - Attaches as SCSI Command PDU data segment
// - Waits for R2T for remaining data
// Wire: [SCSICmd+ImmData] -> [R2T] -> [DataOut...] -> [SCSIResp]

// Combination 2: ImmediateData=Yes, InitialR2T=No
// - Submit reads immediate data as above
// - After sending command, sends unsolicited Data-Out up to FirstBurstLength
// - Then waits for R2T for remaining
// Wire: [SCSICmd+ImmData] [DataOut(unsol)...] -> [R2T] -> [DataOut...] -> [SCSIResp]

// Combination 3: ImmediateData=No, InitialR2T=Yes
// - No data on command PDU
// - All data via R2T solicitation
// Wire: [SCSICmd] -> [R2T] -> [DataOut...] -> [SCSIResp]

// Combination 4: ImmediateData=No, InitialR2T=No
// - No data on command PDU
// - Unsolicited Data-Out up to FirstBurstLength (TTT=0xFFFFFFFF)
// - Then R2T for remaining
// Wire: [SCSICmd] [DataOut(unsol)...] -> [R2T] -> [DataOut...] -> [SCSIResp]
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Pre-buffer entire write | Read-on-demand per R2T | Standard practice | Memory efficiency for large writes |
| Parallel R2T processing | Sequential per task (D-04) | Design decision | Simpler, correct; optimize later if needed |
| Separate ImmediateData field | io.Reader for all write data (D-01) | Design decision | Uniform interface; Submit handles partitioning |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` (Go 1.25) |
| Config file | none -- stdlib testing needs no config |
| Quick run command | `go test ./internal/session/ -run TestWrite -count=1` |
| Full suite command | `go test ./... -race -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WRITE-01 | R2T handling with R2TSN tracking | unit | `go test ./internal/session/ -run TestWriteR2T -count=1` | Wave 0 |
| WRITE-02 | Solicited Data-Out generation | unit | `go test ./internal/session/ -run TestWriteSolicited -count=1` | Wave 0 |
| WRITE-03 | Immediate data piggybacking | unit | `go test ./internal/session/ -run TestWriteImmediate -count=1` | Wave 0 |
| WRITE-04 | Unsolicited Data-Out | unit | `go test ./internal/session/ -run TestWriteUnsolicited -count=1` | Wave 0 |
| WRITE-05 | MaxBurstLength enforcement | unit | `go test ./internal/session/ -run TestWriteMaxBurst -count=1` | Wave 0 |
| ALL | 2x2 matrix parameterized | unit | `go test ./internal/session/ -run TestWriteMatrix -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/session/ -run TestWrite -count=1 -race`
- **Per wave merge:** `go test ./... -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/session/dataout_test.go` -- covers WRITE-01 through WRITE-05 plus matrix test
- [ ] No framework install needed -- stdlib testing is available

## Common Protocol Details for Implementation

### DataSN Numbering (Claude's Discretion)
**Recommendation:** DataSN resets to 0 for each burst. Per RFC 7143, each R2T-solicited sequence and the unsolicited sequence have independent DataSN counters starting at 0. This is confirmed by multiple sources and matches the per-burst tracking model.

### Unsolicited Data-Out TTT
Unsolicited Data-Out PDUs (sent before any R2T) use TTT=0xFFFFFFFF (reserved value indicating "no R2T"). This is the standard mechanism for the target to distinguish solicited from unsolicited data.

### Immediate Data Attachment (Claude's Discretion)
**Recommendation:** Immediate data goes in the SCSICommand PDU's data segment (the existing `ImmediateData []byte` field and `DataSegmentLen` in the BHS). This is how the existing Submit already works -- no architectural change needed for the attachment mechanism itself. The change is that Submit reads from cmd.Data (io.Reader) instead of using a pre-provided []byte slice.

### ExpectedDataTransferLength (Claude's Discretion)
**Recommendation:** Set from `cmd.ExpectedDataTransferLen` which the caller must set correctly. This field already exists on the Command struct and is already marshaled into the SCSICommand PDU. For writes, the caller knows the transfer size because the SCSI CDB encodes it (e.g., WRITE(10) has a transfer length field). No new logic needed -- just ensure callers set it.

### Buffer Offset Tracking (Claude's Discretion)
**Recommendation:** Track cumulative bytes sent per task. For immediate data: offset starts at 0, advances by immediate data length. For unsolicited: continues from immediate data offset. For each R2T: use R2T.BufferOffset (the target specifies where data should start). This handles the case where the target might request data in a non-sequential order (rare but RFC-compliant).

### Key Wire Format Details
- Data-Out opcode: 0x05 (already in `pdu.OpDataOut`)
- R2T opcode: 0x31 (already in `pdu.OpR2T`)
- Data-Out PDU has NO CmdSN field (it's not a new command, just data transfer)
- Data-Out PDU has ExpStatSN in bytes 28-31
- Data-Out Final bit is byte 1 bit 7 (standard Final position in BHS)
- ITT in Data-Out matches ITT from original SCSI Command
- LUN in Data-Out is reserved (set to 0)

## Open Questions

1. **Command.ImmediateData field removal timing**
   - What we know: Current Command has `ImmediateData []byte` which Submit uses directly. D-01 says write data comes from `io.Reader` field.
   - What's unclear: Should we remove `ImmediateData` now and break the existing API, or add `Data io.Reader` alongside it?
   - Recommendation: Remove `ImmediateData []byte`, add `Data io.Reader`. Update Submit to read immediate data from Data. This is a clean break since no external consumers exist yet. Update existing tests to use the new interface.

2. **io.Reader ownership after Submit returns**
   - What we know: Submit reads immediate data synchronously, then the io.Reader is passed to the task goroutine for unsolicited/R2T data.
   - What's unclear: Should Submit document that the io.Reader must remain valid until the Result is received?
   - Recommendation: Document clearly: "The io.Reader provided as cmd.Data must remain readable until the Result channel delivers. The caller must not close or modify the underlying data source until the write completes."

## Sources

### Primary (HIGH confidence)
- RFC 7143 Sections 11.7, 11.8, 11.2, 4.2, 13.9-13.17 -- Data-Out/R2T format, burst length rules, negotiation parameters
- Existing codebase: `internal/pdu/initiator.go` (DataOut), `internal/pdu/target.go` (R2T), `internal/session/session.go` (Submit, taskLoop), `internal/session/datain.go` (task pattern), `internal/login/params.go` (NegotiatedParams)

### Secondary (MEDIUM confidence)
- WebFetch of RFC 7143 for DataSN numbering scope (per-R2T reset) -- consistent with multiple sources but exact spec text paraphrased by AI
- WebFetch of gostor/gotgt conn.go for target R2T behavior -- partial view, details incomplete

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all stdlib, no new dependencies, existing types verified in codebase
- Architecture: HIGH - extends existing proven patterns (taskLoop, writeCh, Router), all building blocks exist
- Pitfalls: HIGH - derived from RFC 7143 protocol rules and verified against existing codebase patterns
- Protocol details (DataSN scope): MEDIUM - multiple sources agree on per-R2T reset, but derived from AI-paraphrased RFC text rather than exact spec quotes

**Research date:** 2026-04-01
**Valid until:** 2026-05-01 (stable protocol, no external dependency changes expected)
