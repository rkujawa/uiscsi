// stream.go provides streaming I/O variants for block read/write.
package uiscsi

import (
	"context"
	"io"

	"github.com/rkujawa/uiscsi/internal/scsi"
)

// StreamWrite writes data from r to the target without intermediate buffering.
// The io.Reader must provide exactly blocks*blockSize bytes.
func (s *Session) StreamWrite(ctx context.Context, lun uint64, lba uint64, blocks uint32, blockSize uint32, r io.Reader) error {
	cmd := scsi.Write16(lun, lba, blocks, blockSize, r)
	_, err := s.submitAndCheck(ctx, cmd)
	return err
}
