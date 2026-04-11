package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/uiscsi/uiscsi/internal/pdu"
)

// PDU hook direction constants. These live in the transport package to
// avoid a circular dependency between transport and session.
const (
	HookSend    uint8 = 0
	HookReceive uint8 = 1
)

// INVARIANT: Single-Writer Rule
//
// All TCP writes to the iSCSI connection MUST go through WritePump via writeCh.
// Direct writes to the underlying net.Conn bypass serialization and risk TCP
// byte interleaving (see CLAUDE.md Pitfall 7).
//
// Verified: All production write paths (Submit, SubmitStreaming, sendDataOutBurst,
// sendKeepalivePing, handleUnsolicitedNOPIn, SendExpStatSNConfirmation, renegotiate)
// send PDUs through writeCh. Login negotiation (login.go) performs direct I/O before
// pumps start, which is correct.
//
// Test coverage: TestWritePump_ConcurrentWriters verifies no data races under -race.

// WritePump owns all writes to the underlying connection. It receives RawPDUs
// from writeCh and serializes them to w one at a time, preventing TCP byte
// interleaving (Pitfall 7). Returns when ctx is cancelled or writeCh is closed.
func WritePump(ctx context.Context, w io.Writer, writeCh <-chan *RawPDU,
	logger *slog.Logger, pduHook func(uint8, *RawPDU), digestByteOrder binary.ByteOrder) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p, ok := <-writeCh:
			if !ok {
				return nil // channel closed
			}
			if pduHook != nil {
				pduHook(HookSend, p)
			}
			if logger.Enabled(ctx, slog.LevelDebug) {
				opcode := pdu.OpCode(p.BHS[0] & 0x3f)
				itt := binary.BigEndian.Uint32(p.BHS[16:20])
				cmdSN := binary.BigEndian.Uint32(p.BHS[24:28])
				dsLen := uint32(p.BHS[5])<<16 | uint32(p.BHS[6])<<8 | uint32(p.BHS[7])
				logger.DebugContext(ctx, "pdu sent",
					"opcode", opcode.String(),
					"itt", fmt.Sprintf("0x%08x", itt),
					"cmd_sn", cmdSN,
					"ds_len", dsLen)
			}
			if err := WriteRawPDU(w, p, digestByteOrder); err != nil {
				return err
			}
		}
	}
}

// ReadPump continuously reads PDUs from r and dispatches them by ITT.
// PDUs with the reserved ITT 0xFFFFFFFF (unsolicited target PDUs such as
// NOP-In pings and async messages) are classified by opcode before dispatch:
//   - NOP-In (opcode 0x20) uses a blocking send so it is never dropped — the
//     target awaits a NOP-Out reply and dropping would cause a session timeout
//     (RFC 7143 Section 11.19). The blocking send respects ctx cancellation.
//   - All other unsolicited PDUs are delivered non-blocking; if unsolicitedCh
//     is full they are dropped and the optional dropCounter is incremented.
//
// All other PDUs are delivered via router.Dispatch. Returns when the read fails
// (connection closed) or ctx is cancelled.
//
// dropCounter is optional (*atomic.Uint64). If non-nil, every dropped optional
// async PDU increments it. Callers (e.g., tests) can assert zero drops for
// RFC-required opcode types.
func ReadPump(ctx context.Context, r io.Reader, router *Router,
	unsolicitedCh chan<- *RawPDU, digestHeader, digestData bool,
	logger *slog.Logger, pduHook func(uint8, *RawPDU), maxRecvDSL uint32,
	digestByteOrder binary.ByteOrder, dropCounter *atomic.Uint64) error {
	for {
		// Check cancellation before each read.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		raw, err := ReadRawPDU(r, digestHeader, digestData, maxRecvDSL, digestByteOrder)
		if err != nil {
			// Check if context was cancelled during the read.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			return err
		}

		if pduHook != nil {
			pduHook(HookReceive, raw)
		}
		if logger.Enabled(ctx, slog.LevelDebug) {
			opcode := pdu.OpCode(raw.BHS[0] & 0x3f)
			itt := binary.BigEndian.Uint32(raw.BHS[16:20])
			statSN := binary.BigEndian.Uint32(raw.BHS[24:28])
			dsLen := uint32(raw.BHS[5])<<16 | uint32(raw.BHS[6])<<8 | uint32(raw.BHS[7])
			logger.DebugContext(ctx, "pdu received",
				"opcode", opcode.String(),
				"itt", fmt.Sprintf("0x%08x", itt),
				"stat_sn", statSN,
				"ds_len", dsLen)
		}

		// Extract ITT from BHS bytes 16-19.
		itt := binary.BigEndian.Uint32(raw.BHS[16:20])

		if itt == reservedITT {
			opcode := pdu.OpCode(raw.BHS[0] & 0x3f)
			switch opcode {
			case pdu.OpNOPIn:
				// RFC 7143 Section 11.19: NOP-In with TTT != 0xFFFFFFFF requires
				// inline NOP-Out reply. MUST NOT be dropped — the target is waiting
				// for a ping reply; dropping causes a session timeout.
				select {
				case unsolicitedCh <- raw:
				case <-ctx.Done():
					return ctx.Err()
				}
			default:
				// Optional async PDUs: deliver if channel has capacity,
				// otherwise count and log the drop.
				select {
				case unsolicitedCh <- raw:
				default:
					if dropCounter != nil {
						dropCounter.Add(1)
					}
					logger.Warn("transport: optional async PDU dropped",
						"opcode", opcode.String())
				}
			}
			continue
		}

		if !router.Dispatch(itt, raw) {
			logger.Warn("transport: no pending entry for ITT, dropping PDU",
				"itt", itt,
				"opcode", raw.BHS[0]&0x3f)
		}
	}
}
