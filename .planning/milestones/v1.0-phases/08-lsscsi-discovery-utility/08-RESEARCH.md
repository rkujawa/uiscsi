# Phase 08: lsscsi-discovery-utility - Research

**Researched:** 2026-04-02
**Domain:** Go CLI tool, iSCSI discovery, columnar text output
**Confidence:** HIGH

## Summary

Phase 08 builds `uiscsi-ls`, a standalone CLI tool in its own Go module that imports the `github.com/rkujawa/uiscsi` library. The tool performs iSCSI SendTargets discovery against one or more portals, connects to each discovered target, probes every LUN (ReportLuns, Inquiry, ReadCapacity), and presents the results in an `lsscsi`-style columnar format or JSON.

The public API already provides everything needed: `Discover()`, `Dial()`, `Session.ReportLuns()`, `Session.Inquiry()`, `Session.ReadCapacity()`, and `Session.Close()`. The existing `examples/discover-read/main.go` demonstrates the exact flow. The CLI layer is straightforward Go: parse flags, call library, format output, handle errors. No new protocol code is needed.

**Primary recommendation:** Use stdlib `flag` for argument parsing, `text/tabwriter` for columnar alignment, and `encoding/json` for JSON output. Keep the tool in a separate `uiscsi-ls/` directory at repo root with its own `go.mod`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Default output is fixed-width columnar (lsscsi-style), one line per LUN showing: target IQN, portal, LUN number, device type, vendor, product, revision, capacity
- **D-02:** `--json` flag switches output to machine-parseable JSON for scripting
- **D-03:** Portal address specified via `--portal` flag (not positional argument)
- **D-04:** Multiple portals supported by repeating the flag: `--portal 10.0.0.1:3260 --portal 10.0.0.2:3260`
- **D-05:** CHAP authentication via `--chap-user` and `--chap-secret` flags with environment variable fallback (`ISCSI_CHAP_USER`, `ISCSI_CHAP_SECRET`). Flags take precedence over env vars.
- **D-06:** Default iSCSI port 3260 if port omitted from portal address
- **D-07:** Always performs full probe: SendTargets discovery -> connect to each target -> ReportLuns -> Inquiry + ReadCapacity per LUN
- **D-08:** No `--discover-only` flag -- keep the tool simple, always full probe
- **D-09:** Separate Go module (`uiscsi-ls`), imports `github.com/rkujawa/uiscsi` as external dependency
- **D-10:** Binary name: `uiscsi-ls`

### Claude's Discretion
- Column widths and alignment strategy for columnar output
- JSON structure (flat vs nested)
- Error handling for unreachable targets during multi-portal scan (skip and report vs fail fast)
- Exit codes
- Flag parsing library (stdlib `flag` vs other)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `flag` | stdlib | CLI argument parsing | D-03/D-04/D-05 require `--portal`, `--chap-user`, `--chap-secret`, `--json` flags. stdlib `flag` handles repeated flags via custom `flag.Value` (string slice). No external dependency needed for this simple CLI. |
| `text/tabwriter` | stdlib | Fixed-width columnar output | D-01 requires lsscsi-style aligned columns. `tabwriter` handles elastic tab stops, auto-sizing columns to content. Used by `go help`, `docker ps`, etc. |
| `encoding/json` | stdlib | JSON output mode | D-02 requires `--json` flag for machine-parseable output. `json.NewEncoder` with `SetIndent` for pretty-printing. |
| `fmt` | stdlib | Formatted output and capacity formatting | String formatting for columnar fields, human-readable capacity (GB/TB). |
| `os` | stdlib | Environment variable fallback, exit codes | D-05 requires `ISCSI_CHAP_USER`/`ISCSI_CHAP_SECRET` env var fallback. |
| `net` | stdlib | Portal address normalization | D-06 requires defaulting port to 3260. `net.SplitHostPort` / `net.JoinHostPort` for address parsing. |
| `github.com/rkujawa/uiscsi` | local | iSCSI library | The whole point -- Discover, Dial, ReportLuns, Inquiry, ReadCapacity. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `flag` | `pflag` (spf13/pflag) | pflag gives GNU-style `--long` flags and native string-slice support. However, stdlib `flag` already supports `--long` syntax and repeated flags via custom `flag.Value`. Adding pflag means an external dependency in a tool that should be minimal. Use stdlib. |
| `text/tabwriter` | Manual `fmt.Sprintf` with fixed widths | Manual widths break when content exceeds expected lengths. `tabwriter` auto-sizes. Use tabwriter. |
| `text/tabwriter` | `tablewriter` (olekukonern/tablewriter) | External dependency for ASCII box-drawing tables. lsscsi uses plain space-separated columns, not boxes. Use tabwriter. |

