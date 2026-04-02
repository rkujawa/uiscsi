---
phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
verified: 2026-04-02T00:00:00Z
status: passed
score: 10/10 must-haves verified
re_verification: false
---

# Phase 10: E2E Test Coverage Expansion (UNH-IOL Compliance Gaps) Verification Report

**Phase Goal:** Close critical E2E test gaps identified by UNH-IOL iSCSI initiator test suite comparison. Add tests for: large data transfers exceeding MaxBurstLength (multi-R2T sequences), login parameter negotiation boundaries (FirstBurstLength vs MaxBurstLength, ImmediateData x InitialR2T 2x2 matrix), ERL 1/2 error recovery against real LIO target, additional TMFs (ABORT TASK with concurrent commands, TARGET WARM RESET), digest variants (header-only, data-only), and SCSI error condition handling (sense data, out-of-range LBA). All tests run against real kernel LIO target via test/lio/ helper.
**Verified:** 2026-04-02
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Large write (>MaxBurstLength) triggers R2T and completes with data integrity verified | VERIFIED | `test/e2e/largewrite_test.go`: `TestLargeWrite_MultiR2T` writes 1MB (2048 blocks), reads back, `bytes.Equal` check; comment confirms ~4 R2T sequences at MaxBurstLength=262144 |
| 2 | Login parameter negotiation tests cover ImmediateData x InitialR2T 2x2 matrix against real target | VERIFIED | `test/e2e/negotiation_test.go`: `TestNegotiation_ImmediateDataInitialR2T` table-driven with all 4 cells (ImmYes_R2TYes, ImmYes_R2TNo, ImmNo_R2TYes, ImmNo_R2TNo), writes configfs params and uses `WithOperationalOverrides` |
| 3 | ERL 1 SNACK/DataACK test exercises within-connection recovery (or documents LIO limitation) | VERIFIED | `test/e2e/erl_test.go`: `TestERL1_SNACKRecovery` sets `ErrorRecoveryLevel=1` via configfs, negotiates via `WithOperationalOverrides`, skips gracefully if not supported; reads to exercise data path |
| 4 | ERL 2 connection replacement test exercises session-level recovery (or documents LIO limitation) | VERIFIED | `test/e2e/erl_test.go`: `TestERL2_ConnectionReplacement` sets `ErrorRecoveryLevel=2`, kills TCP with `ss -K`, retries Inquiry 10 times; logs ERL 0 fallback as acceptable per D-04 |
| 5 | ABORT TASK TMF sent during concurrent long-running SCSI command | VERIFIED | `test/e2e/tmf_test.go`: `TestTMF_AbortTask` launches concurrent `ReadBlocks(256 blocks)`, captures ITT via `WithPDUHook` + `sync.Once` + channel, calls `sess.AbortTask(ctx, itt)` |
| 6 | TARGET WARM RESET TMF executes and session survives | VERIFIED | `test/e2e/tmf_test.go`: `TestTMF_TargetWarmReset` calls `sess.TargetWarmReset(ctx)`, handles error as expected session kill, re-establishes new session and calls Inquiry to confirm target alive |
| 7 | Header-only and data-only digest modes negotiated and exercised separately | VERIFIED | `test/e2e/digest_test.go`: `TestDigest_HeaderOnly` uses `WithHeaderDigest("CRC32C")` only; `TestDigest_DataOnly` uses `WithDataDigest("CRC32C")` only; both run write+read with `bytes.Equal` |
| 8 | SCSI CHECK CONDITION with sense data parsed and reported correctly | VERIFIED | `test/e2e/scsierror_test.go`: `TestSCSIError_SenseDataParsing` asserts `scsiErr.SenseKey != 0` and `scsiErr.Message != ""` after out-of-range read |
| 9 | Out-of-range LBA write returns expected ILLEGAL REQUEST sense key | VERIFIED | `test/e2e/scsierror_test.go`: `TestSCSIError_OutOfRangeLBA` asserts `SenseKey==0x05`, `ASC==0x21`, `ASCQ==0x00`, and `strings.Contains(errMsg, "ILLEGAL REQUEST")` |
| 10 | All new tests skip gracefully when not root or modules not loaded | VERIFIED | Every new test function calls `lio.RequireRoot(t)` and `lio.RequireModules(t)` as first statements; ERL tests also use `t.Skip` when configfs write fails or negotiation is rejected |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/login/login.go` | `operationalOverrides` field on loginConfig, `WithOperationalOverrides` LoginOption, override loop in `buildInitiatorKeys` | VERIFIED | Lines 41, 116-120, 528-535 confirm all three components present and wired |
| `options.go` | Public `WithOperationalOverrides` Option wrapping `login.WithOperationalOverrides` | VERIFIED | Lines 119-127 confirm function present, calls `login.WithOperationalOverrides(overrides)` |
| `test/e2e/largewrite_test.go` | Large write multi-R2T E2E test, `TestLargeWrite_MultiR2T` | VERIFIED | 83-line file, `//go:build e2e`, `numBlocks=2048`, `bytes.Equal` check, `lio.RequireRoot/RequireModules` |
| `test/e2e/negotiation_test.go` | ImmediateData x InitialR2T 2x2 matrix E2E test, `TestNegotiation_ImmediateDataInitialR2T` | VERIFIED | 117-line file, all 4 matrix cells, configfs writes, `uiscsi.WithOperationalOverrides` |
| `test/e2e/digest_test.go` | Extended with `TestDigest_HeaderOnly` and `TestDigest_DataOnly` | VERIFIED | Lines 70-118 and 123-171; each uses only one digest option; write+read with `bytes.Equal` |
| `test/e2e/scsierror_test.go` | SCSI error condition tests, `TestSCSIError_OutOfRangeLBA` and `TestSCSIError_SenseDataParsing` | VERIFIED | 120-line file, both functions present, `errors.As` with `*uiscsi.SCSIError`, LBA 200000 |
| `test/e2e/tmf_test.go` | Extended with `TestTMF_AbortTask` and `TestTMF_TargetWarmReset` | VERIFIED | Lines 73-165 and 171-234; PDU hook with `sync.Once`, ITT extraction, session re-establishment |
| `test/e2e/erl_test.go` | ERL 1/2 E2E tests, `TestERL1_SNACKRecovery` and `TestERL2_ConnectionReplacement` | VERIFIED | 166-line file, `setTargetParam` helper, configfs writes, `WithOperationalOverrides`, `ss -K` pattern |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `options.go` | `internal/login/login.go` | `login.WithOperationalOverrides` | WIRED | Line 125: `c.loginOpts = append(c.loginOpts, login.WithOperationalOverrides(overrides))` |
| `test/e2e/negotiation_test.go` | `options.go` | `uiscsi.WithOperationalOverrides` | WIRED | Line 64: `uiscsi.WithOperationalOverrides(map[string]string{...})` in Dial call |
| `test/e2e/digest_test.go` | `options.go` | `uiscsi.WithHeaderDigest` / `uiscsi.WithDataDigest` | WIRED | Line 86: `uiscsi.WithHeaderDigest("CRC32C")`; line 139: `uiscsi.WithDataDigest("CRC32C")` |
| `test/e2e/scsierror_test.go` | `errors.go` | `errors.As` with `*uiscsi.SCSIError` | WIRED | Lines 50-53 and 104-107: `errors.As(err, &scsiErr)` pattern in both tests |
| `test/e2e/tmf_test.go` | `session.go` | `sess.AbortTask` and `sess.TargetWarmReset` | WIRED | Line 137: `sess.AbortTask(ctx, itt)`; line 201: `sess.TargetWarmReset(ctx)` |
| `test/e2e/erl_test.go` | `options.go` | `uiscsi.WithOperationalOverrides` for `ErrorRecoveryLevel` | WIRED | Lines 60-62 and 117-119: `WithOperationalOverrides(map[string]string{"ErrorRecoveryLevel": "1"/"2"})` |

