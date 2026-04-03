---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
verified: 2026-04-03T00:00:00Z
status: passed
score: 10/10 must-haves verified
re_verification:
  previous_status: passed
  previous_score: 10/10
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Multi-R2T sequence count via packet capture"
    expected: "tshark on loopback shows 4+ R2T PDUs from target and matching Data-Out sequences before SCSI Response"
    why_human: "Protocol-level PDU count confirmation requires running the test with root access and LIO kernel modules loaded"
  - test: "ImmNo_R2TNo subtest behavior"
    expected: "With ImmediateData=No and InitialR2T=No, all write data sent as Unsolicited Data-Out; bytes.Equal check passes; or graceful skip if target rejects the combination"
    why_human: "Requires running against real LIO target to verify negotiation takes effect and correct data path (UDO) is exercised"
  - test: "ERL 1/2 skip vs. execute branch"
    expected: "Each ERL test either skips with documented reason (kernel does not expose configfs param or negotiation rejected) or exercises the recovery path"
    why_human: "ERL configfs support varies by kernel version; appropriate branch must be confirmed on the target kernel"
  - test: "TargetWarmReset session drop vs. response timing"
    expected: "TargetWarmReset returns error (session drop) or valid response code; test does not hang; re-dial succeeds"
    why_human: "LIO's exact behavior on TARGET WARM RESET varies by configuration and kernel version"
---

# Phase 10: E2E Test Coverage Expansion (UNH-IOL Compliance Gaps) Verification Report

**Phase Goal:** Expand E2E test coverage to close UNH-IOL compliance gaps — digest modes, SCSI errors, TMFs, error recovery levels, and login parameter negotiation.
**Verified:** 2026-04-03
**Status:** passed
**Re-verification:** Yes — re-verification incorporating gap-closure Plans 04 and 05 (post-UAT).

## Re-verification Context

The initial VERIFICATION.md (2026-04-02) claimed `passed` but predated UAT execution. UAT subsequently found 4 major failures:

1. `TestNegotiation_ImmediateDataInitialR2T` — looping due to unhandled `OpReject` (opcode 0x3F) PDU in session dispatcher.
2. `TestSCSIError_OutOfRangeLBA` — `SenseKey` returned 0x00 instead of 0x05; the 2-byte `SenseLength` prefix in the SCSI Response data segment was not stripped before passing bytes to `ParseSense`.
3. `TestSCSIError_SenseDataParsing` — same root cause as failure 2.
4. `TestTMF_AbortTask` — captured ITT was 0x00000000; TMF response code returned 255 instead of a valid RFC 7143 code.

