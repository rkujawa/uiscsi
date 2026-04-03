---
phase: 11-audit-remediation-correctness-security-and-api-hardening
verified: 2026-04-03T00:00:00Z
status: human_needed
score: 17/17 must-haves verified
re_verification: false
human_verification:
  - test: "Verify that a non-auth login failure (StatusClass != 2) returned from a real iSCSI target wraps as TransportError, not AuthError at runtime"
    expected: "errors.As(err, &te) succeeds for *TransportError; errors.As(err, &ae) fails for *AuthError"
    why_human: "No integration test exercises this code path. uiscsi_test.go only tests unreachable-host TransportErrors. The else-branch in Dial() line 62 needs a target that returns StatusClass != 2 login failure."
  - test: "Verify that sending a SNACK with writeCh full actually times out with error (not blocks forever or panics)"
    expected: "After 5 seconds, sendSNACK returns non-nil error containing 'SNACK send timed out'"
    why_human: "No test exercises a full writeCh with SNACK delivery. Concurrency timing makes automated test fragile."
  - test: "Verify Execute() with 17-byte CDB returns error before any network I/O on a real session"
    expected: "err.Error() contains 'exceeds maximum 16 bytes'; no PDU sent to target"
    why_human: "No test function exists for AUDIT-10 CDB length validation. The plan described TestExecute_CDBTooLong but it was not implemented."
---

# Phase 11: Audit Remediation Verification Report

**Phase Goal:** Fix all 17 audit findings (AUDIT-1 through AUDIT-17) across correctness, security, and API hardening categories
**Verified:** 2026-04-03
**Status:** human_needed — all 17 code changes verified; 3 test coverage gaps need human confirmation
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | ReadRawPDU rejects PDUs exceeding MaxRecvDataSegmentLength | VERIFIED | `framer.go:51-52`: `if maxRecvDSL > 0 && dsLen > maxRecvDSL { return nil, fmt.Errorf(...) }` |
| 2  | newCHAPState returns error instead of panicking on entropy failure | VERIFIED | `chap.go:62`: signature `(*chapState, error)`; no `panic` calls in file |
| 3  | Non-auth login errors in Dial() are TransportError, not AuthError | VERIFIED | `uiscsi.go:60-62`: single `wrapAuthError` for StatusClass==2; else branch `&TransportError{Op: "login"}` |
| 4  | Digest byte order is configurable via DigestByteOrder on transport.Conn | VERIFIED | `conn.go:25,91-101`: field + getter + setter; `options.go:131-137`: `WithDigestByteOrder`; framer variadic parameter |
| 5  | SNACK timer goroutines obtain current writeCh via getter at send time | VERIFIED | `session.go:308-313`: `getWriteCh()` method; `snack.go:18,29`: getter function parameter threaded through |
| 6  | SNACK sends use blocking send with context timeout, not silent drop | VERIFIED | `snack.go:50-57`: `context.WithTimeout(context.Background(), 5*time.Second)` with select; no `default:` case |
| 7  | ERL 2 connection replacement keeps old ITT registered until reassignment confirmation | VERIFIED | `connreplace.go:121-137`: `Unregister(itt)` at line 137 is AFTER `sendTMF` at line 121; `Unregister(newITT)` on failure path at line 129 |
| 8  | submitAndCheck returns error on residual overflow | VERIFIED | `session.go:67-71`: `if result.Overflow { return nil, &SCSIError{...} }` |
| 9  | submitAndCheck reports unparseable sense data in SCSIError.Message | VERIFIED | `session.go:62`: `se.Message = fmt.Sprintf("sense data present but unparseable: %v", parseErr)` |
| 10 | Execute returns error when CDB exceeds 16 bytes | VERIFIED | `session.go:336-339`: `len(cdb) > 16` and `len(cdb) == 0` guards before network I/O |
| 11 | Reconnect backoff does not shift by negative value | VERIFIED | `recovery.go:71`: `1<<uint(max(0, attempt-1))` |
| 12 | SCSIError.Error() is consistent (documented non-wrapping) | VERIFIED | `errors.go:22-24`: comment "does not implement Unwrap() because it does not wrap an underlying error" |
| 13 | AuthError.Error() includes StatusClass and StatusDetail | VERIFIED | `errors.go:59`: `fmt.Sprintf("iscsi auth: %s (class=%d detail=%d)", e.Message, e.StatusClass, e.StatusDetail)` |
| 14 | WithAsyncHandler and WithPDUHook callbacks receive context.Context as first parameter | VERIFIED | `options.go:89,100`: signatures updated; `types.go:121-122`: field types updated; invoked via `context.Background()` at call sites |
| 15 | MockTarget logs unhandled opcodes instead of silently ignoring them | VERIFIED | `test/target.go:192-199`: `slog.Warn("mock target: unhandled opcode", ...)` with strict mode |
| 16 | encodeDataSegmentLength panics for values > 0xFFFFFF | VERIFIED | `bhs.go:18-20`: `if dsLen > 0xFFFFFF { panic(...) }` |
| 17 | UnmarshalAHS rejects unknown AHS types gracefully and enforces max length | VERIFIED | `ahs.go:86-97`: `dataLen > 16384` error return; unknown types accepted with comment for forward compat |

