# Phase 2: Connection and Login - Research

**Researched:** 2026-03-31
**Domain:** iSCSI login state machine, CHAP authentication, RFC 7143 Section 13 operational parameter negotiation
**Confidence:** HIGH

## Summary

Phase 2 builds the login layer on top of the Phase 1 transport. The existing codebase already has `LoginReq`/`LoginResp` PDU types with full BHS marshal/unmarshal, a `transport.Conn` with `SetDigests()` and `SetMaxRecvDSL()` hooks, and the `Router`/`ReadPump`/`WritePump` infrastructure. Phase 2 needs to implement: (1) a text key-value codec for the login data segment, (2) a declarative negotiation engine that processes RFC 7143 Section 13 keys by type (BooleanAnd, BooleanOr, NumericalMin, NumericalMax, ListSelect), (3) the login state machine (SecurityNegotiation -> LoginOperationalNegotiation -> FullFeaturePhase), (4) CHAP authentication (one-way and mutual), and (5) digest activation post-negotiation.

A critical finding: **gotgt (the test target) does NOT support CHAP** -- it only supports `AuthMethod=None`. CHAP tests must use a mock iSCSI target implemented in the test harness itself, simulating the target side of the CHAP exchange. AuthMethod=None tests can use gotgt or a simpler mock. Digest activation tests need a mock target that negotiates `CRC32C` and sends PDUs with digests -- gotgt defaults `HeaderDigest=None, DataDigest=None` and may not support digest negotiation either.

**Primary recommendation:** Build the negotiation engine as a standalone, heavily unit-tested component with the declarative key registry pattern (D-04). Layer the login state machine on top, and test CHAP via a mock target that speaks just enough iSCSI to exercise all authentication paths.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Functional options pattern for login configuration -- `conn.Login(ctx, WithTarget("iqn..."), WithCHAP(user, secret), WithHeaderDigest(CRC32C), ...)`
- **D-02:** Separate Dial + Login steps -- keep `transport.Dial()` and `conn.Login()` as distinct operations. Caller controls connection lifecycle. No combined Connect() convenience in this phase.
- **D-03:** Login returns a session handle (or connection-level result) with access to negotiated parameters
- **D-04:** Declarative key registry -- each RFC 7143 Section 13 key is a struct describing its type (BoolAnd, BoolOr, NumericMin, NumericMax, ListSelect), default value, valid range, and RFC reference. A generic engine processes all keys uniformly.
- **D-05:** Negotiated parameters stored in a typed `NegotiatedParams` struct with direct field access (HeaderDigest bool, DataDigest bool, MaxRecvDSL uint32, MaxBurstLen, FirstBurstLen, InitialR2T, ImmediateData, MaxOutR2T, etc.). Compile-time safe, no string map lookups at use sites.
- **D-06:** CHAP credentials provided via functional options -- `WithCHAP(user, secret)` for one-way CHAP, `WithMutualCHAP(user, secret, targetSecret)` for bidirectional authentication. No callback interface or credential provider abstraction in v1.
- **D-07:** AuthMethod=None is the default when no CHAP options are provided
- **D-08:** Typed `LoginError` struct with StatusClass (uint8) and StatusDetail (uint8) fields mapping directly to RFC 7143 Section 11.13 login response status codes. Human-readable Message field. Callers inspect via `errors.As()`.
- **D-09:** StatusClass values: 0=success, 1=redirect, 2=initiator error, 3=target error -- direct mapping from the spec

