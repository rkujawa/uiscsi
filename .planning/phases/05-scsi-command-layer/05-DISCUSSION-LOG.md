# Phase 5: SCSI Command Layer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-01
**Phase:** 05-scsi-command-layer
**Areas discussed:** Package structure, CDB builder API style, Response parsing depth, Session integration

---

## Package Structure

| Option | Description | Selected |
|--------|-------------|----------|
| New internal/scsi/ package | Clean separation from iSCSI transport. Mirrors libiscsi. | ✓ |
| Extend internal/session/ | Commands as methods on Session directly. Simpler import. | |
| Public scsi/ package | Top-level, external consumers can use CDB builders independently. | |

**User's choice:** New internal/scsi/ package
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| By command group | inquiry.go, readwrite.go, capacity.go, sense.go, etc. ~7-8 files. | ✓ |
| One file per command | 19+ tiny files. Maximum granularity. | |
| Core vs extended split | core.go + extended.go + sense.go + vpd.go. Just 4 files. | |

**User's choice:** By command group
**Notes:** None

---

## CDB Builder API Style

| Option | Description | Selected |
|--------|-------------|----------|
| Plain functions returning Command | scsi.Read10(lba, blocks). Simple, discoverable, Go-idiomatic. | ✓ |
| Struct literals with Build() | Callers fill typed struct, call .Build(). More verbose. | |
| Method chain builder | Fluent API. Less idiomatic in Go. | |

**User's choice:** Plain functions returning Command
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Functional options | scsi.Read10(lba, blocks, scsi.WithFUA()). Consistent with Session pattern. | ✓ |
| Bitfield flags argument | scsi.Read10(lba, blocks, scsi.FUA\|scsi.DPO). Compact but less discoverable. | |

**User's choice:** Functional options
**Notes:** None

---

## Response Parsing Depth

| Option | Description | Selected |
|--------|-------------|----------|
| Commonly used fields + raw access | Parse fields callers use, expose Raw for niche fields. 95% coverage. | ✓ |
| Full SPC-4 field coverage | Every defined field parsed. Comprehensive but more code. | |
| Minimal + raw bytes | Only device type and vendor/product. Everything else raw. | |

**User's choice:** Commonly used fields + raw access
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Typed enum + human-readable | SenseKey typed constant, ASC/ASCQ + String() method, Is() helper. | ✓ |
| Raw fields only | Parse format, expose raw key/asc/ascq as uint8. No strings. | |

**User's choice:** Typed enum + human-readable
**Notes:** None

---

## Session Integration

| Option | Description | Selected |
|--------|-------------|----------|
| Standalone scsi functions only | scsi/ builds CDBs, caller composes with sess.Submit(). Zero dependency on Session. | ✓ |
| Both: standalone + Session convenience | Pure CDB builders AND Session convenience methods. More ergonomic. | |
| Session methods only | All SCSI commands as Session methods. Tight coupling. | |

**User's choice:** Standalone scsi functions only
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Take Result directly | scsi.ParseInquiry(result) handles reading, status check, and parsing. | ✓ |
| Take raw []byte | Pure parser, caller reads result.Data themselves. | |

**User's choice:** Take Result directly
**Notes:** None

---

## Claude's Discretion

- ASC/ASCQ string lookup table coverage
- Internal helper patterns for CDB byte packing
- Test fixture organization
- Whether to export the functional option type

## Deferred Ideas

None
