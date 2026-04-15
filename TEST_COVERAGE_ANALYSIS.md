# Test Coverage Analysis

Generated: 2026-04-15

## Coverage Summary

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/serial` | **100.0%** | Fully covered |
| `test/pducapture` | **100.0%** | Fully covered |
| `internal/scsi` | **92.8%** | Strong coverage, minor gaps |
| `internal/digest` | **92.3%** | Strong coverage |
| `internal/pdu` | **91.3%** | Strong coverage |
| `internal/transport` | **81.0%** | Good, but Conn accessors untested |
| `internal/login` | **80.4%** | Good, error paths under-tested |
| `internal/session` | **73.4%** | Core logic covered, async/renegotiation gaps |
| `test` (target infra) | **37.4%** | Test helpers — many only used by conformance/e2e |
| Root package (`uiscsi`) | **20.7%** | **Critical gap** — public API layer barely tested |
| **Total** | **72.0%** | |

## Recommended Improvements

### 1. Public API Layer (root package) — Priority: HIGH

**Current: 20.7% coverage**

The entire public-facing API has near-zero unit test coverage. Every method
on the four Ops types is at 0%:

- **`scsi_ops.go`**: All 19 SCSI methods (ReadBlocks, WriteBlocks, Inquiry,
  ReadCapacity, TestUnitReady, RequestSense, ReportLuns, ModeSense6/10,
  ModeSelect6/10, SynchronizeCache, Verify, WriteSame, Unmap, CompareAndWrite,
  StartStopUnit, PersistReserveIn, PersistReserveOut) — **all 0%**
- **`raw_ops.go`**: Execute, StreamExecute, WithDataIn, WithDataOut,
  execute, streamExecute — **all 0%**
- **`tmf_ops.go`**: AbortTask, AbortTaskSet, ClearTaskSet, LUNReset,
  TargetWarmReset, TargetColdReset — **all 0%**
- **`protocol_ops.go`**: Logout, SendExpStatSNConfirmation — **all 0%**
- **`session.go`**: initOps, Close, Drain, SCSI, TMF, Raw, Protocol,
  submitAndWait, submitAndCheck — **all 0%**
- **`types.go`**: convertTarget, convertInquiry, convertCapacity16,
  convertCapacity10, convertTMFResult, convertAsyncEvent, convertMetricEvent,
  Wait, WaitResidual, String — **all 0%**
- **`uiscsi.go`**: Discover — **0%**

**Why this matters:** These are the functions every consumer calls. The
internal packages are well-tested in isolation, but the glue layer that
wires internal types to public types, converts errors, and handles edge
cases (residual overflow, empty sense data, CDB length validation) has no
dedicated test coverage. Bugs in type conversion, error wrapping, or option
propagation would ship undetected.

**Recommended tests:**

- **`session_test.go`**: Test `submitAndWait` and `submitAndCheck` with a
  mock `session.Session`. Cover the happy path, transport errors,
  CHECK CONDITION with sense data, residual overflow, and context
  cancellation.
- **`scsi_ops_test.go`**: Test at least ReadBlocks, Inquiry, ReadCapacity
  (including the 16→10 fallback), and ReportLuns through a mock session.
  Verify correct CDB construction and response parsing end-to-end across
  the public API boundary.
- **`raw_ops_test.go`**: Test Execute with empty CDB (error), oversized CDB
  (error), read-only, write-only, and bidirectional ops. Test
  StreamExecute's streaming reader contract.
- **`tmf_ops_test.go`**: Test each TMF function with a mock that verifies
  the correct TMF function code is submitted.
- **`types_test.go`**: Test all `convert*` functions with representative
  internal types. Test `Wait` and `WaitResidual` on `StreamResult`.
- **`options_test.go`**: Test that each `With*` option correctly populates
  the `dialConfig` (login opts and session opts). Especially test
  `WithMaxRecvDataSegmentLength`, `WithMaxBurstLength`, and
  `WithFirstBurstLength` which encode values as string overrides.

---

### 2. Async Event Handling (`internal/session/async.go`) — Priority: HIGH

**Current: `handleTargetRequestedLogout` 0%, `renegotiate` 0%, `applyRenegotiatedParams` 0%**

The async message dispatcher (`handleAsyncMsg`) is exercised by the
conformance tests, but the three most complex branches are untested:

- **`handleTargetRequestedLogout`**: Implements RFC 7143 S11.9.1 — the
  initiator MUST logout within Parameter3 seconds. Contains timer logic,
  DefaultTime2Wait delay, context cancellation on session close, and a
  fallback deadline. A bug here violates the RFC and could leave sessions
  hanging.
- **`renegotiate`**: Implements the Text Request/Response exchange for
  AsyncEvent code 4 (parameter renegotiation). Contains CmdSN acquisition,
  PDU construction, router registration, and response parsing. Completely
  untested.
- **`applyRenegotiatedParams`**: Parses renegotiated key-values and updates
  session params under a mutex. Untested.

**Recommended tests:**

- Use `testing/synctest` (already used elsewhere in the session package) to
  test `handleTargetRequestedLogout` with various Parameter3 deadlines and
  DefaultTime2Wait values, including the case where the session is closed
  during the wait.
- Test `renegotiate` with a mock transport that feeds back a TextResp PDU.
  Verify the negotiated params are applied. Test context timeout.
- Test `applyRenegotiatedParams` directly with various key-value
  combinations, including unknown keys (should be silently ignored).

---

### 3. Transport `Conn` Accessors — Priority: MEDIUM

**Current: 11 methods at 0% in `internal/transport/conn.go`**

All getter/setter methods on `Conn` are untested: `NewConnFromNetConn`,
`DigestHeader`, `DigestData`, `SetDeadline`, `SetDigests`, `SetMaxRecvDSL`,
`MaxRecvDSL`, `SetDigestByteOrder`, `DigestByteOrder`.

These are simple accessors, but `DigestByteOrder` has a nil-check fallback
to `binary.LittleEndian` that should be verified.

**Recommended tests:**

- A single `TestConnAccessors` that creates a Conn via `NewConnFromNetConn`,
  exercises all setters, and asserts all getters return the expected values.
- Test `DigestByteOrder` default (nil → LittleEndian) and explicit
  BigEndian.

---

### 4. Login Error Messages — Priority: MEDIUM

**Current: `statusMessage` at 28.6%, `String` at 42.9% in `internal/login/errors.go`**

The login error formatting paths have many untested branches — each iSCSI
login status class/detail combination has a human-readable message. Missing
coverage means users may see cryptic raw error codes instead of helpful
messages for uncommon login failures (e.g., "target moved temporarily",
"session does not exist", "missing parameter").

**Recommended tests:**

- Table-driven test covering all `StatusClass` × `StatusDetail`
  combinations in `statusMessage`. There are only ~20 defined combinations.
- Test `LoginError.String()` and `LoginError.Error()` formatting.

---

### 5. Login Options — Priority: MEDIUM

**Current: `WithInitiatorName`, `WithSessionType`, `WithISID`, `WithOperationalOverrides`, `WithTSIH` all at 0%**

Five login options are never exercised in unit tests. While some may be
covered by conformance tests (which show `[no statements]` in coverage
since they're in a separate test binary), there are no unit-level tests
verifying these options correctly configure the login state machine.

**Recommended tests:**

- Test each option by calling `Login()` with a mock TCP connection and
  verifying the login PDU contains the expected fields (ISID, TSIH,
  InitiatorName, SessionType).
- Test `WithOperationalOverrides` propagation into negotiation keys.

---

### 6. SCSI ModeSelect Commands — Priority: MEDIUM

**Current: `ModeSelect6` 0%, `ModeSelect10` 0% in `internal/scsi/modesense.go`**

The ModeSense commands are tested but the corresponding ModeSelect builders
are not. These construct CDBs for write-direction mode page operations.

**Recommended tests:**

- Test ModeSelect6 and ModeSelect10 CDB construction with known inputs.
  Verify opcode, PF bit, parameter list length, and data payload.

---

### 7. PDU SAM LUN Encoding — Priority: MEDIUM

**Current: `EncodeSAMLUN` 0%, `DecodeSAMLUN` 0% in `internal/pdu/header.go`**

SAM LUN encoding/decoding converts between flat LUN IDs and the 8-byte
SAM-5 wire format. These are used for LUN addressing in PDU headers.

**Recommended tests:**

- Round-trip test: encode a LUN, decode it, verify it matches.
- Test boundary values: LUN 0, LUN 255, LUN 256 (peripheral device
  addressing method vs. flat space), LUN 16383.

---

### 8. Transport Router Methods — Priority: LOW

**Current: `RegisterPersistent` 0%, `AllocateITT` 0%, `UnregisterAndClose` 0%**

Three router methods are untested. `RegisterPersistent` is used for
persistent PDU routing (e.g., NOP-In), `AllocateITT` for task tag
management, and `UnregisterAndClose` for cleanup.

**Recommended tests:**

- Test `RegisterPersistent` → verify dispatched PDU arrives on the
  persistent channel.
- Test `AllocateITT` → verify ITT uniqueness and `PendingCount` tracking.
- Test `UnregisterAndClose` → verify channel is closed and ITT is freed.

---

### 9. Transport `WritePump` — Priority: LOW

**Current: 60% in `internal/transport/pump.go`**

The write pump has untested error paths. Since the write pump is a
long-running goroutine that serializes PDUs to the wire, errors here cause
session failures.

**Recommended tests:**

- Test WritePump with a writer that returns errors (simulate network
  failure). Verify the pump shuts down and reports the error.
- Test WritePump with digest-enabled connections.

---

### 10. Session `SendExpStatSNConfirmation` — Priority: LOW

**Current: 0% in `internal/session/keepalive.go`**

This sends a NOP-Out with no expected response, used to confirm sequence
numbers. It's an RFC 7143 mechanism for sequence number synchronization.

**Recommended tests:**

- Test that calling `SendExpStatSNConfirmation` sends a NOP-Out PDU with
  ITT = 0xFFFFFFFF (no response expected) and the correct ExpStatSN.

---

## Structural Observations

### What's working well

- **Internal packages are well-tested**: `internal/scsi` (92.8%),
  `internal/pdu` (91.3%), `internal/digest` (92.3%) have strong coverage.
- **Conformance test suite is thorough**: 87 wire-level tests validating
  RFC 7143 compliance.
- **Fuzzing is in place**: 6 fuzz test files covering parsing-heavy code
  in pdu, scsi, login, session, and transport.
- **Goroutine leak detection**: `go.uber.org/goleak` in TestMain prevents
  goroutine leaks from going unnoticed.
- **Race detection**: `-race` enabled in CI.

### Coverage gap pattern

The coverage gap follows a clear architectural boundary: **internal
packages are well-tested, but the public API "glue" layer is not**. This
is a common pattern in Go libraries where internal packages have thorough
unit tests, but the thin public wrapper assumes correctness by composition.

The risk is that bugs in error wrapping, type conversion, option
propagation, and edge-case handling at the API boundary go undetected.
These are exactly the bugs that affect library consumers.

### No coverage thresholds enforced

The CI pipeline generates `coverage.out` and uploads it as an artifact,
but there is no coverage threshold gate. Adding a minimum coverage
threshold (e.g., 70% overall, 50% per-package) to CI would prevent
coverage regression as new code is added.
