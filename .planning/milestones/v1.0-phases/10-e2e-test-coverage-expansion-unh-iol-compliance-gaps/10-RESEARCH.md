# Phase 10: E2E Test Coverage Expansion (UNH-IOL Compliance Gaps) - Research

**Researched:** 2026-04-02
**Domain:** iSCSI E2E testing — login negotiation, data transfer, error recovery, task management, SCSI error handling
**Confidence:** HIGH

## Summary

Phase 10 adds ~10 new E2E test scenarios against a real Linux kernel LIO iSCSI target. The scope is test code only (`test/e2e/`, `test/lio/`) plus one small library addition (`WithOperationalOverrides` login option). All existing patterns from Phase 9 are reused directly — the test infrastructure, LIO helper, and E2E conventions are fully established.

The key technical challenges are: (1) `WithOperationalOverrides(map[string]string)` must patch `buildInitiatorKeys` output to override login parameters like ImmediateData, InitialR2T, FirstBurstLength, MaxBurstLength, and ErrorRecoveryLevel; (2) LIO supports ERL 0/1/2 but requires explicit `param/ErrorRecoveryLevel` configfs writes on the target side for the initiator to negotiate > 0; (3) ABORT TASK testing requires a concurrent long-running command to create an in-flight task tag; (4) TARGET WARM RESET may kill the session, requiring graceful handling in the test.

**Primary recommendation:** Add `WithOperationalOverrides` to the login option chain, then implement tests in groups: data transfer (large write + 2x2 matrix), digest variants (header-only, data-only), SCSI errors (out-of-range LBA, sense parsing), TMFs (ABORT TASK, TARGET WARM RESET), and ERL 1/2 (best-effort with t.Skip fallback).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Add `WithOperationalOverrides(map[string]string)` as a single generic login option that patches `buildInitiatorKeys` output. This keeps the option surface minimal -- one function handles all parameter overrides (InitialR2T, ImmediateData, FirstBurstLength, MaxBurstLength, ErrorRecoveryLevel). Individual typed options are not needed.
- **D-02:** ImmediateData x InitialR2T 2x2 matrix test uses the overrides to propose each combination, then performs a write+read to verify each mode works against LIO. LIO target-side params can also be set via configfs `param/` directory if needed to control negotiation outcome.
- **D-03:** Burst length boundary test writes data exceeding default MaxBurstLength (262144 bytes). A 1MB write (2048 blocks at 512 bytes) triggers ~4 R2T sequences. Verify data integrity after multi-R2T transfer.
- **D-04:** Best-effort approach with `t.Skip`. Attempt ERL 1/2 negotiation (via WithOperationalOverrides setting ErrorRecoveryLevel=1 or 2). If LIO rejects (negotiates down to 0), `t.Skip` with clear message documenting the limitation. No TCP proxy or fault injection needed.
- **D-05:** If ERL > 0 negotiation succeeds, ERL 1 test: inject PDU loss (via `ss -K` or similar), verify SNACK recovery. ERL 2 test: kill connection mid-transfer, verify connection replacement within session. Both are stretch goals -- passing with skip is an acceptable outcome per ROADMAP success criteria.
- **D-06:** ABORT TASK test: start a large concurrent SCSI command (e.g., large read in a goroutine), then send AbortTask for that command's task tag. Verify the TMF response (Function Complete or Task Not Found -- both are valid since the command may complete before the abort arrives). Uses `WithPDUHook` or direct access to get the task tag.
- **D-07:** TARGET WARM RESET test: send TargetWarmReset TMF. If LIO returns Function Complete, verify session can be re-established. If LIO returns Not Supported, `t.Skip` with documented limitation. Session may be killed by the reset -- test must handle this gracefully.
- **D-08:** Header-only digest test: `WithHeaderDigest("CRC32C")` without `WithDataDigest` (defaults to None). Perform read+write cycle to verify header digests computed correctly while data flows without digest.
- **D-09:** Data-only digest test: `WithDataDigest("CRC32C")` without `WithHeaderDigest`. Same verification pattern.
- **D-10:** Out-of-range LBA test: write to LBA beyond capacity (e.g., LBA 200000 on a 64MB/131072-block LUN). Assert full SPC-4 sense tuple: SenseKey=ILLEGAL_REQUEST (0x05), ASC=0x21, ASCQ=0x00 ("Logical block address out of range"). Use `errors.As` to extract `SCSIError` with parsed sense data.
- **D-11:** CHECK CONDITION with sense data test: verify that the library correctly parses and reports sense data from a real target. This is implicitly tested by D-10 but should be explicit -- the error string must include the sense key name and ASC/ASCQ description.
- **D-12:** All new tests follow Phase 9 patterns: `//go:build e2e`, package `e2e_test`, `lio.RequireRoot(t)`, `lio.RequireModules(t)`, unique TargetSuffix, 30-60s context timeouts, `defer cleanup()`.
- **D-13:** No changes to test/lio/lio.go Setup -- the existing iblock+loop helper handles all needed configurations. CHAP tests are not in scope (already complete from Phase 9).

