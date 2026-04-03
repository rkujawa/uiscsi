# Phase 7: Public API, Observability, and Release - Context

**Gathered:** 2026-04-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Expose the internal iSCSI library as a clean, Go-idiomatic public API with high-level typed functions and raw CDB pass-through. Build IOL-inspired conformance test suite with automated test infrastructure. Write godoc, README, and four worked examples. OBS-01/02/03 are already complete from Phase 06.1 — this phase focuses on API (API-01 through API-05), testing (TEST-01, TEST-02), and documentation (DOC-01 through DOC-05).

</domain>

<decisions>
## Implementation Decisions

### Public API Package Structure
- **D-01:** Top-level `github.com/rkujawa/uiscsi` package exports the public API. Consumers write `uiscsi.Dial()`, `uiscsi.ReadBlocks()`, etc.
- **D-02:** Public wrapper types in the uiscsi package (`Session`, `Result`, `Target`, `SenseInfo`, etc.) wrap internal types. Internal types stay internal — consumers never import `internal/`.
- **D-03:** Two-step connection flow: `uiscsi.Discover(ctx, addr)` returns targets, then `uiscsi.Dial(ctx, addr, opts...)` creates a session. Discovery is optional — Dial works directly with a target name.

### High-Level vs Low-Level API Boundary
- **D-04:** Primary block I/O uses `[]byte` — `ReadBlocks` returns `[]byte`, `WriteBlocks` takes `[]byte`. Separate streaming functions (`StreamRead`/`StreamWrite` or similar) return `io.Reader` / take `io.Reader` for large transfers.
- **D-05:** Raw CDB pass-through is a method on Session: `sess.Execute(ctx, lun, cdb, opts...)` takes raw CDB bytes and returns raw response + status. Options like `WithDataIn(allocLen)` control transfer direction.
- **D-06:** Typed error hierarchy: `SCSIError` (wraps sense data + status), `TransportError` (wraps iSCSI/TCP errors), `AuthError` (login failures). All implement `error`. Consumers use `errors.As()` to extract detail.

### E2E Test Infrastructure
- **D-07:** Tiered test approach: custom mock target (in Go) for PDU-level conformance tests, gotgt embedded target for full-stack integration tests.
- **D-08:** IOL structure-inspired conformance suite: organize by IOL test categories (login, full-feature, error recovery, task management), write Go-idiomatic table-driven tests, use IOL test names/numbers as comments for traceability.
- **D-09:** Test infrastructure lives in top-level `test/` directory. `test/target.go` for mock target, `test/conformance/` for IOL-inspired tests, `test/integration/` for gotgt-based E2E.

### Documentation and Examples
- **D-10:** Four runnable example programs in `examples/` directory (DOC-02 through DOC-05): discover+login+read, write+verify, raw CDB pass-through, error handling and recovery. Each is a standalone `main()` program.
- **D-11:** Godoc testable examples (`func ExampleDial()`, `func ExampleSession_ReadBlocks()`, etc.) for API reference discoverability, in addition to the `examples/` programs.
- **D-12:** README.md with overview, quick start, feature list, links to examples, API reference (godoc link), requirements, and license. Standard Go library README structure.

### Claude's Discretion
- Exact method signatures for high-level functions (parameter order, option names)
- Which internal types need public equivalents vs which stay opaque
- Mock target implementation detail (handler registration pattern, PDU sequence control)
- Streaming API naming (StreamRead vs ReadStream vs ReadTo)
- Godoc example selection (which functions get Example tests)
- README sections beyond the agreed structure

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### RFC and Protocol
- RFC 7143 — iSCSI protocol specification, drives all PDU/session/login behavior
- RFC 1994 — CHAP authentication used during login

### Existing Internal API (wrap these for public API)
- `internal/session/types.go` — `Session`, `Command`, `Result`, `SessionOption`, `AsyncEvent`, `DiscoveryTarget`, `TMFResult` types
- `internal/session/session.go` — `NewSession()`, `Submit()`, `Close()` — core session lifecycle
- `internal/session/discovery.go` — `SendTargets()` — discovery implementation
- `internal/session/tmf.go` — `AbortTask()`, `LUNReset()`, etc. — task management
- `internal/session/logout.go` — `Logout()`, `LogoutConnection()` — teardown
- `internal/scsi/commands.go` — CDB builders: `TestUnitReady`, `RequestSense`, `ReportLuns`, `Verify10/16`, `CompareAndWrite`, `StartStopUnit`
- `internal/scsi/readwrite.go` — `Read10/16`, `Write10/16` CDB builders
- `internal/scsi/inquiry.go` — `Inquiry`, `InquiryVPD`, `ParseInquiry`
- `internal/scsi/capacity.go` — `ReadCapacity10/16`, `ParseReadCapacity10/16`
- `internal/scsi/sense.go` — Sense data parsing, ASC/ASCQ lookup
- `internal/scsi/provisioning.go` — `WriteSame10/16`, `Unmap`
- `internal/scsi/reservations.go` — `PersistReserveIn/Out`

### Observability (already complete — Phase 06.1)
- `internal/session/metrics.go` — `WithPDUHook`, `WithMetricsHook`, `MetricEvent`
- `internal/digest/errors.go` — `DigestError` type
- `internal/pdu/stringer.go` — PDU `String()` methods for debugging

### Login and Transport
- `internal/login/login.go` — `Login()`, `LoginOption`, `NegotiatedParams`
- `internal/transport/framer.go` — `RawPDU`, `ReadRawPDU`, `WriteRawPDU`
- `internal/transport/pump.go` — `ReadPump`, `WritePump` with hooks/logger

### Prior Research
- `.planning/research/e2e-target-infrastructure.md` — E2E testing strategy context
- `.planning/research/e2e-testing-approaches.md` — Why observability is prerequisite for E2E

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/scsi` package: 19 CDB builders + parsers ready to wrap as high-level functions
- `internal/session.Session`: Full session lifecycle (submit, close, reconnect, TMF) ready to wrap
- `SessionOption` functional options pattern: reuse for public `uiscsi.WithCHAP()`, `uiscsi.WithLogger()`, etc.
- `session_test.go` mock target goroutines: pattern for building the test/ mock target
- `internal/pdu/stringer.go`: 18 String() methods for debug output in tests and examples

### Established Patterns
- Functional options (`SessionOption`) for all configuration
- CDB builders return `session.Command` — public API wraps this pattern
- `context.Context` on all operations (already in internal API)
- io.Reader for write data, bytes.Buffer for read reassembly
- Table-driven tests with subtests throughout the codebase

### Integration Points
- Root package `github.com/rkujawa/uiscsi` wraps `internal/session` + `internal/scsi` + `internal/login`
- `uiscsi.Dial()` orchestrates: TCP connect → `transport.NewConn()` → `login.Login()` → `session.NewSession()`
- `uiscsi.Discover()` creates a discovery session type → `SendTargets()` → parse results
- Test mock target connects to `internal/transport` and `internal/pdu` for PDU handling

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 07-public-api-observability-and-release*
*Context gathered: 2026-04-01*