### Claude's Discretion
- Login state machine internal design (flat function vs explicit state type)
- How login PDU exchanges are sequenced internally (loop vs recursive steps)
- CHAP challenge/response crypto implementation details
- Text key-value encoding/decoding format details (key=value\0 pairs in data segment)
- Internal package organization for login code (internal/login/ vs internal/conn/)
- Whether NegotiatedParams is embedded in a connection/session type or returned standalone

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LOGIN-01 | Full login phase state machine (security negotiation, operational negotiation, leading connection, normal connection) | RFC 7143 Section 6 login stages (CSG 0, 1, 3), Transit/Continue bit semantics, existing LoginReq/LoginResp PDU types |
| LOGIN-02 | Text key-value negotiation engine for all RFC 7143 Section 13 mandatory keys | Declarative key registry pattern (D-04), five negotiation types documented below, all 14 mandatory keys catalogued |
| LOGIN-03 | AuthMethod=None authentication | Simplest login path -- CSG=0 with AuthMethod=None, Transit to CSG=1 or CSG=3. gotgt supports this natively |
| LOGIN-04 | CHAP authentication (one-way: target authenticates initiator) | CHAP_A/CHAP_I/CHAP_C/CHAP_N/CHAP_R exchange sequence documented, MD5 hash algorithm (id+secret+challenge), hex encoding with 0x prefix |
| LOGIN-05 | Mutual CHAP authentication (bidirectional: both sides authenticate) | Initiator sends its own CHAP_I+CHAP_C alongside CHAP_N+CHAP_R, target responds with CHAP_N+CHAP_R for initiator's challenge |
| LOGIN-06 | Operational parameter negotiation (HeaderDigest, DataDigest, MaxRecvDataSegmentLength, MaxBurstLength, FirstBurstLength, InitialR2T, ImmediateData, MaxOutstandingR2T, DataPDUInOrder, DataSequenceInOrder, DefaultTime2Wait, DefaultTime2Retain, MaxConnections, ErrorRecoveryLevel) | Full key registry with negotiation types, defaults, and valid ranges documented in Architecture Patterns section |
| INTEG-01 | Header digest negotiation and CRC32C verification on received PDUs | HeaderDigest list negotiation, existing digest.HeaderDigest() function, Conn.SetDigests() hook |
| INTEG-02 | Data digest negotiation and CRC32C verification on received PDUs | DataDigest list negotiation, existing digest.DataDigest() function, ReadRawPDU already handles digest reads |
| INTEG-03 | Digest generation on outgoing PDUs when negotiated | WriteRawPDU already handles HasHDigest/HasDDigest flags, login must activate via SetDigests() |
| TEST-04 | Parameterized tests for negotiation parameter matrix | Table-driven tests per negotiation type (BoolAnd, BoolOr, NumMin, NumMax, ListSelect) with combinatorial coverage |

</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Language:** Go 1.25 (verified: go1.25.5 netbsd/amd64 installed)
- **Dependencies:** Minimal external -- stdlib only for production code. `gostor/gotgt` for integration tests only.
- **Standard:** RFC 7143 compliance drives implementation
- **Testing:** Must be testable without manual infrastructure. Table-driven tests with `t.Run` subtests.
- **API style:** `context.Context` for cancellation, `io.Reader/Writer` where natural, structured errors via `errors.As()`
- **No testify:** Use stdlib `testing` package exclusively
- **Logging:** `log/slog` only -- library provides injectable Handler, no third-party loggers

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/md5` | stdlib | CHAP MD5 hash computation | RFC 1994 CHAP uses MD5. stdlib covers this. |
| `crypto/hmac` | stdlib | CHAP HMAC support | Available but CHAP actually uses raw MD5(id+secret+challenge), not HMAC. Still import if needed for mutual auth validation. |
| `crypto/rand` | stdlib | CHAP challenge generation (mutual) | Generate cryptographically random challenge bytes for mutual CHAP |
| `encoding/hex` | stdlib | CHAP binary value encoding | CHAP_C and CHAP_R use hex encoding with "0x" prefix on wire |
| `hash/crc32` | stdlib | Digest computation (already in Phase 1) | CRC32C Castagnoli for header/data digests |
| `bytes` | stdlib | Text key-value buffer construction | Building null-separated key=value pairs |
| `strings` | stdlib | Text key-value parsing | Splitting comma-separated lists, key=value parsing |
| `strconv` | stdlib | Numeric parameter conversion | Converting negotiation values to/from strings |
| `fmt` | stdlib | Error formatting | LoginError message construction |
| `errors` | stdlib | Error wrapping/unwrapping | errors.As() for LoginError |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing/synctest` | stdlib (Go 1.25) | Deterministic concurrent login tests | Testing login timeout behavior, state machine transitions with virtual time |
| `net` | stdlib | TCP pipe for tests | `net.Pipe()` for unit tests, loopback TCP for integration |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `crypto/md5` directly | `crypto/hmac` with MD5 | CHAP is NOT HMAC -- it's raw `MD5(id + secret + challenge)`. Using hmac would be incorrect. |
| `encoding/hex` for CHAP values | Manual hex formatting | stdlib hex package is cleaner and tested |
| Custom text codec | `bufio.Scanner` | Scanner doesn't handle null-byte delimiters naturally; manual split on `\0` is simpler |

## Architecture Patterns