### Claude's Discretion
- Exact test data patterns for large transfers
- How to obtain task tag for ABORT TASK (PDU hook vs internal API)
- Whether to use subtests (t.Run) for the 2x2 matrix or separate test functions
- Retry/backoff strategy for ERL tests if negotiation needs retries

### Deferred Ideas (OUT OF SCOPE)
- Multi-connection per session (MC/S) testing -- out of scope by design decision (Phase 1)
- Discovery session edge cases (multi-portal, timeout) -- low priority per UNH-IOL comparison
- Async event handling tests -- medium priority but not in success criteria
- ABORT TASK SET TMF -- lower priority than ABORT TASK
- TARGET COLD RESET TMF -- destructive, hard to test meaningfully
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| E2E-11 | Large data transfer exceeding MaxBurstLength (multi-R2T) | D-03: 1MB write (2048 blocks x 512B) triggers ~4 R2T sequences. `WithOperationalOverrides` sets MaxBurstLength. Existing `dataout.go` handleR2T and sendDataOutBurst handle multi-R2T. Write+read integrity verification pattern from `data_test.go`. |
| E2E-12 | ImmediateData x InitialR2T 2x2 negotiation matrix | D-01/D-02: `WithOperationalOverrides` sets both params. LIO configfs `param/` can also force target-side values. Table-driven subtests (t.Run) for 4 combinations. |
| E2E-13 | ERL 1 within-connection recovery (SNACK/DataACK) | D-04/D-05: Set `ErrorRecoveryLevel=1` via overrides + LIO configfs `param/ErrorRecoveryLevel=1`. LIO does support ERL 1. If negotiation fails, `t.Skip`. SNACK implementation in `snack.go`. |
| E2E-14 | ERL 2 connection replacement | D-04/D-05: Set `ErrorRecoveryLevel=2` via overrides + LIO configfs. LIO supports ERL 2. Connection replacement in `connreplace.go`. `t.Skip` if negotiation fails. |
| E2E-15 | ABORT TASK TMF with concurrent command | D-06: Start large read in goroutine, capture task tag via PDU hook, send AbortTask. Accept Function Complete (0) or Task Not In Task Set (5). |
| E2E-16 | TARGET WARM RESET TMF | D-07: Send TargetWarmReset. Handle session drop gracefully. Re-establish session after reset. `t.Skip` if target returns Not Supported. |
| E2E-17 | Header-only digest mode | D-08: `WithHeaderDigest("CRC32C")` only. Write+read cycle. Existing digest_test.go pattern with both digests can be split. |
| E2E-18 | Data-only digest mode | D-09: `WithDataDigest("CRC32C")` only. Same pattern as E2E-17. |
| E2E-19 | SCSI CHECK CONDITION with sense data | D-10/D-11: Write to out-of-range LBA. Extract `SCSIError` via `errors.As`. Verify SenseKey=0x05, ASC=0x21, ASCQ=0x00. Verify human-readable error message contains "ILLEGAL REQUEST" and "Logical block address out of range". |
| E2E-20 | All tests skip gracefully (non-root, no modules) | D-12: `lio.RequireRoot(t)`, `lio.RequireModules(t)` at top of every test. Already established pattern. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | Go 1.25 | Test framework | Project constraint -- no testify |
| Go stdlib `errors` | Go 1.25 | `errors.As` for SCSIError extraction | Standard error inspection pattern |
| `test/lio` (internal) | N/A | LIO configfs target setup/teardown | Phase 9 established infrastructure |
| `uiscsi` (public API) | HEAD | All SCSI commands, TMFs, options | The library under test |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/binary` | stdlib | Parse PDU BHS in hook to extract ITT | ABORT TASK test needs task tag from PDU hook |
| `sync` | stdlib | WaitGroup/Mutex for concurrent test coordination | ABORT TASK concurrent goroutine |
| `os/exec` | stdlib | `ss -K` for connection kill in ERL tests | Same pattern as recovery_test.go |

## Architecture Patterns

### Recommended Test File Structure
```
test/e2e/
  e2e_test.go           # TestMain (existing)
  data_test.go          # existing
  recovery_test.go      # existing
  tmf_test.go           # existing (extend with AbortTask, TargetWarmReset)
  digest_test.go        # existing (extend with header-only, data-only)
  largewrite_test.go    # NEW: large write + multi-R2T
  negotiation_test.go   # NEW: 2x2 matrix + burst length boundaries
  erl_test.go           # NEW: ERL 1/2 tests
  scsierror_test.go     # NEW: out-of-range LBA, sense data parsing
