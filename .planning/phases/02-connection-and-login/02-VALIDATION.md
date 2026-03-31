---
phase: 2
slug: connection-and-login
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-31
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (go1.25.5) |
| **Config file** | None needed — `go test` with default settings |
| **Quick run command** | `go test ./internal/login/ -v -count=1` |
| **Full suite command** | `go test ./... -race -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/login/ -race -count=1`
- **After every plan wave:** Run `go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | LOGIN-02 | unit | `go test ./internal/login/ -run TestTextCodec -v` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | LOGIN-06 | unit | `go test ./internal/login/ -run TestNegotiation -v` | ❌ W0 | ⬜ pending |
| 02-01-03 | 01 | 1 | TEST-04 | unit | `go test ./internal/login/ -run TestNegotiationMatrix -v` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | LOGIN-04 | unit | `go test ./internal/login/ -run TestCHAP -v` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | LOGIN-05 | unit | `go test ./internal/login/ -run TestMutualCHAP -v` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 2 | LOGIN-01 | unit | `go test ./internal/login/ -run TestLoginStateMachine -v` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 2 | LOGIN-03 | unit | `go test ./internal/login/ -run TestLoginAuthNone -v` | ❌ W0 | ⬜ pending |
| 02-03-03 | 03 | 2 | LOGIN-04 | unit | `go test ./internal/login/ -run TestLoginCHAP -v` | ❌ W0 | ⬜ pending |
| 02-03-04 | 03 | 2 | LOGIN-05 | unit | `go test ./internal/login/ -run TestLoginMutualCHAP -v` | ❌ W0 | ⬜ pending |
| 02-04-01 | 04 | 2 | INTEG-01 | unit | `go test ./internal/login/ -run TestHeaderDigest -v` | ❌ W0 | ⬜ pending |
| 02-04-02 | 04 | 2 | INTEG-02 | unit | `go test ./internal/login/ -run TestDataDigest -v` | ❌ W0 | ⬜ pending |
| 02-04-03 | 04 | 2 | INTEG-03 | unit | `go test ./internal/login/ -run TestDigestGeneration -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/login/` directory — does not exist yet, create package
- [ ] `internal/login/textcodec_test.go` — text key-value codec tests
- [ ] `internal/login/negotiation_test.go` — parameterized negotiation tests (TEST-04)
- [ ] `internal/login/chap_test.go` — CHAP unit tests with known test vectors
- [ ] `internal/login/login_test.go` — integration tests with mock target

*Wave 0 creates all test files as stubs, filled during execution.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | — | — | — |

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
