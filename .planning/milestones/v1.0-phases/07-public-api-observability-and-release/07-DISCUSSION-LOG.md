# Phase 7: Public API, Observability, and Release - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-01
**Phase:** 07-public-api-observability-and-release
**Areas discussed:** Public API package structure, High-level vs low-level API boundary, E2E test infrastructure approach, Documentation and examples scope

---

## Public API Package Structure

### Q1: Where should the public API package live?

| Option | Description | Selected |
|--------|-------------|----------|
| Top-level uiscsi package | Root package exports types/functions directly. `uiscsi.Dial()`, `uiscsi.ReadBlocks()`. | ✓ |
| Separate iscsi sub-package | Under `github.com/rkujawa/uiscsi/iscsi`. Adds one import segment. | |
| Re-export internal types | Move types from internal/ to public. Exposes implementation. | |

**User's choice:** Top-level uiscsi package

### Q2: How should internal types surface in the public API?

| Option | Description | Selected |
|--------|-------------|----------|
| Public wrapper types | New public types wrapping internal types. Full API control. | ✓ |
| Type aliases to internal | `type Session = session.Session`. Less code but fragile. | |
| Minimal facade — functions only | No exported structs, only functions. Maximum encapsulation. | |

**User's choice:** Public wrapper types

### Q3: Should discovery be separate from session creation?

| Option | Description | Selected |
|--------|-------------|----------|
| Two-step: Discover then Connect | Separate Discover() and Dial() functions. Mirrors RFC model. | ✓ |
| One-step: Dial handles everything | Single Dial() that connects and logs in. | |
| Builder pattern | Chainable builder. More verbose but explicit. | |

**User's choice:** Two-step: Discover then Connect

---

## High-Level vs Low-Level API Boundary

### Q1: How should block I/O work at the high-level API?

| Option | Description | Selected |
|--------|-------------|----------|
| []byte in/out | ReadBlocks returns []byte, WriteBlocks takes []byte. Simple. | |
| io.Reader/io.Writer streaming | Enables large transfers without buffering. | |
| Both: []byte default, streaming optional | Primary uses []byte. Separate Stream functions for large transfers. | ✓ |

**User's choice:** Both: []byte default, streaming optional

### Q2: What should raw CDB pass-through look like?

| Option | Description | Selected |
|--------|-------------|----------|
| Method on Session | `sess.Execute(ctx, lun, cdb, opts...)`. Minimal wrapper. | ✓ |
| Separate RawCommand function | Package-level function, not a method. | |
| Command builder | Builder pattern for constructing raw commands. | |

**User's choice:** Method on Session

### Q3: How should errors be structured?

| Option | Description | Selected |
|--------|-------------|----------|
| Typed error hierarchy | SCSIError, TransportError, AuthError. Use errors.As(). | ✓ |
| Single error type with code | One Error type with Kind field. Simpler but less idiomatic. | |
| Sentinel errors + wrapping | Package-level sentinels with wrapping. | |

**User's choice:** Typed error hierarchy

---

## E2E Test Infrastructure

### Q1: What should the primary test target be?

| Option | Description | Selected |
|--------|-------------|----------|
| Custom mock target in Go | Purpose-built mock, full control, in-process. | |
| gotgt embedded target | Real protocol, limited conformance control. | |
| External tgtd via IPC | Real target, platform-dependent. | |
| Tiered: mock for unit, gotgt for integration | Custom mock for conformance, gotgt for full-stack. | ✓ |

**User's choice:** Tiered: mock for unit, gotgt for integration

### Q2: How IOL-inspired should the conformance suite be?

| Option | Description | Selected |
|--------|-------------|----------|
| Structure-inspired | IOL categories, Go-idiomatic tests, IOL refs in comments. | ✓ |
| Minimal conformance checks | RFC checks at key points without IOL mapping. | |
| Full IOL port | Port as many IOL test cases as possible. Very labor-intensive. | |

**User's choice:** Structure-inspired

### Q3: Where should test infrastructure live?

| Option | Description | Selected |
|--------|-------------|----------|
| test/ top-level package | Separate from internal/. Clear unit vs integration split. | ✓ |
| internal/testing sub-package | Closer to code but not importable by consumers. | |
| Alongside existing tests | Mix unit and E2E in existing files. | |

**User's choice:** test/ top-level package

---

## Documentation and Examples

### Q1: What form should examples take?

| Option | Description | Selected |
|--------|-------------|----------|
| Runnable example programs | Full main() programs under examples/. go run-able. | |
| Godoc testable examples | Example functions in _test.go. Shows in godoc. | |
| Both: examples/ programs + godoc examples | Standalone programs plus godoc Examples. | ✓ |

**User's choice:** Both: examples/ programs + godoc examples

### Q2: Documentation scope beyond godoc and examples?

| Option | Description | Selected |
|--------|-------------|----------|
| README + godoc + examples | Good README with overview, quick start, features. | ✓ |
| Godoc + examples only | Minimal, let code speak. | |
| Full docs/ site | Separate documentation site. High effort. | |

**User's choice:** README + godoc + examples

---

## Claude's Discretion

- Exact method signatures for high-level functions
- Which internal types need public equivalents
- Mock target handler registration pattern
- Streaming API naming
- Godoc example selection
- README sections beyond agreed structure

## Deferred Ideas

None — discussion stayed within phase scope.