**Score:** 17/17 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/transport/framer.go` | MaxRecvDSL enforcement in ReadRawPDU | VERIFIED | `dsLen > maxRecvDSL` at line 51; variadic `digestByteOrder` parameter |
| `internal/transport/conn.go` | DigestByteOrder config; MaxRecvDSL getter | VERIFIED | `SetDigestByteOrder`, `DigestByteOrder()`, `MaxRecvDSL()` all present |
| `internal/login/chap.go` | Error return from newCHAPState | VERIFIED | Returns `(*chapState, error)`; `return nil, fmt.Errorf` at lines 73, 80 |
| `uiscsi.go` | Correct error wrapping in Dial else branch | VERIFIED | Line 62: `&TransportError{Op: "login", Err: err}` |
| `session.go` | Residual overflow, sense parse failure, CDB validation | VERIFIED | All three checks present and correct |
| `errors.go` | AuthError with status codes; SCSIError Unwrap comment | VERIFIED | `class=%d detail=%d` in Error(); comment at line 22 |
| `internal/session/recovery.go` | Safe backoff calculation | VERIFIED | `max(0, attempt-1)` at line 71 |
| `internal/pdu/bhs.go` | 24-bit validation in encodeDataSegmentLength | VERIFIED | `dsLen > 0xFFFFFF` panic at line 18 |
| `internal/pdu/ahs.go` | AHS type validation and max length check | VERIFIED | `dataLen > 16384` error at line 86; unknown type switch at lines 93-97 |
| `internal/session/session.go` | getWriteCh getter; context in callbacks | VERIFIED | `getWriteCh()` at line 308; `pduHook` called with `context.Background()` at line 369 |
| `internal/session/snack.go` | Blocking SNACK send with context timeout | VERIFIED | `context.WithTimeout` at line 50; getter function signature |
| `internal/session/connreplace.go` | ITT lifecycle fix | VERIFIED | Old ITT unregistered after TMF confirmation |
| `options.go` | context.Context in callbacks; WithDigestByteOrder | VERIFIED | Both present |
| `test/target.go` | Unhandled opcode logging + strict mode | VERIFIED | `slog.Warn` + `strict` field + `SetStrictMode` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/transport/framer.go` | `internal/transport/conn.go` | `maxRecvDSL` parameter passed to `ReadRawPDU` | WIRED | `conn.MaxRecvDSL()` called at `session.go:393` |
| `internal/login/chap.go` | `internal/login/login.go` | `newCHAPState` error propagation | WIRED | `login.go:176`: `chap, err := newCHAPState(...)` with error check |
| `internal/session/recovery.go` | `internal/session/session.go` | `getWriteCh` getter for current channel | WIRED | `session.go:308-313` defines getter; used throughout `snack.go` |
| `internal/session/snack.go` | `internal/session/session.go` | `writeCh` send with timeout via getter | WIRED | `snack.go:29`: parameter type `func() chan<- *transport.RawPDU` |
| `options.go` | `internal/session/types.go` | `context.Context` in callback type definitions | WIRED | `types.go:121-122` field types match `options.go:89,100` signatures |
| `internal/transport/conn.go` | `internal/transport/framer.go` | `digestByteOrder` passed through | WIRED | Variadic parameter; `conn.DigestByteOrder()` used at `session.go` level |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| AUDIT-1 | 11-01-PLAN.md | MaxRecvDSL enforcement — memory exhaustion DoS | SATISFIED | `framer.go:51-52` |
| AUDIT-2 | 11-01-PLAN.md | CHAP no panic on entropy failure | SATISFIED | `chap.go:62-80` |
| AUDIT-3 | 11-01-PLAN.md | Non-auth login errors wrapped as TransportError | SATISFIED | `uiscsi.go:62` |
| AUDIT-4 | 11-04-PLAN.md | Configurable digest byte order | SATISFIED | `conn.go`, `framer.go`, `options.go` |
| AUDIT-5 | 11-03-PLAN.md | Goroutine/channel leak during reconnect | SATISFIED | `session.go:308`, `snack.go:29` |
| AUDIT-6 | 11-03-PLAN.md | Silent SNACK drop when writeCh full | SATISFIED | `snack.go:50-57` |
| AUDIT-7 | 11-03-PLAN.md | ERL 2 ITT lifecycle bug | SATISFIED | `connreplace.go:121-137` |
| AUDIT-8 | 11-02-PLAN.md | Residual overflow detection | SATISFIED | `session.go:67-71` |
| AUDIT-9 | 11-02-PLAN.md | Unparseable sense data reporting | SATISFIED | `session.go:62` |
| AUDIT-10 | 11-02-PLAN.md | CDB length validation in Execute() | SATISFIED | `session.go:336-339` |
| AUDIT-11 | 11-02-PLAN.md | Reconnect backoff negative shift | SATISFIED | `recovery.go:71` |
| AUDIT-12 | 11-02-PLAN.md | SCSIError Unwrap documentation | SATISFIED | `errors.go:22-24` |
| AUDIT-13 | 11-02-PLAN.md | AuthError.Error() includes status codes | SATISFIED | `errors.go:59` |
| AUDIT-14 | 11-04-PLAN.md | context.Context in async/PDU hook callbacks | SATISFIED | `options.go:89,100`, `types.go:121-122` |
| AUDIT-15 | 11-04-PLAN.md | MockTarget unhandled opcode logging | SATISFIED | `test/target.go:192-199` |
| AUDIT-16 | 11-02-PLAN.md | encodeDataSegmentLength 24-bit validation | SATISFIED | `bhs.go:18-20` |
| AUDIT-17 | 11-02-PLAN.md | AHS type/length validation | SATISFIED | `ahs.go:86-97` |