## Architecture Patterns

### Recommended Project Structure
```
uiscsi-ls/
├── go.mod              # module github.com/rkujawa/uiscsi-ls
├── go.sum
├── main.go             # flag parsing, orchestration, exit codes
├── probe.go            # discovery + per-target LUN probing logic
├── format.go           # columnar and JSON formatters
├── format_test.go      # formatter unit tests
├── probe_test.go       # probe logic tests (mock-able via interface)
└── device_type.go      # SCSI peripheral device type name table
```

### Pattern 1: Repeated Flag via flag.Value Interface
**What:** stdlib `flag` supports repeated flags by implementing `flag.Value` on a string slice.
**When to use:** D-04 requires `--portal` to be specified multiple times.
**Example:**
```go
// Source: stdlib flag package pattern
type stringSlice []string

func (s *stringSlice) String() string { return fmt.Sprintf("%v", *s) }
func (s *stringSlice) Set(v string) error {
    *s = append(*s, v)
    return nil
}

var portals stringSlice
flag.Var(&portals, "portal", "iSCSI target portal address (repeatable)")
```

### Pattern 2: Portal Address Normalization
**What:** Parse portal address, default port to 3260 if missing per D-06.
**When to use:** Every portal address from `--portal` flag.
**Example:**
```go
func normalizePortal(addr string) string {
    host, port, err := net.SplitHostPort(addr)
    if err != nil {
        // No port specified -- assume bare host or IP.
        return net.JoinHostPort(addr, "3260")
    }
    if port == "" {
        port = "3260"
    }
    return net.JoinHostPort(host, port)
}
```

### Pattern 3: Probe Flow (D-07)
**What:** Full discovery-to-LUN-detail pipeline using the uiscsi public API.
**When to use:** The core business logic of the tool.
**Example:**
```go
// For each portal:
targets, err := uiscsi.Discover(ctx, portal, opts...)
// For each target:
sess, err := uiscsi.Dial(ctx, portal, uiscsi.WithTarget(t.Name), opts...)
luns, err := sess.ReportLuns(ctx)
// For each LUN:
inq, err := sess.Inquiry(ctx, lun)
cap, err := sess.ReadCapacity(ctx, lun)
sess.Close()
```

### Pattern 4: lsscsi-Style Columnar Output
**What:** Fixed-width columns matching lsscsi aesthetic: device type, vendor (8 chars padded), product (16 chars padded), revision (4 chars), capacity.
**When to use:** Default (non-JSON) output mode per D-01.

Real `lsscsi -s` output for reference:
```
[0:0:0:0]    disk    ATA      WDC WD40EZRZ-00G 0A80  /dev/sda   4.00TB
[6:0:0:0]    cd/dvd  TSSTcorp CDDVDW SH-222AB  SB00  /dev/sr0   1.07GB
```

For `uiscsi-ls`, the "HCTL" tuple and device node are meaningless (no kernel). Replace with target IQN (shortened) + portal + LUN:
```
iqn.2026-03.com.example:storage  10.0.0.1:3260  LUN 0  disk    VENDOR   PRODUCT          REV   100.0GB
iqn.2026-03.com.example:storage  10.0.0.1:3260  LUN 1  disk    VENDOR   PRODUCT2         REV   500.0GB
```

Use `text/tabwriter` with tab-separated fields for automatic alignment.

### Pattern 5: CHAP Credential Resolution (D-05)
**What:** Flags take precedence over environment variables.
**When to use:** Resolving CHAP credentials before calling library.
**Example:**
```go
func resolveCHAP(flagUser, flagSecret string) (string, string) {
    user := flagUser
    if user == "" {
        user = os.Getenv("ISCSI_CHAP_USER")
    }
    secret := flagSecret
    if secret == "" {
        secret = os.Getenv("ISCSI_CHAP_SECRET")
    }
    return user, secret
}
```

### Pattern 6: JSON Output Structure (Claude's Discretion)
**Recommendation:** Nested structure grouping LUNs under targets, targets under portals. This reflects the natural hierarchy and is more useful for scripting than flat arrays.
```json
{
  "portals": [
    {
      "address": "10.0.0.1:3260",
      "targets": [
        {
          "iqn": "iqn.2026-03.com.example:storage",
          "luns": [
            {
              "lun": 0,
              "device_type": "disk",
              "vendor": "VENDOR",
              "product": "PRODUCT",
              "revision": "REV",
              "capacity_bytes": 107374182400,
              "capacity_blocks": 209715200,
              "block_size": 512
            }
          ]
        }
      ]
    }
  ]
}
```