```

### Pattern 1: WithOperationalOverrides Implementation
**What:** A single login option that takes `map[string]string` and patches the output of `buildInitiatorKeys`.
**When to use:** Whenever tests need to override default negotiation parameters.
**Implementation path:**
```go
// In internal/login/login.go -- add to loginConfig:
type loginConfig struct {
    // ... existing fields ...
    operationalOverrides map[string]string // D-01
}

// New login option:
func WithOperationalOverrides(overrides map[string]string) LoginOption {
    return func(c *loginConfig) {
        c.operationalOverrides = overrides
    }
}

// In buildInitiatorKeys, after building default keys, apply overrides:
func buildInitiatorKeys(cfg *loginConfig) []KeyValue {
    keys := []KeyValue{
        // ... existing defaults ...
    }
    if cfg.operationalOverrides != nil {
        for i, kv := range keys {
            if override, ok := cfg.operationalOverrides[kv.Key]; ok {
                keys[i].Value = override
            }
        }
    }
    return keys
}

// Public wrapper in options.go:
func WithOperationalOverrides(overrides map[string]string) Option {
    return func(c *dialConfig) {
        c.loginOpts = append(c.loginOpts, login.WithOperationalOverrides(overrides))
    }
}
```

### Pattern 2: Table-Driven 2x2 Matrix with Subtests
**What:** Use `t.Run` with named subtests for the ImmediateData x InitialR2T matrix.
**Recommendation:** Use subtests (`t.Run`) -- cleaner output, can run individual cells.
```go
func TestNegotiation_ImmediateDataInitialR2T(t *testing.T) {
    lio.RequireRoot(t)
    lio.RequireModules(t)

    matrix := []struct {
        name         string
        immediateData string
        initialR2T    string
    }{
        {"ImmYes_R2TYes", "Yes", "Yes"},
        {"ImmYes_R2TNo", "Yes", "No"},
        {"ImmNo_R2TYes", "No", "Yes"},
        {"ImmNo_R2TNo", "No", "No"},
    }

    for _, tc := range matrix {
        t.Run(tc.name, func(t *testing.T) {
            tgt, cleanup := lio.Setup(t, lio.Config{
                TargetSuffix: "neg-" + strings.ToLower(tc.name),
                InitiatorIQN: initiatorIQN,
            })
            defer cleanup()
            // Set target-side params via configfs if needed
            // Connect with WithOperationalOverrides
            // Write + Read to verify data path works
        })
    }
}
```

### Pattern 3: Task Tag Capture via PDU Hook
**What:** Use `WithPDUHook` to capture the ITT of a SCSI Command PDU for ABORT TASK.
**Recommendation:** PDU hook approach -- it uses the public API only and avoids needing internal access.
```go
var capturedITT uint32
var ittCaptured sync.WaitGroup
ittCaptured.Add(1)