### Recommended Project Structure
```
internal/
  login/
    login.go          # Login() function, state machine, functional options
    login_test.go     # Login integration tests with mock target
    negotiation.go    # Key registry, negotiation engine
    negotiation_test.go # Parameterized negotiation tests (TEST-04)
    chap.go           # CHAP authentication implementation
    chap_test.go      # CHAP unit tests
    textcodec.go      # key=value\0 encoding/decoding
    textcodec_test.go # Text codec tests
    errors.go         # LoginError type, status code constants
    params.go         # NegotiatedParams struct, defaults
  pdu/                # (existing)
  transport/          # (existing)
  digest/             # (existing)
```

### Pattern 1: Login State Machine
**What:** The login proceeds through stages using CSG/NSG fields in the LoginReq/LoginResp PDUs.
**When to use:** Every login attempt.

**Login stages (CSG/NSG encoding):**
- Stage 0: Security Negotiation
- Stage 1: Login Operational Negotiation
- Stage 3: Full Feature Phase (terminal)

**Stage transitions:**
```
AuthMethod=None path:
  CSG=0, NSG=1, T=1 (skip security, go to operational)
  OR CSG=0, NSG=3, T=1 (skip both, go straight to FFP if no operational needed)

CHAP path:
  CSG=0, NSG=0, T=0 (security exchange continues)
  CSG=0, NSG=1, T=1 (security done, move to operational)
  CSG=1, NSG=3, T=1 (operational done, move to FFP)
```

**Example:**
```go
// Login state machine loop
func (l *loginState) run(ctx context.Context) (*NegotiatedParams, error) {
    for {
        resp, err := l.sendLoginPDU(ctx)
        if err != nil {
            return nil, err
        }
        if resp.StatusClass != 0 {
            return nil, &LoginError{
                StatusClass:  resp.StatusClass,
                StatusDetail: resp.StatusDetail,
                Message:      statusMessage(resp.StatusClass, resp.StatusDetail),
            }
        }
        if resp.Transit && resp.NSG == 3 {
            // Reached Full Feature Phase
            return l.params, nil
        }
        // Process response keys, advance stage
        l.processResponse(resp)
    }
}
```

### Pattern 2: Declarative Key Registry (D-04)
**What:** Each negotiation key is described by a struct with its type, default, valid range, and RFC reference. The engine processes them uniformly.
**When to use:** All operational parameter negotiation.

**Negotiation types and their algorithms:**

| Type | Algorithm | Example Keys |
|------|-----------|-------------|
| ListSelect | Initiator sends ordered preference list; target selects first mutually acceptable item | HeaderDigest, DataDigest |
| BooleanAnd | Result = initiator_value AND target_value (both must say Yes for Yes) | ImmediateData |
| BooleanOr | Result = initiator_value OR target_value (either saying Yes yields Yes) | InitialR2T |
| NumericalMin | Result = min(initiator_value, target_value) | MaxBurstLength, FirstBurstLength, MaxConnections, DefaultTime2Wait, DefaultTime2Retain, MaxOutstandingR2T, ErrorRecoveryLevel |
| NumericalMax | Result = max(initiator_value, target_value) | (none in current mandatory set, but engine should support for completeness) |
| Declarative | Each side declares independently; not negotiated | MaxRecvDataSegmentLength (each side declares its own receive limit) |

**Example:**
```go
type NegotiationType int

const (
    ListSelect NegotiationType = iota
    BooleanAnd
    BooleanOr
    NumericalMin
    NumericalMax
    Declarative
)

type KeyDef struct {
    Name       string
    Type       NegotiationType
    Default    string
    ValidRange [2]uint32 // for numeric types; 0,0 means N/A
    RFCRef     string    // e.g., "RFC 7143 Section 13.1"
}
```

### Pattern 3: Text Key-Value Codec
**What:** iSCSI login data segments carry key=value pairs separated by null bytes (0x00).
**When to use:** Every login PDU data segment.

**Wire format:** `key1=value1\0key2=value2\0` -- each pair terminated by a null byte.

**Example:**
```go
// Encode key-value pairs into data segment
func EncodeTextKV(pairs map[string]string) []byte {
    var buf bytes.Buffer
    for k, v := range pairs {
        buf.WriteString(k)
        buf.WriteByte('=')
        buf.WriteString(v)
        buf.WriteByte(0)
    }
    return buf.Bytes()
}

// Decode data segment into key-value pairs
func DecodeTextKV(data []byte) map[string]string {
    pairs := make(map[string]string)
    for _, item := range bytes.Split(data, []byte{0}) {
        if len(item) == 0 {
            continue
        }
        k, v, ok := strings.Cut(string(item), "=")
        if ok {
            pairs[k] = v
        }
    }
    return pairs
}
```

