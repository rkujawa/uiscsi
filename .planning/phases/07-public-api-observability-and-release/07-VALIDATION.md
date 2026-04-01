---
phase: 07
slug: public-api-observability-and-release
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-01
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — Go test infrastructure already established |
| **Quick run command** | `go test ./... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | API-01, API-05 | unit | `go test ./... -run TestExecute -count=1` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | API-02, API-03, API-04 | unit | `go test ./... -run "TestReadBlocks\|TestWriteBlocks\|TestInquiry" -count=1` | ❌ W0 | ⬜ pending |
| 07-02-01 | 02 | 1 | TEST-02 | integration | `go test ./test/... -count=1` | ❌ W0 | ⬜ pending |
| 07-02-02 | 02 | 1 | TEST-01 | conformance | `go test ./test/conformance/... -count=1` | ❌ W0 | ⬜ pending |
| 07-03-01 | 03 | 2 | DOC-01 | build | `go doc ./...` | ✅ | ⬜ pending |
| 07-03-02 | 03 | 2 | DOC-02, DOC-03, DOC-04, DOC-05 | build | `go build ./examples/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `uiscsi.go` — public API package root
- [ ] `test/target.go` — mock target for conformance tests
- [ ] `test/helpers.go` — test dial/setup helpers

*Existing go test infrastructure covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Godoc renders correctly | DOC-01 | Godoc rendering is visual | Run `godoc -http=:6060` and check package docs |
| Examples are runnable with real target | DOC-02-05 | Requires real iSCSI target | Run `go run ./examples/discover -target <addr>` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
