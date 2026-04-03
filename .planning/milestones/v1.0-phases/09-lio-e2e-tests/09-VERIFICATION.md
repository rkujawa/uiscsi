---
phase: 09-lio-e2e-tests
verified: 2026-04-02T17:30:00Z
status: human_needed
score: 9/9 automated must-haves verified
re_verification: false
human_verification:
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestBasicConnectivity"
    expected: "Test passes: Discover returns target, Dial succeeds, Inquiry returns LIO-ORG VendorID, ReadCapacity returns non-zero BlockSize consistent with 64MB LUN, TestUnitReady succeeds, Close succeeds"
    why_human: "Requires root, loaded kernel modules (target_core_mod, iscsi_target_mod, target_core_file), and a kernel that supports configfs/LIO. Cannot verify in CI or sandboxed shell."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDataIntegrity"
    expected: "Writes 8-block pattern at LBA 0 and LBA 100, reads back, bytes.Equal comparison passes both times"
    why_human: "Requires LIO kernel target. Tests the uiscsi write+read path end-to-end with real kernel SCSI handling."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestCHAP"
    expected: "Dial with WithCHAP(e2e-user, e2e-secret-pass) succeeds; TestCHAPBadPassword dial fails with auth error"
    why_human: "Requires LIO CHAP configfs path to be writable and readable by target kernel module."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestCHAPMutual"
    expected: "Dial with WithMutualCHAP succeeds; bidirectional auth completes without error"
    why_human: "Requires LIO mutual CHAP (authenticate_target=1) configfs support."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDigests"
    expected: "CRC32C header+data digest negotiation completes; write+read cycle returns identical bytes"
    why_human: "Requires target to negotiate CRC32C; LIO does this by default but needs real kernel."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestMultiLUN"
    expected: "ReportLuns returns at least 3 entries (LUNs 0, 1, 2); ReadCapacity for each LUN returns correct byte count for 32/64/128MB sizes"
    why_human: "Requires LIO multi-LUN configfs setup and real SCSI Report Luns response."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestTMF_LUNReset"
    expected: "LUNReset returns response=0 (Function Complete); Inquiry still succeeds afterward"
    why_human: "Requires LIO TMF support in kernel. TMF-03 (LUN RESET) is marked Pending in REQUIREMENTS.md — this tests whether the uiscsi implementation actually works against a real target."
  - test: "Run: sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestErrorRecovery_ConnectionDrop"
    expected: "ss -K kills TCP socket; retry loop succeeds within 10 attempts (approximately 5-10 seconds); Inquiry returns valid data after reconnect"
    why_human: "Requires ss tool available on host, root, and ERL-01 reconnect implementation to be functional end-to-end."
notes:
  - "E2E-01 through E2E-10 are referenced in ROADMAP.md Phase 9 and PLAN frontmatter but do not exist in REQUIREMENTS.md. These IDs have no corresponding requirement definitions. See Requirements Coverage section."
---

# Phase 9: LIO-based E2E Test Suite — Verification Report

**Phase Goal:** Build `test/lio/` helper package for configfs-based LIO target setup/teardown, implement E2E tests covering CHAP, digests, data integrity, multi-LUN, and error recovery against a real kernel iSCSI target. Drop gotgt stubs. Local execution only (CI deferred).
**Verified:** 2026-04-02T17:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `test/lio/` helper creates and tears down real LIO iSCSI targets via configfs with correct ordering | VERIFIED | `test/lio/lio.go` (391 lines): Setup() creates backstore, IQN, TPG, NP, LUNs, ACLs in order; teardownState() reverses exactly — disable TPG, remove ACL mapped LUNs, remove ACL, remove LUN symlinks, remove LUN dirs, remove NP, remove TPG, remove IQN, remove backstore, remove /dev/shm files |
| 2 | All 7 E2E scenarios pass against a real kernel iSCSI target when run as root | HUMAN NEEDED | All 7 scenarios have substantive, non-stub implementations. Runtime correctness requires root + LIO kernel modules. |
| 3 | Tests skip gracefully with clear messages when not root or kernel modules not loaded | VERIFIED | RequireRoot checks `os.Getuid() != 0`, calls `t.Skip("e2e tests require root (configfs writes need CAP_SYS_ADMIN)")`. RequireModules reads `/proc/modules`, skips with `t.Skipf("kernel module %s not loaded", mod)` for each missing module. Both called at top of every test function. |
| 4 | Dead gotgt integration stubs are removed | VERIFIED | `test/integration/gotgt_test.go` does not exist. Commit 5ddac9d confirms deletion of 105 lines. |