**Important:** Use ordered slice of pairs (not map) for encoding to ensure deterministic wire order. Maps in Go have random iteration. The encoding function above is illustrative; implementation should use `[]KeyValue` pairs.

### Pattern 4: CHAP Authentication
**What:** Challenge-Handshake Authentication Protocol for iSCSI login.
**When to use:** When WithCHAP or WithMutualCHAP options are provided.

**One-way CHAP exchange (target authenticates initiator):**
```
Initiator -> Target:  AuthMethod=CHAP
Target -> Initiator:  AuthMethod=CHAP
Initiator -> Target:  CHAP_A=5           (algorithm: 5=MD5)
Target -> Initiator:  CHAP_A=5, CHAP_I=<id>, CHAP_C=0x<challenge_hex>
Initiator -> Target:  CHAP_N=<username>, CHAP_R=0x<response_hex>
Target -> Initiator:  (login success or failure)
```

**Mutual CHAP (bidirectional) -- initiator also authenticates target:**
```
Initiator -> Target:  AuthMethod=CHAP
Target -> Initiator:  AuthMethod=CHAP
Initiator -> Target:  CHAP_A=5
Target -> Initiator:  CHAP_A=5, CHAP_I=<tid>, CHAP_C=0x<tchallenge_hex>
Initiator -> Target:  CHAP_N=<iname>, CHAP_R=0x<iresp_hex>,
                       CHAP_I=<iid>, CHAP_C=0x<ichallenge_hex>
Target -> Initiator:  CHAP_N=<tname>, CHAP_R=0x<tresp_hex>
Initiator:            verify target's CHAP_R against expected
```

**MD5 hash computation:** `MD5(CHAP_I_byte || secret_bytes || CHAP_C_bytes)`

**Value encoding:**
- `CHAP_A`: decimal integer as string (e.g., `"5"` for MD5)
- `CHAP_I`: decimal integer as string (e.g., `"42"`)
- `CHAP_C`: hex-encoded with `0x` prefix (e.g., `"0x1a2b3c..."`)
- `CHAP_R`: hex-encoded with `0x` prefix (e.g., `"0xabcd..."`)
- `CHAP_N`: plain text username

### Pattern 5: Functional Options (D-01)
**What:** Login configuration via functional options.
**When to use:** `Login()` function API.

```go
type LoginOption func(*loginConfig)

type loginConfig struct {
    targetName  string
    chapUser    string
    chapSecret  string
    mutualCHAP  bool
    targetSecret string
    headerDigest []string // preference list, e.g., ["CRC32C", "None"]
    dataDigest   []string
    // ... more options
}

func WithTarget(iqn string) LoginOption {
    return func(c *loginConfig) { c.targetName = iqn }
}

func WithCHAP(user, secret string) LoginOption {
    return func(c *loginConfig) {
        c.chapUser = user
        c.chapSecret = secret
    }
}

func WithMutualCHAP(user, secret, targetSecret string) LoginOption {
    return func(c *loginConfig) {
        c.chapUser = user
        c.chapSecret = secret
        c.mutualCHAP = true
        c.targetSecret = targetSecret
    }
}
```

### Anti-Patterns to Avoid
- **String map for negotiated params:** D-05 explicitly requires typed struct fields. Do NOT use `map[string]string` for storing final negotiated values -- that defeats compile-time type safety.
- **Combined Dial+Login:** D-02 explicitly separates these. Do not create a convenience `Connect()` in this phase.
- **Using crypto/hmac for CHAP:** CHAP uses raw `MD5(id || secret || challenge)`, NOT HMAC-MD5. These are different algorithms.
- **Non-deterministic key-value encoding:** Use ordered pairs, not Go maps, when building text data segments.
- **Ignoring Continue bit:** Login PDUs can be split across multiple exchanges if the data segment exceeds MaxRecvDataSegmentLength. Handle the C (Continue) bit.

## RFC 7143 Section 13: Complete Mandatory Key Registry