**Orphaned requirements:** AUDIT-1 through AUDIT-17 are referenced in `ROADMAP.md` but not defined in `.planning/REQUIREMENTS.md`. These identifiers exist only in PLAN frontmatter and ROADMAP. This is a documentation gap — the requirements are fully covered by the four PLAN files, but REQUIREMENTS.md is incomplete for Phase 11.

### Anti-Patterns Found

No TODO, FIXME, placeholder comments, or empty implementations found in any of the 14 modified files.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/session/async.go` | 80 | `context.Background()` passed to asyncHandler instead of propagated ctx | Info | asyncHandler receives a context but it is always Background; cancellation cannot propagate from session to handler |
| `internal/session/session.go` | 369 | `context.Background()` passed to pduHook instead of propagated ctx | Info | Same issue as async.go — the context in the callback signature is present but not useful |

These are info-level observations. The AUDIT-14 requirement is "callbacks receive context.Context as first parameter" — the signatures are correct. The actual context value passed (`Background()`) is a separate concern not covered by AUDIT-14.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` succeeds | `go build ./...` | exit 0, no output | PASS |
| `go vet ./...` clean | `go vet ./...` | exit 0, no output | PASS |
| All tests pass | `go test ./...` | 10 packages pass | PASS |
| Race detector clean | `go test -race ./...` | 10 packages pass | PASS |
| TestReadRawPDU_ExceedsMaxRecvDSL | `go test ./internal/transport/ -run TestReadRawPDU_Exceeds` | PASS | PASS |
| TestNewCHAPState_ReturnsNoError | `go test ./internal/login/ -run TestNewCHAPState` | PASS | PASS |
| TestAuthError_ErrorIncludesStatusCodes | `go test -run TestAuthError_ErrorIncludesStatusCodes ./...` | PASS | PASS |
| TestEncodeDataSegmentLength_PanicsOnOverflow | `go test ./internal/pdu/ -run TestEncodeDataSegmentLength_Panics` | PASS | PASS |
| TestUnmarshalAHS_UnknownType | `go test ./internal/pdu/ -run TestUnmarshalAHS_Unknown` | PASS | PASS |
| TestUnmarshalAHS_ExcessiveLength | `go test ./internal/pdu/ -run TestUnmarshalAHS_Excessive` | PASS | PASS |