### Pattern 7: Error Handling for Multi-Portal (Claude's Discretion)
**Recommendation:** Skip-and-report. When scanning multiple portals, one unreachable portal should not abort the entire scan. Print errors to stderr, continue with next portal/target/LUN. Exit code 0 if at least one portal succeeded, non-zero if all failed.

### Pattern 8: Exit Codes (Claude's Discretion)
**Recommendation:**
| Code | Meaning |
|------|---------|
| 0 | Success (at least one LUN discovered) |
| 1 | Usage error (no portals specified, invalid flags) |
| 2 | All portals failed (no results at all) |

### Anti-Patterns to Avoid
- **Importing internal packages:** The CLI is a separate module. It MUST use only the public API (`uiscsi.Discover`, `uiscsi.Dial`, `uiscsi.WithTarget`, etc.). No imports from `github.com/rkujawa/uiscsi/internal/...`.
- **Hardcoded column widths:** Use `tabwriter` instead. Hardcoded widths break with long IQNs or product names.
- **Printing secrets:** Never log or print CHAP credentials. The `--chap-secret` flag value and `ISCSI_CHAP_SECRET` env var must never appear in output or error messages.
- **Ignoring context cancellation:** All library calls accept `context.Context`. Wire up signal handling (SIGINT/SIGTERM) to context cancellation so Ctrl+C cleanly aborts in-flight operations.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Column alignment | Manual space-padding with `fmt.Sprintf("%-8s")` | `text/tabwriter` | Handles variable-width content, elastic tab stops, battle-tested |
| Flag parsing with repeated values | Custom argument parser | `flag.Var` with `flag.Value` interface | stdlib pattern, well-understood, zero dependencies |
| Address parsing/normalization | Regex or string splitting on ":" | `net.SplitHostPort` / `net.JoinHostPort` | Handles IPv6 addresses in brackets, edge cases |
| JSON encoding | Manual string concatenation | `encoding/json.Marshal` | Handles escaping, nested structures, proper formatting |
| Human-readable capacity | Custom rounding logic | Simple division with `fmt.Sprintf("%.1fGB")` | Two lines of code, no library needed |

## SCSI Peripheral Device Type Names

The CLI needs to display device type names (D-01). Map the 5-bit peripheral device type from INQUIRY to a short string matching lsscsi conventions:

| Code | lsscsi Name | SPC Description |
|------|-------------|-----------------|
| 0x00 | disk | Direct access block device |
| 0x01 | tape | Sequential access device |
| 0x02 | printer | Printer device |
| 0x03 | processor | Processor device |
| 0x04 | worm | Write-once device |
| 0x05 | cd/dvd | CD/DVD device |
| 0x06 | scanner | Scanner device |
| 0x07 | optical | Optical memory device |
| 0x08 | medchgr | Media changer |
| 0x09 | comms | Communications device |
| 0x0C | storage | Storage array controller |
| 0x0D | enclosu | Enclosure services |
| 0x0E | disk | Simplified direct access |
| 0x0F | osd | Object-based storage |
| 0x11 | osd | Object-based storage (OSD-2) |
| 0x1E | wlun | Well-known logical unit |
| 0x1F | unknown | Unknown or no device type |

**Source:** T10 SPC-4 peripheral device type definitions.

## Human-Readable Capacity Formatting

Convert `Capacity.LogicalBlocks * Capacity.BlockSize` to human-readable:

```go
func formatCapacity(blocks uint64, blockSize uint32) string {
    bytes := blocks * uint64(blockSize)
    switch {
    case bytes >= 1e12:
        return fmt.Sprintf("%.2fTB", float64(bytes)/1e12)
    case bytes >= 1e9:
        return fmt.Sprintf("%.2fGB", float64(bytes)/1e9)
    case bytes >= 1e6:
        return fmt.Sprintf("%.2fMB", float64(bytes)/1e6)
    default:
        return fmt.Sprintf("%dB", bytes)
    }
}
```

Use decimal (SI) units like lsscsi does (TB = 10^12, GB = 10^9).

## Common Pitfalls

### Pitfall 1: IPv6 Portal Addresses
**What goes wrong:** Naive string splitting on ":" breaks for IPv6 addresses like `[::1]:3260`.
**Why it happens:** Developers assume addresses are always `host:port`.
**How to avoid:** Always use `net.SplitHostPort` / `net.JoinHostPort` which handle IPv6 bracket syntax.
**Warning signs:** Test with IPv6 loopback `[::1]:3260`.

