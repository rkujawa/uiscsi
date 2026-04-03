# Phase 10: E2E Test Coverage Expansion (UNH-IOL Compliance Gaps) - Context

**Gathered:** 2026-04-02 (assumptions mode)
**Status:** Ready for planning

<domain>
## Phase Boundary

Close critical E2E test gaps identified by comparing against UNH-IOL iSCSI initiator test suites (Login Phase v3.0, Full Feature Phase v1.3, CHAP v3.1). Add ~10 new test scenarios covering: large data transfers (multi-R2T), login parameter negotiation boundaries (ImmediateData x InitialR2T 2x2 matrix, burst length limits), ERL 1/2 error recovery, additional TMFs (ABORT TASK, TARGET WARM RESET), digest variants (header-only, data-only), and SCSI error condition handling (sense data, out-of-range LBA). All tests run against real kernel LIO target via existing test/lio/ helper.

Scope: test code only (test/e2e/, test/lio/) plus a small library addition (WithOperationalOverrides login option). No changes to core protocol implementation.
</domain>

<decisions>
## Implementation Decisions

### Login Parameter Negotiation
- **D-01:** Add `WithOperationalOverrides(map[string]string)` as a single generic login option that patches `buildInitiatorKeys` output. This keeps the option surface minimal — one function handles all parameter overrides (InitialR2T, ImmediateData, FirstBurstLength, MaxBurstLength, ErrorRecoveryLevel). Individual typed options are not needed.
- **D-02:** ImmediateData x InitialR2T 2x2 matrix test uses the overrides to propose each combination, then performs a write+read to verify each mode works against LIO. LIO target-side params can also be set via configfs `param/` directory if needed to control negotiation outcome.
- **D-03:** Burst length boundary test writes data exceeding default MaxBurstLength (262144 bytes). A 1MB write (2048 blocks at 512 bytes) triggers ~4 R2T sequences. Verify data integrity after multi-R2T transfer.

### ERL 1/2 Testing
- **D-04:** Best-effort approach with `t.Skip`. Attempt ERL 1/2 negotiation (via WithOperationalOverrides setting ErrorRecoveryLevel=1 or 2). If LIO rejects (negotiates down to 0), `t.Skip` with clear message documenting the limitation. No TCP proxy or fault injection needed.
- **D-05:** If ERL > 0 negotiation succeeds, ERL 1 test: inject PDU loss (via `ss -K` or similar), verify SNACK recovery. ERL 2 test: kill connection mid-transfer, verify connection replacement within session. Both are stretch goals — passing with skip is an acceptable outcome per ROADMAP success criteria.

### Additional TMFs
- **D-06:** ABORT TASK test: start a large concurrent SCSI command (e.g., large read in a goroutine), then send AbortTask for that command's task tag. Verify the TMF response (Function Complete or Task Not Found — both are valid since the command may complete before the abort arrives). Uses `WithPDUHook` or direct access to get the task tag.
- **D-07:** TARGET WARM RESET test: send TargetWarmReset TMF. If LIO returns Function Complete, verify session can be re-established. If LIO returns Not Supported, `t.Skip` with documented limitation. Session may be killed by the reset — test must handle this gracefully.

### Digest Variants
- **D-08:** Header-only digest test: `WithHeaderDigest("CRC32C")` without `WithDataDigest` (defaults to None). Perform read+write cycle to verify header digests computed correctly while data flows without digest.
- **D-09:** Data-only digest test: `WithDataDigest("CRC32C")` without `WithHeaderDigest`. Same verification pattern.

### SCSI Error Conditions
- **D-10:** Out-of-range LBA test: write to LBA beyond capacity (e.g., LBA 200000 on a 64MB/131072-block LUN). Assert full SPC-4 sense tuple: SenseKey=ILLEGAL_REQUEST (0x05), ASC=0x21, ASCQ=0x00 ("Logical block address out of range"). Use `errors.As` to extract `SCSIError` with parsed sense data.
- **D-11:** CHECK CONDITION with sense data test: verify that the library correctly parses and reports sense data from a real target. This is implicitly tested by D-10 but should be explicit — the error string must include the sense key name and ASC/ASCQ description.

### Test Infrastructure
- **D-12:** All new tests follow Phase 9 patterns: `//go:build e2e`, package `e2e_test`, `lio.RequireRoot(t)`, `lio.RequireModules(t)`, unique TargetSuffix, 30-60s context timeouts, `defer cleanup()`.
- **D-13:** No changes to test/lio/lio.go Setup — the existing iblock+loop helper handles all needed configurations. CHAP tests are not in scope (already complete from Phase 9).