### Human Verification Required

#### 1. Non-Auth Login Error Classification (AUDIT-3)

**Test:** Set up a mock iSCSI target that responds to a login request with a Login Response PDU where StatusClass is 0 or 1 (not 2). Call `uiscsi.Dial()` and check the returned error type.
**Expected:** `errors.As(err, &te)` matches `*uiscsi.TransportError` with `Op == "login"`; `errors.As(err, &ae)` does not match `*uiscsi.AuthError`.
**Why human:** No integration test exists for this path. All existing tests either use unreachable hosts (TransportError at dial, not login) or succeed through full login. The `else` branch at `uiscsi.go:62` is untested by the test suite.

#### 2. SNACK Delivery Timeout Under Backpressure (AUDIT-6)

**Test:** Create a session with a small-buffered writeCh, trigger a SNACK retry while the write channel is saturated, and verify the error is returned after ~5 seconds.
**Expected:** `sendSNACK` returns non-nil error containing "SNACK send timed out after 5s"; no goroutine block or panic.
**Why human:** Timing-sensitive concurrency test. Automated tests for this behavior are brittle and the snack_test.go only covers the happy path (writeCh always has capacity).

#### 3. CDB Length Validation Test Coverage (AUDIT-10)

**Test:** Call `sess.Execute(ctx, lun, make([]byte, 17), nil, 0)` on a valid session and verify the error.
**Expected:** `err.Error()` contains "exceeds maximum 16 bytes"; no PDU is sent to the target.
**Why human:** The plan described `TestExecute_CDBTooLong` but no such test was implemented. The production code check exists and is correct (verified by code inspection), but test coverage is absent. A human should confirm the test is intentionally deferred or add it.

### Gaps Summary

No gaps blocking goal achievement. All 17 AUDIT findings have correct production code implementations verified against the codebase. The phase goal is achieved.

Three items require human verification:
1. AUDIT-3 has no integration test exercising the non-auth login error path
2. AUDIT-6 SNACK timeout behavior is not covered by automated tests
3. AUDIT-10 CDB length validation test was not implemented despite being specified in 11-02-PLAN.md

These are test coverage gaps, not correctness gaps. The code changes are present and correct.

One documentation gap: AUDIT-1 through AUDIT-17 are not defined in `.planning/REQUIREMENTS.md`. They exist only as requirement IDs in PLAN frontmatter and a collective reference in ROADMAP.md. This does not affect the implementation but should be remedied for traceability.

---

_Verified: 2026-04-03_
_Verifier: Claude (gsd-verifier)_