**Score:** 3/4 truths verified automatically; 1 requires human execution.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `test/lio/lio.go` | LIO configfs target setup/teardown helper | VERIFIED | 391 lines, `//go:build e2e`, exports Setup, RequireRoot, RequireModules, RequireConfigfs, Config, Target structs |
| `test/lio/sweep.go` | Orphan target sweep for TestMain | VERIFIED | 118 lines, `//go:build e2e`, exports SweepOrphans(), teardownTarget, removeLUNDirs, cleanOrphanBackstores |
| `test/e2e/e2e_test.go` | TestMain + TestBasicConnectivity | VERIFIED | 109 lines, `//go:build e2e`, TestMain calls lio.SweepOrphans, TestBasicConnectivity exercises full Discover/Dial/Inquiry/ReadCapacity/TestUnitReady/Close |
| `test/e2e/data_test.go` | Data integrity E2E test | VERIFIED | 119 lines, TestDataIntegrity with write+read at LBA 0 and LBA 100, bytes.Equal comparison |
| `test/e2e/chap_test.go` | CHAP authentication E2E tests | VERIFIED | 113 lines, TestCHAP (one-way), TestCHAPMutual, TestCHAPBadPassword |
| `test/e2e/digest_test.go` | CRC32C digest E2E test | VERIFIED | 65 lines, TestDigests negotiates CRC32C header+data, write+read cycle |
| `test/e2e/multilun_test.go` | Multi-LUN enumeration E2E test | VERIFIED | 98 lines, TestMultiLUN with 3 LUNs, ReportLuns + ReadCapacity for each |
| `test/e2e/tmf_test.go` | Task management E2E test | VERIFIED | 70 lines, TestTMF_LUNReset sends LUN Reset, checks response=0, verifies session survives |
| `test/e2e/recovery_test.go` | Error recovery E2E test | VERIFIED | 84 lines, TestErrorRecovery_ConnectionDrop uses ss -K, 10-attempt retry loop |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `test/e2e/e2e_test.go` | `test/lio/lio.go` | `lio.Setup(t, config)` call | WIRED | Line 34: `lio.Setup(t, lio.Config{...})` — import present, used in TestBasicConnectivity |
| `test/e2e/e2e_test.go` | uiscsi public API | `uiscsi.Dial, sess.Inquiry, sess.ReadCapacity` | WIRED | Lines 44, 64, 74, 84: Discover, Dial, Inquiry, ReadCapacity, TestUnitReady all called with results checked |
| `test/e2e/data_test.go` | uiscsi public API | `sess.WriteBlocks + sess.ReadBlocks` | WIRED | Lines 63, 68: WriteBlocks then ReadBlocks, bytes.Equal comparison — response consumed |
| `test/e2e/chap_test.go` | uiscsi options | `uiscsi.WithCHAP, uiscsi.WithMutualCHAP` | WIRED | Lines 34, 71: WithCHAP and WithMutualCHAP passed to Dial, session result used |
| `test/e2e/recovery_test.go` | uiscsi reconnect | `uiscsi.WithMaxReconnectAttempts + post-drop Inquiry` | WIRED | Line 37: WithMaxReconnectAttempts(3) in Dial; lines 73-82: retry loop calls Inquiry after ss -K kill |

### Data-Flow Trace (Level 4)

