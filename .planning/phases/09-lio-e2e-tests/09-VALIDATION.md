---
phase: 09
slug: lio-e2e-tests
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-02
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) with `//go:build e2e` tag |
| **Config file** | none -- `go test -tags e2e` activates E2E tests |
| **Quick run command** | `go vet -tags e2e ./test/lio/ ./test/e2e/` |
| **Full suite command** | `sudo go test -tags e2e -v -count=1 ./test/e2e/` |
| **Non-E2E regression** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds (E2E suite), ~10 seconds (vet + non-E2E) |
| **Root required** | Yes -- LIO configfs operations require CAP_SYS_ADMIN |

---

## Sampling Rate

- **After every task commit:** `go vet -tags e2e ./test/lio/ ./test/e2e/` + `go test ./... -count=1`
- **After every plan wave:** `sudo go test -tags e2e -v -count=1 ./test/e2e/`
- **Before `/gsd:verify-work`:** Full E2E suite green + non-E2E regression green
- **Max feedback latency:** 30 seconds (E2E), 10 seconds (vet-only)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | E2E-01, E2E-02 | build | `go vet -tags e2e ./test/lio/` | no W0 | pending |
| 09-01-02 | 01 | 1 | E2E-03, E2E-10 | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestBasicConnectivity` | no W0 | pending |
| 09-02-01 | 02 | 2 | E2E-04, E2E-05, E2E-06 | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run "TestDataIntegrity\|TestCHAP\|TestDigests"` | no W0 | pending |
| 09-02-02 | 02 | 2 | E2E-07, E2E-08, E2E-09 | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run "TestMultiLUN\|TestTMF\|TestErrorRecovery"` | no W0 | pending |

*Status: pending / green / red / flaky*

---

## Requirement -> Test Mapping

| Req ID | Behavior | Test Function | Plan |
|--------|----------|---------------|------|
| E2E-01 | LIO configfs target setup helper | `lio.Setup` (exercised by all tests) | 01 |
| E2E-02 | LIO configfs teardown + orphan sweep | `lio.SweepOrphans`, cleanup func | 01 |
| E2E-03 | Basic connectivity (Discover+Dial+Inquiry+ReadCapacity+Close) | `TestBasicConnectivity` | 01 |
| E2E-04 | Data integrity (write+read+verify) | `TestDataIntegrity` | 02 |
| E2E-05 | CHAP authentication (one-way + mutual) | `TestCHAP`, `TestCHAPMutual`, `TestCHAPBadPassword` | 02 |
| E2E-06 | CRC32C header + data digest negotiation | `TestDigests` | 02 |
| E2E-07 | Multi-LUN enumeration via ReportLuns | `TestMultiLUN` | 02 |
| E2E-08 | Task management (LUNReset) | `TestTMF_LUNReset` | 02 |
| E2E-09 | Error recovery (connection drop, ERL 0 reconnect) | `TestErrorRecovery_ConnectionDrop` | 02 |
| E2E-10 | Delete dead gotgt stubs | `test ! -f test/integration/gotgt_test.go` | 01 |

---

## Wave 0 Requirements

All test files are new -- nothing exists yet.

- [ ] `test/lio/lio.go` -- LIO configfs setup/teardown helper
- [ ] `test/lio/sweep.go` -- Orphan target sweep
- [ ] `test/e2e/e2e_test.go` -- TestMain + TestBasicConnectivity
- [ ] `test/e2e/data_test.go` -- Data integrity test
- [ ] `test/e2e/chap_test.go` -- CHAP authentication tests
- [ ] `test/e2e/digest_test.go` -- CRC32C digest test
- [ ] `test/e2e/multilun_test.go` -- Multi-LUN test
- [ ] `test/e2e/tmf_test.go` -- Task management test
- [ ] `test/e2e/recovery_test.go` -- Error recovery test

---

## Two-Tier Verification Strategy

E2E tests require root and kernel modules. Verification uses two tiers:

**Tier 1 -- Always available (no root):**
- `go vet -tags e2e ./test/lio/ ./test/e2e/` -- compilation and static analysis
- `go test ./... -count=1` -- non-E2E regression (ensures build tag isolation)
- File existence checks (grep for expected functions)

**Tier 2 -- Root required:**
- `sudo go test -tags e2e -v -count=1 ./test/e2e/` -- full E2E execution
- Tests self-skip with clear message if root/modules unavailable

Tier 1 runs after every task. Tier 2 runs after each wave and at phase gate.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| E2E tests skip gracefully without root | E2E-01 | Requires running as non-root | Run `go test -tags e2e -v ./test/e2e/` as non-root, verify skip messages |
| gotgt stubs deleted | E2E-10 | File deletion check | `test ! -f test/integration/gotgt_test.go` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify commands
- [x] Sampling continuity: every task has Tier 1 automated verify
- [x] Wave 0 covers all MISSING references (all files are new)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
