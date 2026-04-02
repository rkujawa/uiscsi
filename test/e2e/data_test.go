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

const initiatorIQN = "iqn.2026-04.com.uiscsi.e2e:initiator"

// TestDataIntegrity verifies write-then-read byte-for-byte data integrity
// against a real LIO iSCSI target. It writes a recognizable pattern at LBA 0,
// reads it back, then repeats at a non-zero LBA to verify offset handling.
func TestDataIntegrity(t *testing.T) {
	lio.RequireRoot(t)
	lio.RequireModules(t)

	tgt, cleanup := lio.Setup(t, lio.Config{
		TargetSuffix: "data",
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

	// Get block size from ReadCapacity.
	cap, err := sess.ReadCapacity(ctx, 0)
	if err != nil {
		t.Fatalf("ReadCapacity: %v", err)
	}
	if cap.BlockSize == 0 {
		t.Fatal("ReadCapacity returned BlockSize=0")
	}
	blockSize := cap.BlockSize
	t.Logf("ReadCapacity: BlockSize=%d LBA=%d", blockSize, cap.LBA)

	// Create test pattern: 8 blocks with block-index encoding.
	const numBlocks = 8
	testData := make([]byte, numBlocks*int(blockSize))
	for i := range testData {
		// Encode block index in high nibble, byte offset in low byte.
		blockIdx := i / int(blockSize)
		testData[i] = byte((blockIdx << 4) | (i & 0x0F))
	}

	// Write 8 blocks at LBA 0.
	if err := sess.WriteBlocks(ctx, 0, 0, numBlocks, blockSize, testData); err != nil {
		t.Fatalf("WriteBlocks(LBA=0): %v", err)
	}

	// Read back 8 blocks at LBA 0.
	readBack, err := sess.ReadBlocks(ctx, 0, 0, numBlocks, blockSize)
	if err != nil {
		t.Fatalf("ReadBlocks(LBA=0): %v", err)
	}

	if !bytes.Equal(readBack, testData) {
		// Find first differing offset for diagnostic.
		for i := range testData {
			if i >= len(readBack) {
				t.Errorf("read data too short: got %d bytes, want %d", len(readBack), len(testData))
				break
			}
			if readBack[i] != testData[i] {
				t.Errorf("data mismatch at offset %d: got 0x%02x, want 0x%02x", i, readBack[i], testData[i])
				break
			}
		}
		t.Fatal("data integrity check failed at LBA 0")
	}
	t.Log("LBA 0: write-then-read byte-for-byte match OK")

	// Test at non-zero LBA to verify offset handling.
	const offsetLBA = 100
	offsetData := make([]byte, numBlocks*int(blockSize))
	for i := range offsetData {
		offsetData[i] = byte(0xAA ^ byte(i&0xFF))
	}

	if err := sess.WriteBlocks(ctx, 0, offsetLBA, numBlocks, blockSize, offsetData); err != nil {
		t.Fatalf("WriteBlocks(LBA=%d): %v", offsetLBA, err)
	}

	readOffset, err := sess.ReadBlocks(ctx, 0, offsetLBA, numBlocks, blockSize)
	if err != nil {
		t.Fatalf("ReadBlocks(LBA=%d): %v", offsetLBA, err)
	}

	if !bytes.Equal(readOffset, offsetData) {
		for i := range offsetData {
			if i >= len(readOffset) {
				t.Errorf("read data too short at LBA %d: got %d bytes, want %d", offsetLBA, len(readOffset), len(offsetData))
				break
			}
			if readOffset[i] != offsetData[i] {
				t.Errorf("data mismatch at LBA %d offset %d: got 0x%02x, want 0x%02x", offsetLBA, i, readOffset[i], offsetData[i])
				break
			}
		}
		t.Fatalf("data integrity check failed at LBA %d", offsetLBA)
	}
	t.Logf("LBA %d: write-then-read byte-for-byte match OK", offsetLBA)
}