| Key | Negotiation Type | Default | Valid Range | RFC Section |
|-----|-----------------|---------|-------------|-------------|
| HeaderDigest | ListSelect | None | None, CRC32C | 13.1 |
| DataDigest | ListSelect | None | None, CRC32C | 13.2 |
| MaxConnections | NumericalMin | 1 | 1-65535 | 13.3 |
| InitialR2T | BooleanOr | Yes | Yes, No | 13.10 |
| ImmediateData | BooleanAnd | Yes | Yes, No | 13.11 |
| MaxRecvDataSegmentLength | Declarative | 8192 | 512-16777215 | 13.12 |
| MaxBurstLength | NumericalMin | 262144 | 512-16777215 | 13.13 |
| FirstBurstLength | NumericalMin | 65536 | 512-16777215 | 13.14 |
| DefaultTime2Wait | NumericalMin | 2 | 0-3600 | 13.15 |
| DefaultTime2Retain | NumericalMin | 20 | 0-3600 | 13.16 |
| MaxOutstandingR2T | NumericalMin | 1 | 1-65535 | 13.17 |
| DataPDUInOrder | BooleanOr | Yes | Yes, No | 13.18 |
| DataSequenceInOrder | BooleanOr | Yes | Yes, No | 13.19 |
| ErrorRecoveryLevel | NumericalMin | 0 | 0-2 | 13.20 |

**Special notes:**
- **MaxRecvDataSegmentLength** is Declarative, not negotiated. Each side declares its own receive buffer limit independently. The initiator's declared value limits what the target sends, and vice versa.
- **InitialR2T = BooleanOr**: If EITHER side wants R2T required, R2T is required. This is the conservative direction.
- **ImmediateData = BooleanAnd**: BOTH sides must agree for immediate data to be enabled.
- **FirstBurstLength** must not exceed MaxBurstLength.
- **SessionType** key is also sent during login (`Normal` or `Discovery`) but is not negotiated per the same rules.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CRC32C computation | Custom CRC implementation | `hash/crc32` with `crc32.Castagnoli` (already in `internal/digest/`) | Hardware-accelerated, stdlib, already implemented |
| MD5 hashing | Custom MD5 | `crypto/md5` stdlib | Standard, well-tested, CHAP only needs MD5 |
| Hex encoding | Manual byte-to-hex | `encoding/hex` stdlib | `hex.EncodeToString()` and `hex.DecodeString()` handle CHAP binary values |
| Random challenge generation | Math/rand | `crypto/rand` | CHAP challenges MUST be cryptographically random per RFC 1994 |
| TCP connection handling | Custom TCP framing | Existing `transport.Conn`, `ReadRawPDU`, `WriteRawPDU` | Phase 1 already solved this |
| PDU routing | Custom dispatch | Existing `transport.Router` | ITT-based PDU correlation already works |

## Common Pitfalls

### Pitfall 1: CHAP Hash Byte Order
**What goes wrong:** Computing CHAP response with wrong byte concatenation order.
**Why it happens:** Confusing HMAC-MD5 with CHAP's raw MD5, or getting the concatenation order wrong.
**How to avoid:** CHAP response = `MD5(single_byte_id || secret_bytes || challenge_bytes)`. The ID is ONE byte (not a 4-byte int). The secret is the raw password bytes. The challenge is the decoded binary from the hex string.
**Warning signs:** Auth failure against known-good targets; response hex doesn't match expected values in test vectors.

### Pitfall 2: CHAP Value Encoding
**What goes wrong:** Sending CHAP_C/CHAP_R without `0x` prefix, or using base64 when target expects hex.
**Why it happens:** RFC 7143 says binary values can be hex (`0x` prefix) or base64 (`0b` prefix). Most implementations use hex.
**How to avoid:** Always use `0x` prefix hex encoding for CHAP_C and CHAP_R. Parse both formats when receiving (some targets may use `0b`).
**Warning signs:** Target returns CHAP auth failure; Wireshark shows correctly formatted PDU but wrong value encoding.

### Pitfall 3: Text Key-Value Encoding Edge Cases
**What goes wrong:** Missing trailing null byte, incorrect handling of multi-value keys (comma-separated lists), or failing to handle empty values.
**Why it happens:** The null-byte-separated format has subtle edge cases.
**How to avoid:** Each key=value pair MUST be followed by a null byte. Handle `AuthMethod=CHAP,None` as a comma-separated list. Handle `key=` (empty value) correctly. Handle multiple keys with the same name (some negotiations send the same key multiple times).
**Warning signs:** Target rejects login with "missing parameter" error (StatusClass=2, StatusDetail=0).

### Pitfall 4: MaxRecvDataSegmentLength is Declarative, Not Negotiated
**What goes wrong:** Applying min/max logic to MaxRecvDataSegmentLength like other numeric keys.
**Why it happens:** It looks like a numeric negotiation key, but it's actually each side independently declaring its own receive buffer size.
**How to avoid:** Initiator declares its MaxRecvDataSegmentLength (how much data it can receive per PDU). Target declares its own. Each side uses the peer's declared value as its send limit.
**Warning signs:** Data segments get truncated or target rejects PDUs as too large.