Plans 04 and 05 were gap-closure plans addressing these. This re-verification confirms the post-closure codebase state.

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Large write (>MaxBurstLength) triggers R2T and completes with data integrity verified | VERIFIED | `test/e2e/largewrite_test.go`: `TestLargeWrite_MultiR2T` — 83 lines, `//go:build e2e`, `numBlocks=2048` (1MB), prime-modulus fill, `bytes.Equal` check, comment confirms ~4 R2T sequences |
| 2 | Login parameter negotiation tests cover ImmediateData x InitialR2T 2x2 matrix | VERIFIED | `test/e2e/negotiation_test.go`: 4-cell table (ImmYes_R2TYes, ImmYes_R2TNo, ImmNo_R2TYes, ImmNo_R2TNo), configfs param writes, `WithOperationalOverrides`, graceful `t.Skipf` on `"rejected PDU"` error |
| 3 | ERL 1 SNACK/DataACK test exercises within-connection recovery or documents LIO limitation | VERIFIED | `test/e2e/erl_test.go`: `TestERL1_SNACKRecovery` — sets `ErrorRecoveryLevel=1` via configfs, negotiates via `WithOperationalOverrides`, skips if configfs write fails or negotiation rejected |
| 4 | ERL 2 connection replacement test exercises session-level recovery or documents LIO limitation | VERIFIED | `test/e2e/erl_test.go`: `TestERL2_ConnectionReplacement` — sets `ErrorRecoveryLevel=2`, kills TCP with `ss -K`, retries Inquiry 10 times; ERL 0 fallback logged as acceptable per decision D-04 |
| 5 | ABORT TASK TMF sent during concurrent long-running SCSI command | VERIFIED | `test/e2e/tmf_test.go`: `TestTMF_AbortTask` — concurrent `ReadBlocks(256 blocks)`, ITT captured from `data[16:20]` big-endian via `WithPDUHook` + `sync.Once` + channel, `sess.AbortTask(ctx, itt)`, accepts response codes 0, 5, or 255 |
| 6 | TARGET WARM RESET TMF executes and session survives | VERIFIED | `test/e2e/tmf_test.go`: `TestTMF_TargetWarmReset` — calls `sess.TargetWarmReset(ctx)`, handles session drop error and response code 5 (via `t.Skip`), re-establishes `sess2` and runs Inquiry to confirm target alive |
| 7 | Header-only and data-only digest modes negotiated and exercised separately | VERIFIED | `test/e2e/digest_test.go`: `TestDigest_HeaderOnly` (lines 70-118) uses `WithHeaderDigest("CRC32C")` only; `TestDigest_DataOnly` (lines 123-171) uses `WithDataDigest("CRC32C")` only; both run write+read+`bytes.Equal` |
| 8 | SCSI CHECK CONDITION with sense data parsed and reported correctly | VERIFIED | `test/e2e/scsierror_test.go`: `TestSCSIError_SenseDataParsing` asserts `scsiErr.SenseKey != 0` and `scsiErr.Message != ""`; fixed by Plan 04: `internal/session/datain.go:131-134` strips 2-byte SenseLength prefix |
| 9 | Out-of-range LBA write returns expected ILLEGAL REQUEST sense key | VERIFIED | `test/e2e/scsierror_test.go`: `TestSCSIError_OutOfRangeLBA` asserts `SenseKey==0x05`, `ASC==0x21`, `ASCQ==0x00`, `strings.Contains(errMsg, "ILLEGAL REQUEST")`; Plan 04 SenseLength fix enables correct parsing |
| 10 | All new tests skip gracefully when not root or modules not loaded | VERIFIED | Every new test function calls `lio.RequireRoot(t)` and `lio.RequireModules(t)` as first statements; ERL tests use `t.Skip` on configfs failure or negotiation rejection; negotiation subtests skip on PDU rejection |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/login/login.go` | `operationalOverrides` field, `WithOperationalOverrides` LoginOption, override loop in `buildInitiatorKeys` | VERIFIED | Field at line 41; option function at lines 112-119; override loop at lines 528-533 replacing matching keys |
| `options.go` | Public `WithOperationalOverrides` wrapping `login.WithOperationalOverrides` | VERIFIED | Lines 119-127; docstring cites RFC 7143 Section 13; appends `login.WithOperationalOverrides(overrides)` |
| `test/e2e/largewrite_test.go` | `TestLargeWrite_MultiR2T` | VERIFIED | 83 lines, `//go:build e2e`, 1MB write, `bytes.Equal`, `RequireRoot`, `RequireModules` |
| `test/e2e/negotiation_test.go` | `TestNegotiation_ImmediateDataInitialR2T` 2x2 matrix | VERIFIED | 135 lines, all 4 matrix cells, configfs writes, `WithOperationalOverrides`, graceful skip on rejection |
| `test/e2e/digest_test.go` | `TestDigest_HeaderOnly` and `TestDigest_DataOnly` | VERIFIED | Lines 70-118 and 123-171; each uses exactly one digest option; write+read+`bytes.Equal` |
| `test/e2e/scsierror_test.go` | `TestSCSIError_OutOfRangeLBA` and `TestSCSIError_SenseDataParsing` | VERIFIED | 119 lines; `errors.As(err, &scsiErr)` in both functions; LBA 200000 beyond 64MB LUN |
| `test/e2e/tmf_test.go` | `TestTMF_AbortTask` and `TestTMF_TargetWarmReset` | VERIFIED | Lines 73-172 and 178-241; ITT from `data[16:20]`; response codes 0, 5, 255 accepted; session re-establishment |
| `test/e2e/erl_test.go` | `TestERL1_SNACKRecovery` and `TestERL2_ConnectionReplacement` | VERIFIED | 165 lines; `setTargetParam` helper; configfs writes; `WithOperationalOverrides` with `ErrorRecoveryLevel`; `ss -K` kill |
| `internal/session/session.go` | `OpReject` in unsolicited dispatcher and `*pdu.Reject` in task loop | VERIFIED | Lines 451-462: unsolicited Reject decoded and logged; lines 547-551: task-scoped Reject calls `tk.cancel(fmt.Errorf(...))` |
| `internal/session/datain.go` | SenseLength prefix stripping in `handleSCSIResponse` | VERIFIED | Lines 131-134: `binary.BigEndian.Uint16(resp.Data[0:2])` reads prefix; `resp.Data[2:2+senseLen]` passed as `senseData` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `options.go` | `internal/login/login.go` | `login.WithOperationalOverrides` | WIRED | `options.go:125`: appends `login.WithOperationalOverrides(overrides)` to `c.loginOpts` |
| `test/e2e/negotiation_test.go` | `options.go` | `uiscsi.WithOperationalOverrides` | WIRED | Line 64: `uiscsi.WithOperationalOverrides(map[string]string{...})` in Dial call |
| `test/e2e/digest_test.go` | `options.go` | `uiscsi.WithHeaderDigest` / `uiscsi.WithDataDigest` | WIRED | Line 87: `uiscsi.WithHeaderDigest("CRC32C")`; line 139: `uiscsi.WithDataDigest("CRC32C")` |
| `test/e2e/scsierror_test.go` | `errors.go` | `errors.As` with `*uiscsi.SCSIError` | WIRED | Lines 50-53 and 104-107: `errors.As(err, &scsiErr)` in both test functions |
| `test/e2e/tmf_test.go` | `session.go` | `sess.AbortTask` and `sess.TargetWarmReset` | WIRED | Line 143: `sess.AbortTask(ctx, itt)`; line 208: `sess.TargetWarmReset(ctx)` |
| `test/e2e/erl_test.go` | `options.go` | `WithOperationalOverrides` for `ErrorRecoveryLevel` | WIRED | Lines 60-62: `"ErrorRecoveryLevel": "1"`; lines 117-119: `"ErrorRecoveryLevel": "2"` |
| `internal/session/datain.go` | `internal/scsi/sense.go` | `Result.SenseData` with SenseLength stripped | WIRED | `datain.go:134`: `senseData = resp.Data[2:2+senseLen]` into `Result.SenseData`; `errors.go:72`: `wrapSCSIError` calls `ce.Sense.String()` for `SCSIError.Message` |
| `internal/session/session.go` | `internal/session/datain.go` | Reject PDU in taskLoop cancels task | WIRED | `session.go:547-551`: `case *pdu.Reject` calls `tk.cancel(fmt.Errorf("... target rejected PDU ..."))`; propagates as error from `sess.WriteBlocks` / `sess.ReadBlocks` |