These are test files — they do not render data, they generate it and assert on it. The data flow is the test logic itself. No Level 4 trace needed (no components rendering dynamic state from a store or API).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go vet -tags e2e ./test/lio/` compiles clean | `go vet -tags e2e ./test/lio/` | no output (exit 0) | PASS |
| `go vet -tags e2e ./test/e2e/` compiles clean | `go vet -tags e2e ./test/e2e/` | no output (exit 0) | PASS |
| Existing test suite unaffected | `go test ./...` | all packages ok | PASS |
| gotgt_test.go deleted | `test -f test/integration/gotgt_test.go` | FILE DELETED | PASS |
| All 4 task commits exist | `git show 3c284f7 5ddac9d c8044ab 6676f92 --stat` | all present, correct files | PASS |
| TestBasicConnectivity has real LIO calls | grep pattern | Discover + Dial + Inquiry + ReadCapacity + TestUnitReady + Close — all present | PASS |
| All 7 test function names present | grep across test files | TestBasicConnectivity, TestDataIntegrity, TestCHAP, TestCHAPMutual, TestCHAPBadPassword, TestDigests, TestMultiLUN, TestTMF_LUNReset, TestErrorRecovery_ConnectionDrop | PASS (9 test functions across 7 files) |

### Requirements Coverage

**Critical finding: E2E-01 through E2E-10 do not exist in REQUIREMENTS.md.**

The ROADMAP.md Phase 9 entry and both PLAN frontmatter files declare requirements `E2E-01` through `E2E-10`. However, REQUIREMENTS.md contains no `E2E-` prefixed requirements — none were ever defined. The requirements document covers PDU-xx, XPORT-xx, LOGIN-xx, SESS-xx, READ-xx, WRITE-xx, INTEG-xx, TMF-xx, ERL-xx, EVT-xx, DISC-xx, SCSI-xx, API-xx, OBS-xx, TEST-xx, and DOC-xx categories, but not E2E-xx.

| Requirement ID | In REQUIREMENTS.md | Status |
|---------------|-------------------|--------|
| E2E-01 | NOT DEFINED | ORPHANED — ID exists in PLAN/ROADMAP, not in REQUIREMENTS.md |
| E2E-02 | NOT DEFINED | ORPHANED |
| E2E-03 | NOT DEFINED | ORPHANED |
| E2E-04 | NOT DEFINED | ORPHANED |
| E2E-05 | NOT DEFINED | ORPHANED |
| E2E-06 | NOT DEFINED | ORPHANED |
| E2E-07 | NOT DEFINED | ORPHANED |
| E2E-08 | NOT DEFINED | ORPHANED |
| E2E-09 | NOT DEFINED | ORPHANED |
| E2E-10 | NOT DEFINED | ORPHANED |

**Assessment:** The absence of E2E-xx definitions in REQUIREMENTS.md is a documentation gap, not an implementation gap. The phase's implementation delivers exactly what the ROADMAP success criteria describe. The E2E testing goal maps to existing requirements TEST-02 ("Integration test infrastructure with automated target setup, no manual SAN configuration") and supports the library-wide RFC 7143 compliance goal. The missing REQUIREMENTS.md entries do not block phase completion but represent incomplete traceability.

Closest existing requirements that the E2E tests exercise:

| E2E Scenario | Existing Requirement(s) Exercised |
|--------------|----------------------------------|
| Basic connectivity (TestBasicConnectivity) | TEST-02, LOGIN-03, DISC-01, DISC-02, SCSI-01, SCSI-02, SCSI-04 |
| Data integrity (TestDataIntegrity) | TEST-02, SCSI-05, SCSI-06 |
| CHAP one-way (TestCHAP) | LOGIN-04 |
| Mutual CHAP (TestCHAPMutual) | LOGIN-05 |
| CRC32C digests (TestDigests) | INTEG-01, INTEG-02, INTEG-03 |
| Multi-LUN (TestMultiLUN) | SCSI-08, DISC-02 |
| TMF LUNReset (TestTMF_LUNReset) | TMF-03 (marked Pending in REQUIREMENTS.md — this E2E test would verify it) |
| Error recovery (TestErrorRecovery) | ERL-01 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | — | — | — |

No TODO/FIXME comments, no placeholder returns, no hardcoded empty data arrays, no stub handlers found across any of the 9 E2E files.

### Human Verification Required

Eight test scenarios require root + LIO kernel modules to validate runtime correctness. All have substantive implementations with real protocol calls — the only unknowable from static analysis is whether the kernel LIO target behaves exactly as the tests expect.

#### 1. Basic Connectivity

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestBasicConnectivity`
**Expected:** Discover returns the created target IQN; Dial succeeds; Inquiry returns `VendorID="LIO-ORG"`; ReadCapacity returns BlockSize consistent with 64MB LUN; TestUnitReady succeeds; Close returns nil.
**Why human:** Requires kernel LIO modules and root. Cannot run in sandboxed shell.