### Pitfall 5: Login PDU Does Not Use Full-Duplex Pumps
**What goes wrong:** Trying to use WritePump/ReadPump during login phase.
**Why it happens:** Login is a synchronous request-response exchange. The full-duplex pump infrastructure is for Full Feature Phase.
**How to avoid:** During login, do synchronous PDU writes directly to the TCP connection and reads via `ReadRawPDU`. Start the ReadPump/WritePump only after login succeeds and Full Feature Phase is entered.
**Warning signs:** Deadlocks during login, or login responses getting lost in the router.

### Pitfall 6: Digest Activation Timing
**What goes wrong:** Enabling digests before login completes, or not enabling them after negotiation.
**Why it happens:** Digests are negotiated during login but must only be active in Full Feature Phase.
**How to avoid:** Login PDUs themselves do NOT have digests (per RFC 7143, digests apply after login). Call `Conn.SetDigests()` only after login returns successfully, before starting the read/write pumps.
**Warning signs:** CRC errors on login PDUs, or missing digests on first Full Feature Phase PDU.

### Pitfall 7: ISID Must Be Consistent Within a Session
**What goes wrong:** Generating a new random ISID for every login attempt.
**Why it happens:** Not understanding that ISID identifies the session.
**How to avoid:** Generate ISID once per initiator instance. Reuse it for session reinstatement. For new sessions, TSIH=0. The target's TSIH in the login response must be preserved for later use.
**Warning signs:** Target treats reconnection as a new session instead of reinstatement.

### Pitfall 8: Mutual CHAP Secret Reuse
**What goes wrong:** Using the same secret for both directions of mutual CHAP.
**Why it happens:** Configuration convenience.
**How to avoid:** RFC 7143 Section 12 explicitly states: "any CHAP secret used for initiator authentication MUST NOT be configured for authentication of any target." Additionally, if the CHAP response from one end matches what the other end would have generated for the same challenge, the connection MUST be terminated.
**Warning signs:** Security vulnerability; some targets will reject the login.

### Pitfall 9: StatSN Tracking During Login
**What goes wrong:** Not properly tracking StatSN from login responses.
**Why it happens:** StatSN is usually a Full Feature Phase concern, but it starts during login.
**How to avoid:** The target's StatSN in each LoginResp must be tracked. The initiator's ExpStatSN in subsequent LoginReq PDUs must reflect the received StatSN + 1.
**Warning signs:** Target rejects login PDUs with sequence number errors.

### Pitfall 10: CmdSN During Login
**What goes wrong:** Incorrectly incrementing CmdSN during login.
**Why it happens:** Login requests are special -- the first login PDU of a new connection sets the initial CmdSN.
**How to avoid:** Set CmdSN in the first LoginReq. Do not increment CmdSN for subsequent login PDUs in the same login phase (login PDUs are not normal commands). CmdSN increments start in Full Feature Phase.
**Warning signs:** Target rejects login with command sequence errors.

## Code Examples

### CHAP Response Computation
```go
// Source: RFC 1994 Section 4.1 + RFC 7143 Section 12.1.3
func chapResponse(id byte, secret, challenge []byte) [16]byte {
    h := md5.New()
    h.Write([]byte{id})
    h.Write(secret)
    h.Write(challenge)
    var digest [16]byte
    copy(digest[:], h.Sum(nil))
    return digest
}
```

### CHAP Value Encoding/Decoding
```go
// Encode binary to hex with 0x prefix
func encodeCHAPBinary(data []byte) string {
    return "0x" + hex.EncodeToString(data)
}

// Decode hex (0x prefix) or base64 (0b prefix)
func decodeCHAPBinary(s string) ([]byte, error) {
    if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
        return hex.DecodeString(s[2:])
    }
    if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
        return base64.StdEncoding.DecodeString(s[2:])
    }
    return nil, fmt.Errorf("chap: unknown binary encoding prefix in %q", s)
}
```