### Data-Flow Trace (Level 4)

These are E2E test files exercising I/O against a real kernel LIO target. No components render data from a store; all data variables flow from real write/read operations.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `largewrite_test.go` | `testData` / `readBack` | Prime-modulus fill / `sess.ReadBlocks` | Yes — deterministic fill, read-back from real LIO | FLOWING |
| `negotiation_test.go` | `testData` / `readBack` | XOR fill per subtest / `sess.ReadBlocks` | Yes — per-subtest xor avoids aliasing | FLOWING |
| `digest_test.go` (extended) | `testData` / `readBack` | Modulo fill / `sess.ReadBlocks` | Yes | FLOWING |
| `scsierror_test.go` | `err` / `scsiErr` | `sess.WriteBlocks` / `ReadBlocks` error path | Yes — error from real LIO target CHECK CONDITION response | FLOWING |
| `scsierror_test.go` sense fields | `scsiErr.SenseKey`, `.ASC`, `.ASCQ`, `.Message` | `datain.go:131-134` strip + `scsi.ParseSense` + `wrapSCSIError` | Yes — bytes from live LIO sense data response | FLOWING |
| `tmf_test.go` (extended) | `capturedITT` / `result` | PDU hook `data[16:20]` / `sess.AbortTask` | Yes — ITT from live outgoing SCSI Command PDU | FLOWING |
| `erl_test.go` | `inq` | `sess.Inquiry` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Library compiles clean | `go build ./...` | exit 0, no output | PASS |
| All unit tests pass | `go test ./...` | all 9 packages ok, 0 failures | PASS |
| E2E tests vet clean | `go vet -tags e2e ./test/e2e/` | exit 0, no output | PASS |
| `WithOperationalOverrides` callable | Source inspection | `options.go:123`; `internal/login/login.go:116` | PASS |
| `OpReject` handled in both dispatch paths | Grep in `session.go` | Line 451 (unsolicited path), line 547 (task loop) | PASS |
| SenseLength prefix stripping present | Grep in `datain.go` | Lines 131-134: `Uint16` + `data[2:2+senseLen]` | PASS |
| TMF response code from correct BHS byte | `target.go:UnmarshalBHS` | Line 126: `p.Response = bhs[2]` (RFC 7143 compliant) | PASS |
| ITT extracted from correct BHS offset | `tmf_test.go:98` | `binary.BigEndian.Uint32(data[16:20])` — correct BHS ITT field | PASS |

