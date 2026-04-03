# Phase 10: E2E Test Coverage Expansion - Discussion Log (Assumptions Mode)

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the analysis.

**Date:** 2026-04-02
**Phase:** 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps
**Mode:** assumptions
**Areas analyzed:** Login Parameter Negotiation, ERL 1/2 Testing, Large Data Transfer, SCSI Error Conditions, TMFs, Digest Variants

## Assumptions Presented

### Login Parameter Negotiation
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Need new login options since buildInitiatorKeys hardcodes all operational params | Confident | internal/login/login.go lines 500-517, loginConfig has no fields for operational params |
| Generic map override (WithOperationalOverrides) preferred over individual typed options | Confirmed | User selected this approach |

### ERL 1/2 Testing
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| LIO likely doesn't support ERL > 0, tests should use best-effort + t.Skip | Likely | Initiator hardcodes ERL=0 in login.go line 515; SNACK/connreplace exist but only unit-tested |
| Best-effort with t.Skip matches success criteria "(or documents LIO limitation)" | Confirmed | User selected this approach |

### Large Data Transfer
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| 1MB write (2048 blocks) exceeds MaxBurstLength (262144) triggering multi-R2T | Confident | login.go default MaxBurstLength=262144, dataout.go caps bursts at MaxBurstLength |

### SCSI Error Conditions
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Out-of-range LBA triggers CHECK CONDITION with ILLEGAL REQUEST sense | Likely | sense.go handles both fixed/descriptor format, ASC table includes 0x2100 |
| Full SPC-4 tuple assertion (SenseKey + ASC + ASCQ) preferred | Confirmed | User selected this approach |

## Corrections Made

No corrections — all assumptions confirmed.

## External Research Needed

- LIO ErrorRecoveryLevel support (kernel 6.19) — determines if ERL 1/2 tests are feasible
- LIO configfs param/ paths for InitialR2T, ImmediateData, burst lengths
- LIO TARGET WARM RESET behavior