### NegotiatedParams Struct (D-05)
```go
type NegotiatedParams struct {
    HeaderDigest           bool   // true = CRC32C, false = None
    DataDigest             bool   // true = CRC32C, false = None
    MaxConnections         uint32
    InitialR2T             bool
    ImmediateData          bool
    MaxRecvDataSegmentLength uint32 // peer's declared value (our send limit)
    MaxBurstLength         uint32
    FirstBurstLength       uint32
    DefaultTime2Wait       uint32
    DefaultTime2Retain     uint32
    MaxOutstandingR2T      uint32
    DataPDUInOrder         bool
    DataSequenceInOrder    bool
    ErrorRecoveryLevel     uint32
    TargetName             string // from TargetName key
    TSIH                   uint16 // assigned by target
}

// Defaults returns NegotiatedParams with RFC 7143 default values.
func Defaults() NegotiatedParams {
    return NegotiatedParams{
        HeaderDigest:           false,
        DataDigest:             false,
        MaxConnections:         1,
        InitialR2T:             true,
        ImmediateData:          true,
        MaxRecvDataSegmentLength: 8192,
        MaxBurstLength:         262144,
        FirstBurstLength:       65536,
        DefaultTime2Wait:       2,
        DefaultTime2Retain:     20,
        MaxOutstandingR2T:      1,
        DataPDUInOrder:         true,
        DataSequenceInOrder:    true,
        ErrorRecoveryLevel:     0,
    }
}
```

### LoginError (D-08)
```go
type LoginError struct {
    StatusClass  uint8
    StatusDetail uint8
    Message      string
}

func (e *LoginError) Error() string {
    return fmt.Sprintf("iscsi login: class=%d detail=%d: %s",
        e.StatusClass, e.StatusDetail, e.Message)
}

// Status code constants (RFC 7143 Section 11.13.5)
const (
    StatusSuccess              = 0x0000 // class=0, detail=0
    StatusRedirectTemp         = 0x0101
    StatusRedirectPerm         = 0x0102
    StatusInitiatorError       = 0x0200
    StatusAuthFailure          = 0x0201
    StatusForbidden            = 0x0202
    StatusTargetNotFound       = 0x0203
    StatusTargetRemoved        = 0x0204
    StatusTargetError          = 0x0300
    StatusServiceUnavailable   = 0x0301
    StatusOutOfResources       = 0x0302
)
```

