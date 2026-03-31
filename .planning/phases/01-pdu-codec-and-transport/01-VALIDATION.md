---
phase: 1
slug: pdu-codec-and-transport
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-31
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test -race -count=1 ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test -race -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | PDU-01 | unit | `go test -run TestBHS ./pdu/` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | PDU-02 | unit | `go test -run TestOpcode ./pdu/` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | PDU-03 | unit | `go test -run TestAHS ./pdu/` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 1 | PDU-04 | unit | `go test -run TestDigest ./digest/` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 2 | XPORT-01 | unit | `go test -run TestConn ./transport/` | ❌ W0 | ⬜ pending |
| 01-03-02 | 03 | 2 | XPORT-02 | unit | `go test -run TestFrame ./transport/` | ❌ W0 | ⬜ pending |
| 01-03-03 | 03 | 2 | XPORT-03 | unit | `go test -run TestSerial ./serial/` | ❌ W0 | ⬜ pending |
| 01-03-04 | 03 | 2 | XPORT-04 | race | `go test -race -run TestPump ./transport/` | ❌ W0 | ⬜ pending |
| 01-04-01 | 04 | 2 | TEST-03 | integration | `go test -race -count=1 ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` — initialize module with Go 1.25
- [ ] Package directory structure (`pdu/`, `digest/`, `transport/`, `serial/`)
- [ ] Test file stubs for each package

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
