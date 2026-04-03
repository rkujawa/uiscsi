---
phase: 08-lsscsi-discovery-utility
verified: 2026-04-02T13:30:00Z
status: passed
score: 13/13 must-haves verified
re_verification: false
---

# Phase 8: lsscsi-discovery-utility Verification Report

**Phase Goal:** Build a standalone CLI tool (uiscsi-ls) that performs iSCSI target discovery on specified portals and presents LUN information in lsscsi-style columnar format or JSON, using the uiscsi library as its backend.
**Verified:** 2026-04-02T13:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `uiscsi-ls --portal <addr>` discovers targets, connects to each, probes all LUNs, and displays results in lsscsi-style columnar format | VERIFIED | `outputColumnar` uses `tabwriter`, called from `main.go`; `probeAll` -> `probePortal` -> `probeTarget` -> `probeLUN` pipeline confirmed in `probe.go` |
| 2 | `--json` flag produces machine-parseable nested JSON output | VERIFIED | `jsonOutput` flag wired to `outputJSON` in `main.go:62-63`; `outputJSON` uses `json.NewEncoder` with `SetIndent`; `TestOutputJSON` validates round-trip unmarshal |
| 3 | CHAP credentials resolve from flags with env var fallback | VERIFIED | `resolveCHAP` in `probe.go:40-50`; flags take precedence over `ISCSI_CHAP_USER`/`ISCSI_CHAP_SECRET`; `TestResolveCHAP` covers all four cases |
| 4 | Multiple portals can be specified via repeated `--portal` flags | VERIFIED | `stringSlice` implements `flag.Value` in `main.go:21-27`; `TestPortalFlagRepeated` confirms accumulation |
| 5 | Unreachable portals are skipped with errors to stderr; remaining portals still probed | VERIFIED | `probeAll` iterates all portals unconditionally in `probe.go:54-59`; `TestProbePortalError` confirms two-portal probe returns both results when both fail |

**Score:** 5/5 success criteria verified

---

### Required Artifacts

#### Plan 01 Artifacts (CLI-04, CLI-05)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `uiscsi-ls/go.mod` | Separate Go module with uiscsi dependency | VERIFIED | Contains `module github.com/rkujawa/uiscsi-ls`, `go 1.25`, replace directive to `../` |
| `uiscsi-ls/device_type.go` | SCSI peripheral device type name table | VERIFIED | 38 lines, exports `deviceTypeName`, 32-entry array with O(1) lookup |
| `uiscsi-ls/format.go` | Columnar and JSON output formatters | VERIFIED | 100 lines, exports `outputColumnar`, `outputJSON`, `PortalResult`, `TargetResult`, `LUNResult`, `formatCapacity` |
| `uiscsi-ls/format_test.go` | Unit tests for formatters | VERIFIED | 163 lines (above 80-line minimum), 5 test functions: TestDeviceTypeName, TestFormatCapacity, TestOutputColumnar, TestOutputJSON, TestOutputColumnarErrorPortal |

#### Plan 02 Artifacts (CLI-01, CLI-02, CLI-03, CLI-06)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `uiscsi-ls/main.go` | CLI entry point with flag parsing, signal handling, exit codes | VERIFIED | 80 lines, contains `func main()`, `stringSlice` flag type, signal.NotifyContext, exit codes 0/1/2 |
| `uiscsi-ls/probe.go` | Discovery and per-target LUN probing logic | VERIFIED | 141 lines, exports `probeAll`, `probePortal`, `normalizePortal`, `resolveCHAP`; also `probeTarget`, `probeLUN` |
| `uiscsi-ls/probe_test.go` | Unit tests for probe helpers including error path | VERIFIED | 138 lines (above 60-line minimum), TestNormalizePortal, TestResolveCHAP, TestProbePortalError |
| `uiscsi-ls/main_test.go` | Unit tests for CLI flag parsing | VERIFIED | 63 lines (above 20-line minimum), TestStringSlice, TestPortalFlagRepeated, TestPortalFlagMissing |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `uiscsi-ls/main.go` | `uiscsi-ls/probe.go` | `probeAll` call | WIRED | `main.go:59`: `results := probeAll(ctx, portals, opts)` |
| `uiscsi-ls/main.go` | `uiscsi-ls/format.go` | `outputColumnar` or `outputJSON` call | WIRED | `main.go:62-65`: conditional call to `outputJSON` or `outputColumnar` |
| `uiscsi-ls/probe.go` | `github.com/rkujawa/uiscsi` | `uiscsi.Discover`, `uiscsi.Dial` | WIRED | `probe.go:16-20`: `var discoverFunc = uiscsi.Discover; var dialFunc = uiscsi.Dial`; called at lines 69, 88 |
| `uiscsi-ls/format.go` | `uiscsi-ls/device_type.go` | `deviceTypeName` call in columnar output | WIRED | `probe.go:118`: `lr.DeviceTypeS = deviceTypeName(inq.DeviceType)` — note: link fires in probe.go, not format.go directly, but is structurally satisfied |

---

### Data-Flow Trace (Level 4)

