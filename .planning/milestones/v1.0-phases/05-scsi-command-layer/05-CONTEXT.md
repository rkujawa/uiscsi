# Phase 5: SCSI Command Layer - Context

**Gathered:** 2026-04-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Typed Go API for constructing SCSI CDBs and parsing responses. Covers 19 SCSI commands (core + extended), VPD page parsing, and sense data interpretation. Built as a standalone layer on top of Session's Submit/Result transport. No changes to Session itself.

</domain>

<decisions>
## Implementation Decisions

### Package Structure
- **D-01:** New `internal/scsi/` package. Clean separation from iSCSI transport — scsi/ owns CDB building, response parsing, sense data. Session stays transport-only. Mirrors how libiscsi separates SCSI from iSCSI.
- **D-02:** Files organized by command group: inquiry.go, readwrite.go, capacity.go, sense.go, provisioning.go (WRITE SAME, UNMAP), reservations.go (PERSISTENT RESERVE), vpd.go, etc. Related commands share a file (e.g., READ 10 and READ 16 in readwrite.go). ~7-8 source files.

### CDB Builder API Style
- **D-03:** Plain functions returning `session.Command`. `scsi.Read10(lba, blocks)` returns a Command with CDB filled in. Required parameters are positional args. Simple, discoverable, Go-idiomatic.
- **D-04:** Optional flags via functional options. `scsi.Write10(lba, blocks, data, scsi.WithFUA(), scsi.WithDPO())`. Consistent with Session's existing `WithKeepaliveInterval` pattern. Simple calls need no options.

### Response Parsing Depth
- **D-05:** Parse commonly used fields into typed structs, expose raw bytes for niche fields. INQUIRY: device type, vendor, product, revision + Raw. VPD 0x83: parsed Designators + Raw. VPD 0xB0: block limits fields. Covers 95% of use cases without parsing every SPC-4 bit.
- **D-06:** Sense data as typed struct with SenseKey enum, ASC/ASCQ uint8 pair, and String() method for human-readable descriptions ("MEDIUM ERROR: Unrecovered read error"). `Is(key SenseKey)` helper for programmatic checking. Both fixed and descriptor formats parsed.

### Session Integration
- **D-07:** Standalone `scsi` functions only — no methods on Session. `scsi.Read10()` builds a Command, caller does `sess.Submit(ctx, cmd)`. The scsi package has zero dependency on Session. Clean layering: scsi/ builds CDBs and parses responses, session/ transports them. Caller composes.
- **D-08:** Parse functions take `session.Result` directly. `scsi.ParseInquiry(result)` handles reading result.Data, checking status, and parsing. One-stop call — errors if status != GOOD or sense data indicates failure.

### Claude's Discretion
- Exact ASC/ASCQ string lookup table coverage (common vs exhaustive)
- Internal helper patterns for CDB byte packing
- Test fixture organization (golden bytes, table-driven, etc.)
- Whether to export the functional option type or keep it internal

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### SCSI specifications
- RFC 7143 (iSCSI) — Section 11.3-11.4 for SCSI Command/Response PDU format, sense data delivery
- SPC-4 (SCSI Primary Commands) — INQUIRY, TEST UNIT READY, REQUEST SENSE, REPORT LUNS, MODE SENSE, PERSISTENT RESERVE, START STOP UNIT command definitions
- SBC-3 (SCSI Block Commands) — READ, WRITE, READ CAPACITY, SYNCHRONIZE CACHE, WRITE SAME, UNMAP, VERIFY, COMPARE AND WRITE command definitions

### Existing codebase
- `internal/session/types.go` — Command and Result structs (the interface scsi/ targets)
- `internal/session/session.go` — Submit method signature
- `internal/pdu/initiator.go` — SCSICommand PDU with CDB [16]byte field
- `internal/pdu/ahs.go` — AHS extended CDB support for >16 byte CDBs

### Reference implementations
- `sahlberg/libiscsi` (C) — Study SCSI command construction patterns in src/scsi-command.c and response parsing

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.Command` struct with `CDB [16]byte`, `Data io.Reader`, `Read/Write bool`, `ExpectedDataTransferLen` — the target interface for all CDB builders
- `session.Result` with `Status`, `SenseData []byte`, `Data io.Reader`, `Err` — the input for all parse functions
- `encoding/binary.BigEndian` — already used throughout for network byte order, same pattern for CDB field packing

### Established Patterns
- Functional options pattern (`WithKeepaliveInterval`, `WithAsyncHandler`) — reuse for SCSI command options
- Table-driven tests with golden bytes — established in pdu/ package for BHS marshal/unmarshal
- `io.Reader` for data transfer — read path returns Reader, write path accepts Reader

### Integration Points
- `scsi.XxxCommand()` → `session.Command` — scsi package produces Commands that Session transports
- `session.Result` → `scsi.ParseXxx(result)` — scsi package consumes Results from Session
- No direct dependency between scsi/ and session/ internal state — clean interface boundary

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The API style previews shown during discussion (plain functions, functional options, Result-based parsing) capture the design intent.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-scsi-command-layer*
*Context gathered: 2026-04-01*