### Claude's Discretion
- Exact test data patterns for large transfers
- How to obtain task tag for ABORT TASK (PDU hook vs internal API)
- Whether to use subtests (t.Run) for the 2x2 matrix or separate test functions
- Retry/backoff strategy for ERL tests if negotiation needs retries
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing E2E test patterns
- `test/e2e/e2e_test.go` — TestMain + TestBasicConnectivity pattern
- `test/e2e/data_test.go` — Write+read integrity pattern with ReadCapacity
- `test/e2e/recovery_test.go` — Connection drop + reconnect pattern
- `test/e2e/tmf_test.go` — TMF execution pattern
- `test/e2e/digest_test.go` — Digest negotiation + data transfer pattern

### LIO helper
- `test/lio/lio.go` — Setup(), Config struct, iblock backstores, configfs paths
- `test/lio/sweep.go` — Orphan cleanup

### Library internals (read to understand capabilities)
- `internal/login/login.go` — `buildInitiatorKeys()` (hardcoded params to override), `loginConfig` struct
- `internal/session/dataout.go` — R2T handling, burst length enforcement
- `internal/session/recovery.go` — ERL 0 reconnect, SNACK (if ERL 1)
- `internal/session/tmf.go` — AbortTask, LUNReset, TargetWarmReset, TargetColdReset
- `internal/session/snack.go` — SNACK implementation (ERL 1)
- `internal/session/connreplace.go` — Connection replacement (ERL 2)
- `internal/transport/framer.go` — Digest computation (CRC32C)
- `internal/scsi/sense.go` — Sense data parsing
- `internal/scsi/opcode.go` — SenseKey constants, CommandError type, IsSenseKey()

### Public API
- `session.go` — ReadCapacity, WriteBlocks, ReadBlocks, AbortTask, LUNReset, TargetWarmReset
- `options.go` — WithHeaderDigest, WithDataDigest, WithOperationalOverrides (to be added)
- `errors.go` — SCSIError type with sense data

### RFC references
- RFC 7143 Section 13 — Login negotiation keys (ImmediateData, InitialR2T, burst lengths)
- RFC 7143 Section 12.1 — Digest negotiation
- RFC 7143 Section 11.5 — Task Management Function Request
- RFC 7143 Section 7.2-7.4 — Error Recovery Levels 0, 1, 2
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/lio/lio.go` Setup() already handles iblock backstores, ephemeral ports, CHAP/non-CHAP — no changes needed
- `test/e2e/data_test.go` pattern of write-then-read with `bytes.Equal` verification — reuse for large transfer test
- `test/e2e/recovery_test.go` pattern of `ss -K` connection kill — reuse for ERL tests
- `internal/scsi/opcode.go` has `IsSenseKey()` helper and full sense key constants — use for error condition tests

### Established Patterns
- All E2E tests use `initiatorIQN` constant from data_test.go
- Each test creates its own LIO target with unique suffix
- Non-CHAP tests use generate_node_acls=1 (auto-ACLs)
- ReadCapacity falls back from RC16 to RC10 for LIO compatibility

### Integration Points
- `WithOperationalOverrides` needs to be added to `options.go` (public) and `internal/login/login.go` (loginConfig + buildInitiatorKeys)
- TMF tests may need access to task tags — check if AbortTask accepts a tag or needs a running command
- ERL negotiation requires `ErrorRecoveryLevel` override via the new WithOperationalOverrides
</code_context>

<specifics>
## Specific Ideas

- UNH-IOL test suites: Login Phase v3.0, FFP v1.3, CHAP v3.1 — these are the compliance reference
- The ImmediateData x InitialR2T 2x2 matrix is the core UNH-IOL FFP test: each cell exercises a different data path (solicited vs unsolicited, with/without immediate data)
- For ABORT TASK: the concurrent command approach mirrors how real applications interact — a slow large read gets cancelled
</specifics>

<deferred>
## Deferred Ideas

- Multi-connection per session (MC/S) testing — out of scope by design decision (Phase 1)
- Discovery session edge cases (multi-portal, timeout) — low priority per UNH-IOL comparison
- Async event handling tests — medium priority but not in success criteria
- ABORT TASK SET TMF — lower priority than ABORT TASK
- TARGET COLD RESET TMF — destructive, hard to test meaningfully

None of these block RFC 7143 compliance for the v1.0 release.
</deferred>

---

*Phase: 10-e2e-test-coverage-expansion-unh-iol-compliance-gaps*
*Context gathered: 2026-04-02*