The uiscsi-ls tool is a CLI that calls out to external iSCSI targets at runtime. The probe pipeline wires correctly to the library: `probePortal` calls `discoverFunc` (bound to `uiscsi.Discover`) and `dialFunc` (bound to `uiscsi.Dial`), populates `PortalResult` structs, and passes them to the formatters. No hardcoded or static data is returned as a substitute for real probe results. The package-level `discoverFunc`/`dialFunc` vars are stubs only in tests (via `t.Cleanup` restore), not in production code.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `probe.go` (probePortal) | `targets []uiscsi.Target` | `uiscsi.Discover` (real iSCSI SendTargets) | Yes — live network call, no static substitution | FLOWING |
| `probe.go` (probeTarget) | `luns []uint64` | `sess.ReportLuns` (real SCSI REPORT LUNS) | Yes — live SCSI command | FLOWING |
| `probe.go` (probeLUN) | `inq *uiscsi.InquiryData` | `sess.Inquiry` (real SCSI INQUIRY) | Yes — live SCSI command | FLOWING |
| `format.go` (outputColumnar) | `[]PortalResult` | Populated by probe pipeline | Yes — receives live probe data | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All tests pass with -race | `cd uiscsi-ls && go test -race -count=1 ./...` | `ok github.com/rkujawa/uiscsi-ls 1.017s` | PASS |
| Module compiles cleanly | `cd uiscsi-ls && go build ./...` | exit 0 | PASS |
| go vet reports no issues | `cd uiscsi-ls && go vet ./...` | exit 0, no output | PASS |
| Binary exits 1 on no portal flags | `/tmp/uiscsi-ls 2>&1; echo "exit: $?"` | stderr usage error, exit: 1 | PASS |
| deviceTypeName(0x00) returns "disk" | verified via TestDeviceTypeName/0x00 | PASS | PASS |
| deviceTypeName(0xFF) returns "unknown" | verified via TestDeviceTypeName/0xFF | PASS | PASS |

---

### Requirements Coverage

The requirement IDs CLI-01 through CLI-06 are phase-specific identifiers defined in ROADMAP.md Phase 8 and the VALIDATION.md. They do not appear in REQUIREMENTS.md (which tracks library requirements under different ID prefixes). This is expected: Phase 8 introduces a new CLI tool with its own requirement namespace.

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CLI-01 | 08-02-PLAN | Portal flag parsing, repeated --portal flags | SATISFIED | `stringSlice` flag.Value + TestPortalFlagRepeated passing |
| CLI-02 | 08-02-PLAN | Default port 3260 when port omitted | SATISFIED | `normalizePortal` in probe.go + TestNormalizePortal passing (5 cases including IPv6) |
| CLI-03 | 08-02-PLAN | CHAP credentials flag > env var precedence | SATISFIED | `resolveCHAP` in probe.go + TestResolveCHAP passing (all 4 cases) |
| CLI-04 | 08-01-PLAN | Columnar output format matches lsscsi style | SATISFIED | `outputColumnar` with tabwriter + TestOutputColumnar passing |
| CLI-05 | 08-01-PLAN | JSON output valid and complete | SATISFIED | `outputJSON` with nested portals/targets/luns + TestOutputJSON round-trip passing |
| CLI-06 | 08-02-PLAN | Unreachable portal skip-and-continue | SATISFIED | `probeAll` unconditional iteration + TestProbePortalError confirms both portals probed |

No orphaned requirements: REQUIREMENTS.md traceability table does not reference Phase 8, which is correct since CLI-01/06 are phase-local identifiers not in the main requirements registry.

---

### Anti-Patterns Found

Scanned all 7 Go source files for stubs, placeholders, hardcoded empty returns, and unimplemented handlers.

| File | Pattern | Severity | Assessment |
|------|---------|----------|------------|
| All files | No TODO/FIXME/HACK/PLACEHOLDER found | - | Clean |
| `main.go` | `os.Exit(2)` at line 79 when no LUNs found | Info | Intentional design: exit 2 means all portals failed or no LUNs, per plan spec |
| `probe.go` | `lr.CapacityStr = "-"` for non-disk types | Info | Intentional design per Pitfall 3 gating — not a stub |
| `probe.go` | `discoverFunc`/`dialFunc` package-level vars | Info | Test stubbing pattern with real implementation as default value — not a production stub |

No blockers. No warnings. All patterns are intentional and correctly implemented.

---

### Human Verification Required

One item cannot be verified programmatically:

**1. Full probe pipeline against a live iSCSI target**

**Test:** Run `./uiscsi-ls --portal <gotgt-address>` against a locally running gotgt instance.
**Expected:** Output shows one line per LUN with target IQN, portal address, LUN number, device type (e.g., "disk"), vendor/product strings, and capacity in GB/TB. With `--json`, output is valid nested JSON with `portals` -> `targets` -> `luns` hierarchy.
**Why human:** The probe pipeline (Discover -> Dial -> ReportLuns -> Inquiry -> ReadCapacity) requires a live iSCSI target. Unit tests verify each component in isolation with stubs; end-to-end flow can only be confirmed against a real target.

---

## Summary

Phase 8 goal is fully achieved. All 8 source files exist and are substantive (no stubs, no placeholders). All 12 test functions pass with `-race`. The module compiles cleanly. The binary correctly exits 1 with a usage error when run without flags. Key links are all wired: `main.go` calls `probeAll` which is bound to `uiscsi.Discover`/`uiscsi.Dial`, and the output path calls `outputColumnar`/`outputJSON` from `format.go`. The `deviceTypeName` lookup from `device_type.go` is called in `probe.go` when populating `LUNResult.DeviceTypeS`.

All 6 phase-specific CLI requirements are satisfied by the implementation and confirmed by passing tests. The only item not verifiable programmatically is the end-to-end probe against a live iSCSI target, which requires manual testing with gotgt.

---

_Verified: 2026-04-02T13:30:00Z_
_Verifier: Claude (gsd-verifier)_