sess, err := uiscsi.Dial(ctx, tgt.Addr,
    uiscsi.WithTarget(tgt.IQN),
    uiscsi.WithInitiatorName(initiatorIQN),
    uiscsi.WithPDUHook(func(dir uiscsi.PDUDirection, data []byte) {
        if dir == uiscsi.HookSend && len(data) >= 48 {
            opcode := data[0] & 0x3F
            if opcode == 0x01 { // SCSI Command opcode
                itt := binary.BigEndian.Uint32(data[16:20])
                atomic.StoreUint32(&capturedITT, itt)
                ittCaptured.Done()
            }
        }
    }),
)
```

### Pattern 4: LIO Target-Side Parameter Override via configfs
**What:** Set iSCSI negotiation parameters on the LIO target side to force specific negotiation outcomes.
**Path:** `/sys/kernel/config/target/iscsi/{iqn}/tpgt_1/param/{key}`
**When to use:** When the initiator's proposed value needs the target to also agree (e.g., for ERL > 0, target must accept).
```go
// Set ErrorRecoveryLevel on target side
tpgDir := filepath.Join("/sys/kernel/config/target/iscsi", tgt.IQN, "tpgt_1")
paramDir := filepath.Join(tpgDir, "param")
os.WriteFile(filepath.Join(paramDir, "ErrorRecoveryLevel"), []byte("2"), 0o644)
```
**Note:** This pattern is already established -- `lio.go` uses `tpgDir/param/AuthMethod` in Setup.

### Anti-Patterns to Avoid
- **Sharing LIO targets between subtests:** Each subtest MUST create its own target with unique suffix. LIO does not support changing negotiated params mid-session.
- **Assuming ERL > 0 will always work:** LIO support for ERL 1/2 is kernel-version dependent. Always use `t.Skip` when negotiation fails.
- **Synchronous AbortTask without concurrent command:** AbortTask requires an in-flight task. Phase 9 correctly identified this limitation and deferred to Phase 10's concurrent approach.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Login param override | Custom option per parameter | `WithOperationalOverrides(map[string]string)` | D-01: single generic override function covers all params |
| Task tag extraction | Internal session API access | `WithPDUHook` + BHS byte parsing | Public API only -- opcode at byte 0, ITT at bytes 16-19 |
| LIO param configuration | Shell out to targetcli | Direct configfs `param/` write | Already established in lio.go, no dependency on targetcli |
| Sense data extraction | Manual byte parsing of errors | `errors.As(&SCSIError{})` | Library already parses sense data in `submitAndCheck` |

## Common Pitfalls

### Pitfall 1: ImmediateData=Yes + InitialR2T=Yes with Small Writes
**What goes wrong:** When ImmediateData=Yes but InitialR2T=Yes, the write sends immediate data up to FirstBurstLength in the SCSI Command PDU, then waits for R2T before sending more. If the write fits entirely in immediate data, no R2T is needed and the test works fine. But the test MUST use a write large enough to trigger R2T for the matrix to be meaningful.
**Why it happens:** Small test payloads (e.g., 1 block) fit in immediate data, so all 4 matrix cells behave identically.
**How to avoid:** Use at least 16 blocks (8KB at 512B/block) for 2x2 matrix tests to ensure the write exceeds immediate data capacity and exercises the different code paths.
**Warning signs:** All 4 matrix cells pass identically with no behavioral difference in logs.

### Pitfall 2: TARGET WARM RESET Kills the Session
**What goes wrong:** After sending TargetWarmReset, LIO may terminate the session. Subsequent commands on the same session fail with connection errors.
**Why it happens:** RFC 7143 Section 11.5.1 says TARGET WARM RESET "affects all sessions to the target." LIO may tear down the calling initiator's session.
**How to avoid:** After the TMF response, attempt a simple command (Inquiry). If it fails with connection error, that's expected behavior. The test should then verify it can re-establish a new session by calling Dial again. Do NOT rely on the same session surviving.
**Warning signs:** Panic or test hang after TargetWarmReset due to unhandled session termination.

### Pitfall 3: ERL Negotiation Down-Negotiation
**What goes wrong:** Initiator proposes ERL=2, LIO accepts but negotiates down to ERL=0 or ERL=1.
**Why it happens:** ERL is a "minimum of" negotiation -- the result is min(initiator, target). If the target-side configfs param is not also set to 2, it defaults to 0 and the negotiation result is 0.
**How to avoid:** Set BOTH the initiator override AND the LIO target-side `param/ErrorRecoveryLevel` configfs value. Read back the negotiated params to verify actual ERL before proceeding with the test.
**Warning signs:** Test passes trivially because ERL was negotiated to 0 despite requesting 2.

### Pitfall 4: Race in ABORT TASK Hook-Based ITT Capture
**What goes wrong:** The PDU hook fires after the command is submitted but the ITT is read before the hook has been called (race between hook goroutine and test goroutine).
**Why it happens:** PDU hook is called synchronously in the write pump, but the test goroutine may try to read the captured ITT before the write pump processes the PDU.
**How to avoid:** Use `sync.WaitGroup` or a channel to synchronize: hook signals when ITT is captured, test waits before calling AbortTask. Note the hook may fire multiple times (NOP-Out, etc.) -- filter by opcode 0x01 (SCSI Command).
**Warning signs:** AbortTask called with ITT=0 (zero-value before hook fires).

### Pitfall 5: Out-of-Range LBA Sense Data Format Varies by Kernel
**What goes wrong:** Some kernel versions return fixed-format (0x70) sense data while others return descriptor-format (0x72). The ASC/ASCQ is the same but the parsing path differs.
**Why it happens:** LIO's sense data format depends on kernel version and SPC-4 compliance level.
**How to avoid:** The library's `ParseSense` already handles both 0x70/0x71 and 0x72/0x73 formats. Test should only assert on the parsed SenseKey/ASC/ASCQ values, not on raw bytes or response code format.
**Warning signs:** Test fails on format assertion but the actual sense key is correct.

### Pitfall 6: MaxBurstLength Configfs Default
**What goes wrong:** LIO's default MaxBurstLength is 262144 (256KB). If the initiator proposes 262144 and the target already has 262144, a 1MB write does trigger multi-R2T. But if either side has a different default, the actual negotiated value may differ.
**How to avoid:** Explicitly set MaxBurstLength on both sides (initiator via WithOperationalOverrides, target via configfs). The test data size (1MB = 1048576 bytes) at MaxBurstLength=262144 produces exactly 4 R2T sequences (1048576 / 262144 = 4), making the test deterministic.
**Warning signs:** Fewer R2T sequences than expected, or write completing with only immediate data.

## Code Examples

### Example 1: Large Write with Multi-R2T Verification
```go
// Source: Derived from test/e2e/data_test.go pattern + D-03
func TestLargeWrite_MultiR2T(t *testing.T) {
    lio.RequireRoot(t)
    lio.RequireModules(t)

    tgt, cleanup := lio.Setup(t, lio.Config{
        TargetSuffix: "large",
        InitiatorIQN: initiatorIQN,
    })
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    sess, err := uiscsi.Dial(ctx, tgt.Addr,
        uiscsi.WithTarget(tgt.IQN),
        uiscsi.WithInitiatorName(initiatorIQN),
        // Default MaxBurstLength=262144, a 1MB write triggers ~4 R2T sequences
    )
    if err != nil {
        t.Fatalf("Dial: %v", err)
    }
    defer sess.Close()

    cap, err := sess.ReadCapacity(ctx, 0)
    if err != nil {
        t.Fatalf("ReadCapacity: %v", err)
    }

    // 1MB = 2048 blocks at 512B, exceeds default MaxBurstLength (262144)
    const numBlocks = 2048
    testData := make([]byte, numBlocks*int(cap.BlockSize))
    for i := range testData {
        testData[i] = byte(i % 251) // prime modulus for pattern detection
    }

    if err := sess.WriteBlocks(ctx, 0, 0, numBlocks, cap.BlockSize, testData); err != nil {
        t.Fatalf("WriteBlocks(1MB): %v", err)
    }

    readBack, err := sess.ReadBlocks(ctx, 0, 0, numBlocks, cap.BlockSize)
    if err != nil {
        t.Fatalf("ReadBlocks(1MB): %v", err)
    }

    if !bytes.Equal(readBack, testData) {
        t.Fatal("1MB write-then-read data integrity check failed")
    }
    t.Log("1MB multi-R2T write: data integrity OK")
}
```

### Example 2: Out-of-Range LBA Error Handling
```go
// Source: Derived from errors.go SCSIError pattern + D-10
func TestSCSIError_OutOfRangeLBA(t *testing.T) {
    lio.RequireRoot(t)
    lio.RequireModules(t)

    tgt, cleanup := lio.Setup(t, lio.Config{
        TargetSuffix: "scsierr",
        InitiatorIQN: initiatorIQN,
    })
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    sess, err := uiscsi.Dial(ctx, tgt.Addr,
        uiscsi.WithTarget(tgt.IQN),
        uiscsi.WithInitiatorName(initiatorIQN),
    )
    if err != nil {
        t.Fatalf("Dial: %v", err)
    }
    defer sess.Close()

    // 64MB LUN = 131072 blocks at 512B. LBA 200000 is out of range.
    oneBlock := make([]byte, 512)
    err = sess.WriteBlocks(ctx, 0, 200000, 1, 512, oneBlock)
    if err == nil {
        t.Fatal("expected error for out-of-range LBA, got nil")
    }

    var scsiErr *uiscsi.SCSIError
    if !errors.As(err, &scsiErr) {
        t.Fatalf("expected *SCSIError, got %T: %v", err, err)
    }

    // SenseKey=ILLEGAL_REQUEST (0x05), ASC=0x21, ASCQ=0x00
    if scsiErr.SenseKey != 0x05 {
        t.Errorf("SenseKey: got 0x%02X, want 0x05 (ILLEGAL_REQUEST)", scsiErr.SenseKey)
    }
    if scsiErr.ASC != 0x21 || scsiErr.ASCQ != 0x00 {
        t.Errorf("ASC/ASCQ: got 0x%02X/0x%02X, want 0x21/0x00", scsiErr.ASC, scsiErr.ASCQ)
    }

    // Verify human-readable message
    if !strings.Contains(scsiErr.Error(), "ILLEGAL REQUEST") {
        t.Errorf("error message should contain 'ILLEGAL REQUEST': %s", scsiErr.Error())
    }
}
```

### Example 3: LIO ConfigFS Parameter Override for ERL
```go
// Source: Derived from lio.go configfs patterns + D-04
// Set ERL on target side before initiator connects
tpgDir := filepath.Join("/sys/kernel/config/target/iscsi", tgt.IQN, "tpgt_1")
paramPath := filepath.Join(tpgDir, "param", "ErrorRecoveryLevel")
if err := os.WriteFile(paramPath, []byte("2"), 0o644); err != nil {
    t.Skipf("cannot set ErrorRecoveryLevel=2 on target: %v (kernel may not support ERL>0)", err)
}