#### 2. Data Integrity

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDataIntegrity`
**Expected:** bytes.Equal passes for both LBA 0 and LBA 100 write-then-read cycles.
**Why human:** Requires functioning LIO write path and real SCSI READ/WRITE commands against kernel target.

#### 3. CHAP Authentication

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestCHAP`
**Expected:** One-way CHAP succeeds; bad password dial returns non-nil error.
**Why human:** Requires LIO to honor configfs CHAP credential paths (`acls/<iqn>/auth/userid`, etc.).

#### 4. Mutual CHAP

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestCHAPMutual`
**Expected:** Bidirectional CHAP completes without error; Inquiry after auth succeeds.
**Why human:** Requires LIO `authenticate_target=1` and proper mutual CHAP challenge/response handling.

#### 5. CRC32C Digests

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDigests`
**Expected:** Dial with WithHeaderDigest("CRC32C")+WithDataDigest("CRC32C") succeeds; write+read returns identical data.
**Why human:** Requires LIO to negotiate CRC32C and real digest verification on both directions.

#### 6. Multi-LUN

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestMultiLUN`
**Expected:** ReportLuns returns LUNs 0, 1, 2; ReadCapacity for each returns exact byte counts for 32/64/128MB.
**Why human:** Requires LIO multi-LUN configfs setup and correct SCSI REPORT LUNS response parsing.

#### 7. TMF LUN Reset

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestTMF_LUNReset`
**Expected:** LUNReset returns response code 0 (Function Complete per RFC 7143 Section 11.6.1); Inquiry succeeds afterward.
**Why human:** Requires kernel LIO TMF support. Also validates TMF-03 (LUN RESET), which is marked Pending in REQUIREMENTS.md — runtime test is the only way to confirm the implementation works.

#### 8. Error Recovery

**Test:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestErrorRecovery_ConnectionDrop`
**Expected:** `ss -K` kills TCP socket; within 10 retry attempts (max ~27.5 seconds), Inquiry succeeds with valid data.
**Why human:** Requires `ss` tool with `-K` flag support, root, and ERL-01 reconnect logic to actually trigger and complete re-login to the same LIO target.

### Gaps Summary

No implementation gaps were found. All artifacts exist with substantive content, all key links are wired, and no anti-patterns were detected. The single category of concern is requirements traceability: E2E-01 through E2E-10 are declared in ROADMAP.md and PLAN frontmatter but are absent from REQUIREMENTS.md. This does not indicate missing functionality — the code covers all 7 scenarios described in the phase goal — but means the requirements document lacks a formal definition for these IDs.

The phase cannot be declared fully verified until at least the basic connectivity test passes on a machine with LIO kernel support (human verification item 1). The remaining 7 human items provide confidence in the specific protocol behaviors but are not strictly blocking if item 1 passes cleanly.

---

_Verified: 2026-04-02T17:30:00Z_
_Verifier: Claude (gsd-verifier)_