### Mock Target for CHAP Tests
```go
// mockTarget simulates an iSCSI target for login testing.
// Handles login PDU exchange and responds appropriately.
func mockTarget(t *testing.T, ln net.Listener, cfg mockTargetConfig) {
    t.Helper()
    conn, err := ln.Accept()
    if err != nil {
        t.Errorf("accept: %v", err)
        return
    }
    defer conn.Close()

    for {
        raw, err := transport.ReadRawPDU(conn, false, false)
        if err != nil {
            return
        }
        loginReq := &pdu.LoginReq{}
        loginReq.UnmarshalBHS(raw.BHS)
        kvs := DecodeTextKV(raw.DataSegment)

        // Process based on current stage and keys...
        resp := buildLoginResponse(loginReq, kvs, cfg)
        transport.WriteRawPDU(conn, resp)

        if resp is final (Transit=true, NSG=3) {
            return
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CHAP with MD5 only | MD5, SHA1, SHA256, SHA3-256 | RFC 7143 allows extensibility via CHAP_A | For v1, MD5 (CHAP_A=5) is sufficient and universally supported. SHA variants are optional. |
| iSCSI draft RFCs (3720, 5048) | RFC 7143 (consolidated, April 2014) | 2014 | RFC 7143 is the authoritative spec. No need to reference older RFCs. |
| `GOEXPERIMENT=synctest` | `testing/synctest` (stdlib) | Go 1.25 (August 2025) | No build tag needed. Available directly for testing. |

## Open Questions

1. **gotgt CHAP support**
   - What we know: gotgt only supports `AuthMethod=None`. It has an `AuthChap` constant but no implementation.
   - What's unclear: Whether gotgt can be extended with CHAP support for integration tests.
   - Recommendation: Use a mock iSCSI target for CHAP testing. Test `AuthMethod=None` against gotgt for integration tests. The mock target approach is more reliable and covers more edge cases anyway.

2. **gotgt digest negotiation**
   - What we know: gotgt defaults HeaderDigest=None, DataDigest=None.
   - What's unclear: Whether gotgt can negotiate CRC32C digests.
   - Recommendation: Test digest negotiation and verification via mock target. For integration tests where gotgt is the target, test the None path.

3. **Login PDU Continue bit**
   - What we know: RFC 7143 allows splitting login data across multiple PDUs using the C (Continue) bit.
   - What's unclear: Whether any real target sends continued login responses.
   - Recommendation: Implement Continue bit support for robustness, but prioritize testing the common single-PDU path. Add a test that exercises Continue bit with the mock target.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | All code | Yes | go1.25.5 netbsd/amd64 | -- |
| `crypto/md5` | CHAP auth | Yes | stdlib | -- |
| `encoding/hex` | CHAP encoding | Yes | stdlib | -- |
| `crypto/rand` | CHAP challenge gen | Yes | stdlib | -- |
| `testing/synctest` | Concurrent tests | Yes | Go 1.25 stdlib | -- |
| `gostor/gotgt` | Integration tests | Not yet imported | HEAD | Mock target for all login tests |

**Missing dependencies with no fallback:** None

**Missing dependencies with fallback:**
- gotgt not yet imported, but mock targets handle all login test scenarios. gotgt integration can be added in a future phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (go1.25.5) |
| Config file | None needed -- `go test` with default settings |
| Quick run command | `go test ./internal/login/ -v -count=1` |
| Full suite command | `go test ./... -race -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LOGIN-01 | Login state machine transitions (CSG 0->1->3) | unit | `go test ./internal/login/ -run TestLoginStateMachine -v` | Wave 0 |
| LOGIN-02 | Text key negotiation engine | unit | `go test ./internal/login/ -run TestNegotiation -v` | Wave 0 |
| LOGIN-03 | AuthMethod=None login | unit+integration | `go test ./internal/login/ -run TestLoginAuthNone -v` | Wave 0 |
| LOGIN-04 | One-way CHAP | unit | `go test ./internal/login/ -run TestLoginCHAP -v` | Wave 0 |
| LOGIN-05 | Mutual CHAP | unit | `go test ./internal/login/ -run TestLoginMutualCHAP -v` | Wave 0 |
| LOGIN-06 | Operational parameter negotiation | unit | `go test ./internal/login/ -run TestNegotiateParams -v` | Wave 0 |
| INTEG-01 | Header digest negotiation + verification | unit | `go test ./internal/login/ -run TestHeaderDigest -v` | Wave 0 |
| INTEG-02 | Data digest negotiation + verification | unit | `go test ./internal/login/ -run TestDataDigest -v` | Wave 0 |
| INTEG-03 | Digest generation on outgoing PDUs | unit | `go test ./internal/login/ -run TestDigestGeneration -v` | Wave 0 |
| TEST-04 | Parameterized negotiation matrix tests | unit | `go test ./internal/login/ -run TestNegotiationMatrix -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/login/ -race -count=1`
- **Per wave merge:** `go test ./... -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/login/` directory -- does not exist yet
- [ ] `internal/login/textcodec_test.go` -- text key-value codec tests
- [ ] `internal/login/negotiation_test.go` -- parameterized negotiation tests (TEST-04)
- [ ] `internal/login/chap_test.go` -- CHAP unit tests with known test vectors
- [ ] `internal/login/login_test.go` -- integration tests with mock target

## Sources

### Primary (HIGH confidence)
- RFC 7143 (rfc-editor.org/rfc/rfc7143.html) -- Login PDU format (Sections 11.12, 11.13), login state machine (Section 6), negotiation keys (Section 13), CHAP (Section 12.1.3)
- RFC 1994 (datatracker.ietf.org/doc/html/rfc1994) -- CHAP algorithm: MD5(id || secret || challenge)
- Existing codebase -- LoginReq/LoginResp PDU types, transport.Conn, digest.HeaderDigest/DataDigest, Router/ReadPump/WritePump

### Secondary (MEDIUM confidence)
- libiscsi login.c (github.com/sahlberg/libiscsi) -- CHAP encoding format verified: hex with 0x prefix, CHAP_A=5 for MD5, CHAP_I as decimal
- scst chap.c (github.com/dmeister/scst) -- Confirmed CHAP value encoding (hex format 0x, base64 format 0b)
- gostor/gotgt login.go (github.com/gostor/gotgt) -- Confirmed: NO CHAP support, AuthMethod=None only

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, no version concerns
- Architecture: HIGH -- RFC 7143 is well-defined, existing codebase patterns established, CONTEXT.md decisions clear
- Pitfalls: HIGH -- verified against libiscsi/scst implementations and RFC text
- CHAP exchange sequence: HIGH -- verified across RFC 1994, RFC 7143, libiscsi, and scst sources
- gotgt limitations: MEDIUM -- verified from source code, but gotgt may add CHAP support in future

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable domain -- RFC hasn't changed since 2014)
