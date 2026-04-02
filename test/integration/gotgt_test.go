//go:build integration

// Package integration_test contains integration tests that run against a
// real iSCSI target (gotgt). These are excluded from normal `go test ./...`
// runs by the build tag. Run with:
//
//	go test -tags integration ./test/integration/
//
// This is tier 2 of the D-07 tiered test approach:
//   - Tier 1: Custom mock target (test/conformance/) -- always runs
//   - Tier 2: Gotgt embedded target (test/integration/) -- requires build tag
package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/rkujawa/uiscsi"
)

// TestGotgt_Dial verifies that Dial connects to an embedded gotgt target
// and establishes a session.
func TestGotgt_Dial(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO (Phase 8 or follow-up): Wire up gotgt embedded target.
	//
	// Intended test flow:
	//   1. Start embedded gotgt target with iscsit.NewServer()
	//   2. Configure a test LUN with a RAM-backed backing store
	//   3. sess, err := uiscsi.Dial(ctx, gotgtAddr, uiscsi.WithTarget(gotgtIQN))
	//   4. Verify session established
	//   5. sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
}

// TestGotgt_ReadWrite verifies data integrity by writing a block then
// reading it back through gotgt.
func TestGotgt_ReadWrite(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO: Write 512 bytes to LBA 0, read back, verify byte-for-byte match.
	//
	// Intended test flow:
	//   1. Start embedded gotgt with RAM-backed LUN
	//   2. Dial and login
	//   3. sess.WriteBlocks(ctx, 0, 0, 1, 512, testData)
	//   4. data, _ := sess.ReadBlocks(ctx, 0, 0, 1, 512)
	//   5. Verify data == testData
	//   6. sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
}

// TestGotgt_Inquiry verifies INQUIRY returns valid device data from gotgt.
func TestGotgt_Inquiry(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO: Dial, login, sess.Inquiry(ctx, 0), verify VendorID and ProductID are non-empty.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
}

// TestGotgt_ReadCapacity verifies READ CAPACITY returns valid size from gotgt.
func TestGotgt_ReadCapacity(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO: Dial, login, sess.ReadCapacity(ctx, 0), verify LBA > 0 and BlockSize > 0.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
}

// TestGotgt_Discovery verifies Discover returns gotgt's target IQN.
func TestGotgt_Discovery(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO: Start gotgt, call uiscsi.Discover(ctx, addr), verify target IQN in results.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx
}

// TestGotgt_CHAP verifies CHAP authentication against gotgt.
func TestGotgt_CHAP(t *testing.T) {
	t.Skip("gotgt integration not yet wired -- enable when gotgt dependency is added")

	// TODO: Configure gotgt with CHAP credentials, Dial with uiscsi.WithCHAP(), verify login.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = ctx
	_ = uiscsi.WithCHAP("user", "secret")
}