### Pitfall 2: ReportLuns Returning LUN 0 Implicitly
**What goes wrong:** Some targets do not include LUN 0 in ReportLuns but still respond to INQUIRY on LUN 0.
**Why it happens:** Well-known LUN (0) behavior varies by target implementation.
**How to avoid:** Use only the LUNs returned by ReportLuns. Do not assume LUN 0 exists.
**Warning signs:** Empty LUN list from a target that should have LUNs.

### Pitfall 3: ReadCapacity Failing on Non-Disk Devices
**What goes wrong:** ReadCapacity is only valid for direct-access (disk) devices. Sending it to a tape, printer, or other device type returns CHECK CONDITION.
**Why it happens:** ReadCapacity is defined in SBC (block commands), not SPC (primary commands).
**How to avoid:** Check `InquiryData.DeviceType` before calling ReadCapacity. Only call it for device types 0x00 (disk), 0x0E (simplified direct access). For other types, display capacity as "-" (like lsscsi does for tapes).
**Warning signs:** SCSIError on ReadCapacity for non-disk LUNs.

### Pitfall 4: CHAP Secrets in Process Listing
**What goes wrong:** `--chap-secret mysecret` is visible in `ps aux` to all users.
**Why it happens:** Command-line arguments are visible in `/proc/pid/cmdline`.
**How to avoid:** Document in usage that env vars (`ISCSI_CHAP_SECRET`) are preferred for security. The tool cannot prevent this for flag-based input, but the env var fallback (D-05) provides a secure alternative.
**Warning signs:** Security audit flags.

### Pitfall 5: Separate Module go.mod Dependency Path
**What goes wrong:** `go.mod` in `uiscsi-ls/` requires `github.com/rkujawa/uiscsi` but during development the module is not published.
**Why it happens:** The separate module cannot import a local unpublished module without a `replace` directive.
**How to avoid:** Use `replace github.com/rkujawa/uiscsi => ../` in `go.mod` during development. Remove it before publishing. Document this in a comment.
**Warning signs:** `go build` fails with "module not found".

### Pitfall 6: Timeout on Unreachable Portals
**What goes wrong:** Default TCP timeout can be 30+ seconds per portal. Multi-portal scans hang.
**Why it happens:** No explicit timeout set for discovery/dial operations.
**How to avoid:** Create a context with timeout (e.g., 10 seconds per portal) using `context.WithTimeout`. Consider a `--timeout` flag as a future enhancement, but for now a reasonable hardcoded default works.
**Warning signs:** Tool appears frozen when pointed at unreachable portal.

## Code Examples

### Complete Main Flow Structure
```go
// Source: derived from examples/discover-read/main.go + CONTEXT.md decisions
func main() {
    var portals stringSlice
    flag.Var(&portals, "portal", "iSCSI target portal (repeatable)")
    chapUser := flag.String("chap-user", "", "CHAP username")
    chapSecret := flag.String("chap-secret", "", "CHAP secret")
    jsonOutput := flag.Bool("json", false, "output as JSON")
    flag.Parse()

    if len(portals) == 0 {
        fmt.Fprintf(os.Stderr, "error: at least one --portal required\n")
        os.Exit(1)
    }

    user, secret := resolveCHAP(*chapUser, *chapSecret)
    var opts []uiscsi.Option
    if user != "" && secret != "" {
        opts = append(opts, uiscsi.WithCHAP(user, secret))
    }

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    results := probeAll(ctx, portals, opts)

    if *jsonOutput {
        outputJSON(os.Stdout, results)
    } else {
        outputColumnar(os.Stdout, results)
    }
}
```

