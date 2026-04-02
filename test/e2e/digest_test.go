//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
	"github.com/rkujawa/uiscsi/test/lio"
)

// TestDigests verifies CRC32C header and data digest negotiation against a
// real LIO target. LIO supports CRC32C by default. After successful negotiation,
// a write+read cycle exercises digests on actual data transfer.
func TestDigests(t *testing.T) {
	lio.RequireRoot(t)
	lio.RequireModules(t)

	tgt, cleanup := lio.Setup(t, lio.Config{
		TargetSuffix: "digest",
		InitiatorIQN: initiatorIQN,
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := uiscsi.Dial(ctx, tgt.Addr,
		uiscsi.WithTarget(tgt.IQN),
		uiscsi.WithInitiatorName(initiatorIQN),
		uiscsi.WithHeaderDigest("CRC32C"),
		uiscsi.WithDataDigest("CRC32C"),
	)
	if err != nil {
		t.Fatalf("Dial with CRC32C digests: %v", err)
	}
	defer sess.Close()

	t.Log("Digest negotiation succeeded (CRC32C header + data)")

	// Exercise data transfer with digests active to verify CRC32C computation
	// in both directions (write = initiator computes, read = initiator verifies).
	const blockSize uint32 = 512
	testData := make([]byte, blockSize)
	for i := range testData {
		testData[i] = byte(i & 0xFF)
	}

	if err := sess.WriteBlocks(ctx, 0, 0, 1, blockSize, testData); err != nil {
		t.Fatalf("WriteBlocks with digests: %v", err)
	}

	readBack, err := sess.ReadBlocks(ctx, 0, 0, 1, blockSize)
	if err != nil {
		t.Fatalf("ReadBlocks with digests: %v", err)
	}

	if !bytes.Equal(readBack, testData) {
		t.Fatal("data mismatch with CRC32C digests active")
	}
	t.Log("Write+Read with CRC32C digests: data integrity OK")
}
