package uiscsi

import (
	"context"
	"fmt"
	"io"

	"github.com/uiscsi/uiscsi/internal/scsi"
	"github.com/uiscsi/uiscsi/internal/session"
)

// Session represents an active iSCSI session. It wraps the internal session
// and provides grouped APIs for SCSI commands, task management, raw CDB
// pass-through, and protocol operations.
//
// Use the accessor methods to access each API group:
//
//	sess.SCSI()     — typed SCSI commands (Inquiry, ReadBlocks, ModeSelect, etc.)
//	sess.TMF()      — task management (AbortTask, LUNReset, etc.)
//	sess.Raw()      — raw CDB pass-through (Execute, StreamExecute)
//	sess.Protocol() — low-level iSCSI protocol (Logout, SendExpStatSNConfirmation)
type Session struct {
	s    *session.Session
	scsi SCSIOps
	tmf  TMFOps
	raw  RawOps
	prot ProtocolOps
}

// initOps sets up the accessor structs after session creation.
func (s *Session) initOps() {
	s.scsi = SCSIOps{s: s.s}
	s.tmf = TMFOps{s: s}
	s.raw = RawOps{s: s}
	s.prot = ProtocolOps{s: s}
}

// Close shuts down the session, performing a graceful logout if possible.
func (s *Session) Close() error {
	return s.s.Close()
}

// SCSI returns the typed SCSI command interface.
func (s *Session) SCSI() *SCSIOps { return &s.scsi }

// TMF returns the task management function interface.
func (s *Session) TMF() *TMFOps { return &s.tmf }

// Raw returns the raw CDB pass-through interface.
func (s *Session) Raw() *RawOps { return &s.raw }

// Protocol returns the low-level iSCSI protocol interface.
func (s *Session) Protocol() *ProtocolOps { return &s.prot }

// ── Internal helpers used by all Ops types ────────────────────────────

// submitAndWait submits a command and waits for the result.
func submitAndWait(ss *session.Session, ctx context.Context, cmd session.Command) (session.Result, error) {
	resultCh, err := ss.Submit(ctx, cmd)
	if err != nil {
		return session.Result{}, wrapTransportError("submit", err)
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		return session.Result{}, ctx.Err()
	}
}

// submitAndCheck submits a command, waits, and checks the result for errors.
func submitAndCheck(ss *session.Session, ctx context.Context, cmd session.Command) ([]byte, error) {
	result, err := submitAndWait(ss, ctx, cmd)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, wrapTransportError("command", result.Err)
	}
	if result.Status != 0 {
		se := &SCSIError{Status: result.Status}
		if len(result.SenseData) > 0 {
			sd, parseErr := scsi.ParseSense(result.SenseData)
			if parseErr == nil {
				se.SenseKey = uint8(sd.Key)
				se.ASC = sd.ASC
				se.ASCQ = sd.ASCQ
				se.Message = sd.String()
			} else {
				se.Message = fmt.Sprintf("sense data present but unparseable: %v", parseErr)
			}
		}
		return nil, se
	}
	if result.Overflow {
		return nil, &SCSIError{
			Status:  result.Status,
			Message: fmt.Sprintf("residual overflow: %d bytes", result.ResidualCount),
		}
	}
	if result.Data != nil {
		return io.ReadAll(result.Data)
	}
	return nil, nil
}

