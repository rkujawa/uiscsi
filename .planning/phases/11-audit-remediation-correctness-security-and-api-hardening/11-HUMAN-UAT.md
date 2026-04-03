---
status: partial
phase: 11-audit-remediation-correctness-security-and-api-hardening
source: [11-VERIFICATION.md]
started: 2026-04-03T00:00:00Z
updated: 2026-04-03T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Non-auth login failure wraps as TransportError
expected: errors.As(err, &te) succeeds for *TransportError; errors.As(err, &ae) fails for *AuthError
result: [pending]

### 2. SNACK send with full writeCh times out
expected: After 5 seconds, sendSNACK returns non-nil error containing 'SNACK send timed out'
result: [pending]

### 3. Execute() with 17-byte CDB returns error
expected: err.Error() contains 'exceeds maximum 16 bytes'; no PDU sent to target
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