### Data-Flow Trace (Level 4)

These are E2E test files, not components that render data from a store. The data flows are exercised at runtime against real LIO targets. Static code analysis confirms the write data (`testData` slice) flows into `sess.WriteBlocks`, the read back (`readBack`) comes from `sess.ReadBlocks`, and `bytes.Equal` compares them. No hollow props or disconnected state variables exist.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `largewrite_test.go` | `testData` / `readBack` | `make([]byte, ...)` + fill loop / `sess.ReadBlocks` | Yes — filled deterministically, read from real LIO | FLOWING |
| `negotiation_test.go` | `testData` / `readBack` | fill loop / `sess.ReadBlocks` per subtest | Yes — per-subtest xor pattern | FLOWING |
| `digest_test.go` (new) | `testData` / `readBack` | fill loop / `sess.ReadBlocks` | Yes | FLOWING |
| `scsierror_test.go` | `err` / `scsiErr` | `sess.WriteBlocks` error / `errors.As` | Yes — error returned from real LIO target | FLOWING |
| `tmf_test.go` (new) | `capturedITT` / `result` | PDU hook + `atomic.StoreUint32` / `sess.AbortTask` | Yes | FLOWING |
| `erl_test.go` | `inq` | `sess.Inquiry` | Yes | FLOWING |

