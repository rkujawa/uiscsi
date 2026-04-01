---
phase: 3
slug: session-read-path-and-discovery
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-01
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (go1.25.5) |
| **Config file** | None needed — `go test` with default settings |
| **Quick run command** | `go test ./internal/session/ -v -count=1` |
| **Full suite command** | `go test ./... -race -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/session/ -race -count=1`
- **After every plan wave:** Run `go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | SESS-01 | unit | `go test ./internal/session/ -run TestSession -v` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | SESS-02 | unit | `go test ./internal/session/ -run TestCmdSN -v` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | READ-01 | unit | `go test ./internal/session/ -run TestSCSIRead -v` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | READ-02 | unit | `go test ./internal/session/ -run TestDataIn -v` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 1 | READ-03 | unit | `go test ./internal/session/ -run TestDataSN -v` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 1 | EVT-01 | unit | `go test ./internal/session/ -run TestNOP -v` | ❌ W0 | ⬜ pending |
| 03-03-02 | 03 | 1 | EVT-02 | unit | `go test ./internal/session/ -run TestAsync -v` | ❌ W0 | ⬜ pending |
| 03-03-03 | 03 | 1 | EVT-03 | unit | `go test ./internal/session/ -run TestLogout -v` | ❌ W0 | ⬜ pending |
| 03-04-01 | 04 | 2 | SESS-03 | unit | `go test ./internal/session/ -run TestKeepalive -v` | ❌ W0 | ⬜ pending |
| 03-04-02 | 04 | 2 | SESS-04 | unit | `go test ./internal/session/ -run TestSessionClose -v` | ❌ W0 | ⬜ pending |
| 03-04-03 | 04 | 2 | SESS-05 | unit | `go test ./internal/session/ -run TestAutoLogout -v` | ❌ W0 | ⬜ pending |
| 03-05-01 | 05 | 2 | DISC-01 | unit | `go test ./internal/session/ -run TestSendTargets -v` | ❌ W0 | ⬜ pending |
| 03-05-02 | 05 | 2 | DISC-02 | unit | `go test ./internal/session/ -run TestDiscover -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/` directory — does not exist yet, create package
- [ ] `internal/session/session_test.go` — session creation and CmdSN tests
- [ ] `internal/session/command_test.go` — SCSI command dispatch and Data-In tests
- [ ] `internal/session/keepalive_test.go` — NOP and async event tests
- [ ] `internal/session/discovery_test.go` — SendTargets and Discover tests

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
