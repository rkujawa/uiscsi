---
status: partial
phase: 09-lio-e2e-tests
source: [09-VERIFICATION.md]
started: 2026-04-02T17:30:00Z
updated: 2026-04-02T17:30:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Basic Connectivity
expected: Discover returns target IQN; Dial succeeds; Inquiry returns VendorID="LIO-ORG"; ReadCapacity returns BlockSize consistent with 64MB LUN; TestUnitReady succeeds; Close returns nil
result: [pending]

### 2. Data Integrity
expected: bytes.Equal passes for both LBA 0 and LBA 100 write-then-read cycles
result: [pending]

### 3. CHAP Authentication
expected: One-way CHAP succeeds; bad password dial returns non-nil error
result: [pending]

### 4. Mutual CHAP
expected: Bidirectional CHAP completes without error; Inquiry after auth succeeds
result: [pending]

### 5. CRC32C Digests
expected: Dial with CRC32C header+data digests succeeds; write+read returns identical data
result: [pending]

### 6. Multi-LUN
expected: ReportLuns returns LUNs 0, 1, 2; ReadCapacity for each returns correct sizes for 32/64/128MB
result: [pending]

### 7. TMF LUN Reset
expected: LUNReset returns response=0 (Function Complete); Inquiry succeeds afterward
result: [pending]

### 8. Error Recovery Connection Drop
expected: ss -K kills TCP socket; retry loop succeeds within 10 attempts; Inquiry returns valid data after reconnect
result: [pending]

## Summary

total: 8
passed: 0
issues: 0
pending: 8
skipped: 0
blocked: 0

## Gaps
