# Quick Task 260404-vq3: Summary

## Task
Build test matrix comparing E2E tests to UNH-IOL Initiator Full Feature Phase test suite.

## What was done
Created `doc/test_matrix_initiator_ffp.md` — a comprehensive matrix mapping all 62 UNH-IOL FFP tests to existing uiscsi E2E and conformance test coverage.

## Key findings
- **11 tests (18%)** fully covered by existing E2E/conformance tests
- **22 tests (35%)** partially covered (unit tests exist but no wire-level E2E validation)
- **29 tests (47%)** not covered

### Well-covered areas
- Logout, NOP-Out/In, data transfer integrity, ImmediateData/InitialR2T negotiation matrix, text request basics

### Main gaps
- ERL 2 features (connection reassignment, task reassign)
- Async message handling (all 4 tests)
- Specific SCSI error status handling (BUSY, RESERVATION CONFLICT, CRC error sense)
- PDU-level field validation in E2E tests (CmdSN, DataSN, F-bit, TTT)
- Abort Task Set TMF
- Text Request advanced features (TTT continuation, reset)

## Files
- `doc/test_matrix_initiator_ffp.md` (created)
- `.planning/STATE.md` (updated)
