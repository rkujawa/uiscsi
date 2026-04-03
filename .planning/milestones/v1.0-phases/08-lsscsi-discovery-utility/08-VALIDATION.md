# Phase 08: lsscsi-discovery-utility - Validation Strategy

**Created:** 2026-04-02
**Source:** Extracted from 08-RESEARCH.md Validation Architecture section

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.25) |
| Config file | None needed -- `go test ./...` |
| Quick run command | `cd uiscsi-ls && go test ./...` |
| Full suite command | `cd uiscsi-ls && go test -race ./...` |

## Requirements to Test Map

| Requirement | Behavior | Test Type | Test Function | File |
|-------------|----------|-----------|---------------|------|
| CLI-04 (D-01) | Columnar output format matches lsscsi style | unit | TestOutputColumnar | format_test.go |
| CLI-05 (D-02) | JSON output is valid and complete | unit | TestOutputJSON | format_test.go |
| CLI-01 (D-03/D-04) | Portal flag parsing, repeated flags | unit | TestPortalFlagRepeated, TestStringSlice | main_test.go |
| CLI-03 (D-05) | CHAP credential resolution (flag > env) | unit | TestResolveCHAP | probe_test.go |
| CLI-02 (D-06) | Default port 3260 | unit | TestNormalizePortal | probe_test.go |
| CLI-06 | Unreachable portal skip-and-continue | unit | TestProbePortalError | probe_test.go |
| D-07 | Full probe pipeline | integration | Manual against gotgt | N/A |

## Sampling Rate

- **Per task commit:** `cd uiscsi-ls && go test ./...`
- **Per wave merge:** `cd uiscsi-ls && go test -race ./...`
- **Phase gate:** Full suite green before verification

## Test Files

| File | Tests | Plan |
|------|-------|------|
| `uiscsi-ls/format_test.go` | TestDeviceTypeName, TestFormatCapacity, TestOutputColumnar, TestOutputJSON, TestOutputColumnarErrorPortal | 08-01 |
| `uiscsi-ls/probe_test.go` | TestNormalizePortal, TestResolveCHAP, TestProbePortalError | 08-02 |
| `uiscsi-ls/main_test.go` | TestStringSlice, TestPortalFlagRepeated, TestPortalFlagMissing | 08-02 |