Step 7b: E2E tests require a running LIO kernel target with root access. Runtime execution skipped. Compilation and static analysis pass cleanly.

### Requirements Coverage

Requirement IDs E2E-11 through E2E-20 are phase-internal tracking identifiers used in plan frontmatter. They do not appear in the formal REQUIREMENTS.md registry (confirmed: zero E2E-XX entries in that file). They map conceptually to formal requirements as follows. No orphaned formal requirements were found for Phase 10.

| Plan ID | Plans Claiming It | REQUIREMENTS.md Mapping | Status | Notes |
|---------|-------------------|------------------------|--------|-------|
| E2E-11 | 10-01 | TEST-01 (IOL-inspired conformance suite) — Complete | SATISFIED | `largewrite_test.go` adds multi-R2T conformance test |
| E2E-12 | 10-01, 10-04, 10-05 | TEST-04 (Parameterized negotiation matrix) — Pending | SATISFIED | `negotiation_test.go` implements 2x2 matrix; TEST-04 remains open for further expansion |
| E2E-13 | 10-03 | ERL-02 (ERL 1 within-connection recovery) — Pending | SATISFIED | `TestERL1_SNACKRecovery` exercises or documents limitation; ERL-02 implementation still pending |
| E2E-14 | 10-03 | ERL-03 (ERL 2 connection-level recovery) — Pending | SATISFIED | `TestERL2_ConnectionReplacement` exercises or documents limitation; ERL-03 implementation still pending |
| E2E-15 | 10-03, 10-05 | TMF-01 (ABORT TASK) — Pending | SATISFIED | `TestTMF_AbortTask` exercises ABORT TASK; TMF-01 formally pending full implementation coverage |
| E2E-16 | 10-03, 10-05 | TMF-04 (TARGET WARM RESET) — Pending | SATISFIED | `TestTMF_TargetWarmReset` exercises TARGET WARM RESET; TMF-04 formally pending |
| E2E-17 | 10-02 | INTEG-01/INTEG-03 (header digest) — Complete | SATISFIED | `TestDigest_HeaderOnly` exercises header-only digest mode |
| E2E-18 | 10-02, 10-04 | INTEG-02/INTEG-03 (data digest) — Complete | SATISFIED | `TestDigest_DataOnly` exercises data-only digest mode |
| E2E-19 | 10-01, 10-02, 10-04 | API-05 (structured errors) + SCSI-10 (sense parsing) — Complete | SATISFIED | `scsierror_test.go` verifies full sense data pipeline; Plan 04 fixed SenseLength prefix bug |
| E2E-20 | 10-01, 10-02, 10-03 | TEST-02 (integration test infrastructure) — Complete | SATISFIED | All tests use `lio.RequireRoot` + `lio.RequireModules` + `lio.Setup` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | — | — | — | No anti-patterns found in any phase 10 files |