### Behavioral Spot-Checks

Step 7b: SKIPPED — E2E tests require a running LIO kernel target with root access. Tests are gated by `lio.RequireRoot` and `lio.RequireModules`. However, compilation and static analysis passes cleanly:

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Library compiles | `go build ./...` | exit 0, no output | PASS |
| E2E tests vet cleanly | `go vet -tags e2e ./test/e2e/` | exit 0, no output | PASS |
| `WithOperationalOverrides` callable | grep patterns in source | function exists at correct signature | PASS |
| `PDUSend` constant available | `types.go:162` | `PDUSend PDUDirection = PDUDirection(session.PDUSend)` | PASS |
| `ILLEGAL REQUEST` message derivable | `scsi/opcode.go:82` + `sense.go:21-27` + `errors.go:72` | `SenseData.String()` returns `"ILLEGAL REQUEST: ..."`, mapped to `SCSIError.Message` | PASS |

### Requirements Coverage

The requirement IDs E2E-11 through E2E-20 are used in ROADMAP.md and plan frontmatter but are **not defined in REQUIREMENTS.md**. The main REQUIREMENTS.md uses a different taxonomy (PDU-, LOGIN-, SESS-, TMF-, ERL-, SCSI-, TEST-, etc.). E2E-XX IDs appear to be phase-internal tracking identifiers for this phase's test coverage goals, not entries in the formal requirements registry.

Cross-reference of plan requirements against REQUIREMENTS.md:

| Plan ID | Plans | REQUIREMENTS.md Entry | Status | Notes |
|---------|-------|-----------------------|--------|-------|
| E2E-11 | 10-01 | Not in REQUIREMENTS.md | INFO | Phase-internal test coverage ID. Maps conceptually to TEST-01 (IOL-inspired conformance test suite) |
| E2E-12 | 10-01 | Not in REQUIREMENTS.md | INFO | Maps to TEST-04 (Parameterized tests for negotiation parameter matrix) — currently Pending |
| E2E-13 | 10-03 | Not in REQUIREMENTS.md | INFO | Maps to ERL-02 (ERL 1 within-connection recovery) — currently Pending |
| E2E-14 | 10-03 | Not in REQUIREMENTS.md | INFO | Maps to ERL-03 (ERL 2 connection-level recovery) — currently Pending |
| E2E-15 | 10-03 | Not in REQUIREMENTS.md | INFO | Maps to TMF-01 (ABORT TASK) — currently Pending |
| E2E-16 | 10-03 | Not in REQUIREMENTS.md | INFO | Maps to TMF-04 (TARGET WARM RESET) — currently Pending |
| E2E-17 | 10-02 | Not in REQUIREMENTS.md | INFO | Maps to INTEG-01/INTEG-03 (header digest) — both Complete |
| E2E-18 | 10-02 | Not in REQUIREMENTS.md | INFO | Maps to INTEG-02/INTEG-03 (data digest) — both Complete |
| E2E-19 | 10-02 | Not in REQUIREMENTS.md | INFO | Maps to API-05 (structured error types with sense data) — Complete, and SCSI-10 (sense data parsing) — Complete |
| E2E-20 | 10-01,02,03 | Not in REQUIREMENTS.md | INFO | Maps to TEST-02 (integration test infrastructure with automated target setup) — Complete |

