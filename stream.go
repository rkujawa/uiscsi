// stream.go provides streaming I/O variants for block read/write.
package uiscsi

import (
	"context"
	"io"

	"github.com/rkujawa/uiscsi/internal/scsi"
)

// StreamRead reads blocks from the target and returns an io.Reader that
// streams the response data. The reader is single-use; the data comes
// directly from the iSCSI Data-In PDU reassembly without intermediate
// buffering into []byte.
func (s *Session) StreamRead(ctx context.Context, lun uint64, lba uint64, blocks uint32, blockSize uint32) (io.Reader, error) {
	cmd := scsi.Read16(lun, lba, blocks, blockSize)
	result, err := s.submitAndWait(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, wrapTransportError("read", result.Err)
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
			}
		}
		return nil, se
	}
	return result.Data, nil
}

// StreamWrite writes data from r to the target without intermediate buffering.
// The io.Reader must provide exactly blocks*blockSize bytes.
func (s *Session) StreamWrite(ctx context.Context, lun uint64, lba uint64, blocks uint32, blockSize uint32, r io.Reader) error {
	cmd := scsi.Write16(lun, lba, blocks, blockSize, r)
	_, err := s.submitAndCheck(ctx, cmd)
	return err
}