### Columnar Output with tabwriter
```go
// Source: stdlib text/tabwriter documentation
func outputColumnar(w io.Writer, results []PortalResult) {
    tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
    for _, pr := range results {
        for _, tr := range pr.Targets {
            for _, lr := range tr.LUNs {
                fmt.Fprintf(tw, "%s\t%s\tLUN %d\t%s\t%-8s\t%-16s\t%-4s\t%s\n",
                    tr.IQN, pr.Portal, lr.LUN,
                    deviceTypeName(lr.DeviceType),
                    lr.Vendor, lr.Product, lr.Revision,
                    lr.CapacityStr)
            }
        }
    }
    tw.Flush()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `os/signal.Notify` + manual channel | `signal.NotifyContext` (Go 1.16+) | Go 1.16 | Cleaner context-based signal handling |
| `flag.String` for repeated values | `flag.Var` with `flag.Value` interface | Always available | Supports `--portal` repeated flag pattern |

## Open Questions

1. **Module path for uiscsi-ls**
   - What we know: D-09 says separate Go module, D-10 says binary name `uiscsi-ls`
   - What's unclear: Whether the module should be `github.com/rkujawa/uiscsi-ls` (separate repo) or `github.com/rkujawa/uiscsi/cmd/uiscsi-ls` (subdirectory with own go.mod)
   - Recommendation: Use `uiscsi-ls/` directory at repo root with `module github.com/rkujawa/uiscsi-ls` and a `replace` directive for local development. This keeps it in the same repo but as a truly separate module per D-09. The directory is at repo root, not under `cmd/`, because it has its own `go.mod`.

2. **Timeout for per-portal operations**
   - What we know: Need reasonable timeouts to avoid hanging on unreachable portals
   - What's unclear: Whether to add a `--timeout` flag or use a hardcoded default
   - Recommendation: Use a hardcoded 10-second timeout per portal for now. Adding `--timeout` flag can be a follow-up if needed.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Compilation | Yes | 1.25.8 | -- |
| github.com/rkujawa/uiscsi | Import | Yes (local) | HEAD | -- |

No external runtime dependencies. The tool is pure Go stdlib + uiscsi library.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.25) |
| Config file | None needed -- `go test ./...` |
| Quick run command | `cd uiscsi-ls && go test ./...` |
| Full suite command | `cd uiscsi-ls && go test -race ./...` |

### Phase Requirements to Test Map

Since no formal requirement IDs are mapped to this phase yet, tests map to CONTEXT.md decisions:

| Decision | Behavior | Test Type | Automated Command | File Exists? |
|----------|----------|-----------|-------------------|-------------|
| D-01 | Columnar output format matches lsscsi style | unit | `go test -run TestColumnarOutput` | Wave 0 |
| D-02 | JSON output is valid and complete | unit | `go test -run TestJSONOutput` | Wave 0 |
| D-03/D-04 | Portal flag parsing, repeated flags | unit | `go test -run TestPortalFlags` | Wave 0 |
| D-05 | CHAP credential resolution (flag > env) | unit | `go test -run TestCHAPResolution` | Wave 0 |
| D-06 | Default port 3260 | unit | `go test -run TestNormalizePortal` | Wave 0 |
| D-07 | Full probe pipeline | integration | `go test -run TestProbeAll -tags integration` | Wave 0 |

### Sampling Rate
- **Per task commit:** `cd uiscsi-ls && go test ./...`
- **Per wave merge:** `cd uiscsi-ls && go test -race ./...`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `uiscsi-ls/format_test.go` -- columnar and JSON output tests (D-01, D-02)
- [ ] `uiscsi-ls/probe_test.go` -- probe logic tests with mocked library calls
- [ ] `uiscsi-ls/main_test.go` -- flag parsing and CHAP resolution tests (D-03, D-04, D-05, D-06)

## Project Constraints (from CLAUDE.md)

- **Language:** Go 1.25 -- verified available (1.25.8)
- **Dependencies:** Minimal external dependencies (Bronx Method). The CLI should have zero external deps beyond `github.com/rkujawa/uiscsi`. stdlib only for the CLI itself.
- **Testing:** stdlib `testing` package with table-driven tests. No testify.
- **API style:** Go idiomatic -- context.Context, structured errors, functional options
- **No testify, no protobuf, no third-party logging** -- all per CLAUDE.md "What NOT to Use"

## Sources

### Primary (HIGH confidence)
- `uiscsi.go`, `session.go`, `options.go`, `types.go`, `errors.go` -- public API review, all methods and types documented
- `examples/discover-read/main.go` -- existing discovery flow pattern
- `go.mod` -- Go 1.25 module, zero external dependencies
- Local `lsscsi` output -- verified columnar format on development machine
- Go stdlib `text/tabwriter`, `flag`, `encoding/json`, `net` package documentation

### Secondary (MEDIUM confidence)
- [T10 SCSI Common Codes](https://t10.org/lists/1spc-lst.htm) -- peripheral device type code table

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, no version concerns, verified Go 1.25 available
- Architecture: HIGH -- straightforward CLI pattern, public API well-understood from source review
- Pitfalls: HIGH -- based on direct API analysis and real lsscsi output inspection

**Research date:** 2026-04-02
**Valid until:** 2026-05-02 (stable domain, no fast-moving dependencies)