// Connect with ERL=2 from initiator side
sess, err := uiscsi.Dial(ctx, tgt.Addr,
    uiscsi.WithTarget(tgt.IQN),
    uiscsi.WithInitiatorName(initiatorIQN),
    uiscsi.WithOperationalOverrides(map[string]string{
        "ErrorRecoveryLevel": "2",
    }),
)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 9: AbortTask not E2E tested (synchronous limitation) | Phase 10: concurrent goroutine + PDU hook for ITT capture | Phase 10 | Enables real ABORT TASK E2E coverage |
| Phase 9: only ERL 0 tested | Phase 10: ERL 1/2 with LIO configfs param override | Phase 10 | Covers RFC 7143 Sections 7.2-7.4 |
| Phase 9: only both-digests tested | Phase 10: header-only and data-only modes | Phase 10 | Covers asymmetric digest configurations |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.25) |
| Config file | None (go test with build tags) |
| Quick run command | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestName` |
| Full suite command | `sudo go test -tags e2e -v -count=1 -timeout 300s ./test/e2e/` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| E2E-11 | Large write >MaxBurstLength with multi-R2T | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestLargeWrite` | Wave 0 |
| E2E-12 | ImmediateData x InitialR2T 2x2 matrix | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestNegotiation` | Wave 0 |
| E2E-13 | ERL 1 SNACK recovery | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestERL1` | Wave 0 |
| E2E-14 | ERL 2 connection replacement | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestERL2` | Wave 0 |
| E2E-15 | ABORT TASK with concurrent command | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestTMF_AbortTask` | Wave 0 |
| E2E-16 | TARGET WARM RESET | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestTMF_TargetWarmReset` | Wave 0 |
| E2E-17 | Header-only digest | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDigest_HeaderOnly` | Wave 0 |
| E2E-18 | Data-only digest | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestDigest_DataOnly` | Wave 0 |
| E2E-19 | SCSI CHECK CONDITION + sense data | e2e | `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestSCSIError` | Wave 0 |
| E2E-20 | Graceful skip when non-root/no modules | e2e | `go test -tags e2e -v -count=1 ./test/e2e/ -run TestBasicConnectivity` (as non-root) | Existing |

### Sampling Rate
- **Per task commit:** `sudo go test -tags e2e -v -count=1 ./test/e2e/ -run TestName`
- **Per wave merge:** `sudo go test -tags e2e -v -count=1 -timeout 300s ./test/e2e/`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `test/e2e/largewrite_test.go` -- covers E2E-11
- [ ] `test/e2e/negotiation_test.go` -- covers E2E-12
- [ ] `test/e2e/erl_test.go` -- covers E2E-13, E2E-14
- [ ] `test/e2e/scsierror_test.go` -- covers E2E-19
- [ ] `options.go` + `internal/login/login.go` -- `WithOperationalOverrides` (required by E2E-12, E2E-13, E2E-14)

## Open Questions

1. **Hook-based ITT capture reliability for ABORT TASK**
   - What we know: `WithPDUHook` fires synchronously in the write pump. The hook receives concatenated BHS+DataSegment. Opcode is at byte 0 (& 0x3F), ITT at bytes 16-19.
   - What's unclear: Whether the hook always fires before the target processes the command (i.e., whether there's a window where the command completes before we can abort it).
   - Recommendation: Accept both TMF response codes (Function Complete = 0, Task Not In Task Set = 5) as valid outcomes. The test validates the TMF mechanism, not guaranteed abort success.

2. **LIO ERL 1/2 kernel version requirements**
   - What we know: LIO's linux-iscsi.org wiki documents ERL 0-2 support. The configfs `param/ErrorRecoveryLevel` path exists.
   - What's unclear: Whether the running kernel (6.19.8-200.fc43) has full ERL 1/2 support or if some code paths are stubs.
   - Recommendation: Per D-04, use `t.Skip` if negotiation fails or target negotiates down. This is explicitly acceptable per success criteria.

3. **LIO configfs param write timing relative to TPG enable**
   - What we know: `lio.go` Setup enables the TPG at the end. Params like AuthMethod are set before enable.
   - What's unclear: Whether `param/ErrorRecoveryLevel` (or ImmediateData/InitialR2T) can be changed on an already-enabled TPG, or if they must be set before enable.
   - Recommendation: Set target-side params AFTER lio.Setup returns but BEFORE initiator connects. If configfs rejects the write, the test should `t.Skip` with a clear message. Alternatively, extend lio.Setup with a Params field, but D-13 says no changes to Setup, so post-Setup configfs writes are the correct approach.

## Sources

### Primary (HIGH confidence)
- Codebase files: `test/e2e/*.go`, `test/lio/lio.go`, `options.go`, `errors.go`, `session.go`, `internal/login/login.go`, `internal/session/tmf.go`, `internal/session/recovery.go`, `internal/session/snack.go`, `internal/session/connreplace.go`, `internal/session/dataout.go`, `internal/scsi/sense.go`, `internal/scsi/opcode.go`
- RFC 7143 Sections 7.2-7.4 (ERL 0/1/2), 11.5 (TMF), 11.16 (SNACK), 13 (negotiation keys)

### Secondary (MEDIUM confidence)
- [Linux SCSI Target - Error Recovery Level](https://linux-iscsi.org/wiki/error_recovery_level) -- LIO ERL 0-2 support and configfs configuration
- [LIO target configfs param documentation](http://linux-iscsi.org/wiki/Target/configFS) -- configfs parameter paths

### Tertiary (LOW confidence)
- [ErrorRecoveryLevel > 0 on LIO Target (mailing list)](https://target-devel.vger.kernel.narkive.com/XdCTJziU/errorrecoverylevel-0-on-lio-target) -- Community discussion on ERL > 0 support (date unknown, may be outdated)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, all patterns established in Phase 9
- Architecture: HIGH -- extends existing E2E test patterns, no new paradigms
- Pitfalls: HIGH -- based on direct code reading and established LIO behavior
- ERL 1/2 testing: MEDIUM -- LIO claims support but kernel-version dependent; t.Skip fallback mitigates risk

**Research date:** 2026-04-02
**Valid until:** 2026-05-02 (stable -- iSCSI protocol and LIO configfs are mature interfaces)