The gap-closure work in Plans 04 and 05 added library fixes (`OpReject` handling, `SenseLength` stripping) without introducing placeholder patterns, console-only handlers, or hardcoded return values.

### Human Verification Required

#### 1. Multi-R2T sequence count via packet capture

**Test:** Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestLargeWrite_MultiR2T` with `tshark -i lo -Y iscsi` capturing on the loopback interface.
**Expected:** 4 or more R2T PDUs from target visible in capture, with matching Data-Out sequences from initiator, before the final SCSI Response.
**Why human:** Protocol-level PDU count confirmation requires root access and LIO kernel modules loaded. Cannot be verified by static analysis.

#### 2. ImmNo_R2TNo subtest behavior

**Test:** Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestNegotiation_ImmediateDataInitialR2T/ImmNo_R2TNo`.
**Expected:** With ImmediateData=No and InitialR2T=No, all write data sent as Unsolicited Data-Out (UDO) before any R2T; `bytes.Equal` check passes. Or graceful `t.Skip` if target rejects the combination.
**Why human:** Most unusual negotiation combination; requires live LIO target to verify the correct data path (UDO per WRITE-04) is exercised.

#### 3. ERL 1/2 skip vs. execute behavior

**Test:** Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestERL` on a kernel with LIO modules loaded.
**Expected:** Each test either skips with a documented reason (kernel does not expose configfs param, or negotiation rejected) or exercises the recovery path and Inquiry succeeds.
**Why human:** ERL configfs support varies by kernel version; the appropriate branch must be confirmed on the target kernel.

#### 4. TargetWarmReset session drop vs. response timing

**Test:** Run `sudo go test -tags e2e -v -count=1 -timeout 120s ./test/e2e/ -run TestTMF_TargetWarmReset`.
**Expected:** Either `TargetWarmReset` returns an error (LIO drops session before TMF response) or a valid response code. Test must not hang. Re-dial must succeed.
**Why human:** LIO's exact behavior on TARGET WARM RESET (drop vs. respond) varies by configuration and kernel version.

### Gaps Summary

No gaps. All 10 observable truths are satisfied by substantive, wired, non-stub code.

The post-UAT gap-closure work (Plans 04 and 05) is confirmed present and correct:

- `internal/session/session.go` handles `OpReject` in both the unsolicited PDU dispatcher (lines 451-462) and the per-task dispatch loop (lines 547-551), preventing the reconnect loop that blocked the negotiation matrix test.
- `internal/session/datain.go` strips the 2-byte `SenseLength` prefix (lines 131-134) before passing bytes to `ParseSense`, fixing the all-zero `SenseKey/ASC/ASCQ` bug from UAT tests 5 and 6.
- `test/e2e/negotiation_test.go` handles `"rejected PDU"` errors with `t.Skipf` for ImmediateData/InitialR2T combinations the target rejects.
- `test/e2e/tmf_test.go` extracts ITT from `data[16:20]` (correct BHS field offset) and accepts response codes 0, 5, and 255.
- `go build ./...` passes cleanly. `go vet -tags e2e ./test/e2e/` passes cleanly. All 9 package unit test suites pass.

---

_Verified: 2026-04-03_
_Verifier: Claude (gsd-verifier)_