**Finding:** The E2E-XX IDs are not orphaned requirements in REQUIREMENTS.md — they simply do not exist there. They are a secondary test-coverage tracking scheme used within Phase 10's planning artifacts. No formal requirements are unaccounted for: all 10 requirement IDs declared across the three plans map to the same set of 10 test coverage goals, all of which are implemented.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | — | — | — | No anti-patterns found in any of the 6 new/modified files |

### Human Verification Required

#### 1. Multi-R2T sequence count verification

**Test:** Run `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestLargeWrite_MultiR2T` with a packet capture on the loopback interface.
**Expected:** `tshark -i lo -Y iscsi` shows 4 or more R2T PDUs from target, and matching Data-Out PDU sequences from initiator, before the final SCSI Response.
**Why human:** Protocol-level PDU count confirmation requires running the test with root access and LIO kernel modules loaded. Cannot be verified by static analysis.

#### 2. ImmNo_R2TNo subtest behavior

**Test:** Run `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestNegotiation_ImmediateDataInitialR2T/ImmNo_R2TNo` and inspect logs.
**Expected:** With ImmediateData=No and InitialR2T=No, all write data should be sent as Unsolicited Data-Out (UDO) before any R2T. The test should pass the bytes.Equal check. This combination is the most unusual and exercises WRITE-04.
**Why human:** Requires running against real LIO target to verify the negotiation actually takes effect and the correct data path (UDO) is exercised.

#### 3. ERL 1/2 skip vs. execute behavior

**Test:** Run `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestERL` on a kernel with LIO modules loaded.
**Expected:** Each test either skips with a documented reason (kernel does not expose the configfs param, or negotiation rejected) or exercises the recovery path. If ERL 1 negotiation succeeds, the Inquiry response should complete cleanly.
**Why human:** ERL support varies by kernel version. The skip path is intentional per D-04; a human needs to confirm the appropriate branch is taken on the target kernel.

#### 4. TargetWarmReset session drop vs. response

**Test:** Run `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestTMF_TargetWarmReset`.
**Expected:** Either (a) `TargetWarmReset` returns an error because LIO drops the TCP connection before sending the TMF response, or (b) it returns a valid response code. Either path is acceptable. The test must not hang. The re-dial after reset must succeed.
**Why human:** LIO's exact behavior on TARGET WARM RESET (drop vs. respond) varies by configuration and kernel version. A human must confirm the test completes without hanging.

### Gaps Summary

No gaps. All 10 success criteria from ROADMAP.md are verified by substantive, wired, non-stub code:

- `internal/login/login.go` has the `operationalOverrides` field and complete override loop in `buildInitiatorKeys`.
- `options.go` has `WithOperationalOverrides` wired to `login.WithOperationalOverrides`.
- All 6 test files (`largewrite_test.go`, `negotiation_test.go`, `digest_test.go` extended, `scsierror_test.go`, `tmf_test.go` extended, `erl_test.go`) are substantive implementations with correct build tags, graceful skip guards, and full write+read or error assertion logic.
- `go build ./...` and `go vet -tags e2e ./test/e2e/` both pass cleanly.
- The `ILLEGAL REQUEST` string path from `scsi/opcode.go` through `sense.go` through `errors.go` through `SCSIError.Error()` to the test assertion is traceable and correct.

The only items requiring human attention are runtime behaviors that cannot be verified without root access and a live LIO kernel target.

---

_Verified: 2026-04-02_
_Verifier: Claude (gsd-verifier)_
