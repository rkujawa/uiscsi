package uiscsi_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/uiscsi/uiscsi"
)

func ExampleDial() {
	ctx := context.Background()
	sess, err := uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
	)
	if err != nil {
		fmt.Println("dial:", err)
		return
	}
	defer sess.Close()
	fmt.Println("connected")
}

func ExampleDiscover() {
	ctx := context.Background()
	targets, err := uiscsi.Discover(ctx, "192.168.1.100:3260")
	if err != nil {
		fmt.Println("discover:", err)
		return
	}
	for _, t := range targets {
		fmt.Printf("target: %s (%d portals)\n", t.Name, len(t.Portals))
	}
}

func ExampleSCSIOps_ReadBlocks() {
	ctx := context.Background()
	sess, err := uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
	)
	if err != nil {
		return
	}
	defer sess.Close()

	data, err := sess.SCSI().ReadBlocks(ctx, 0, 0, 1, 512)
	if err != nil {
		fmt.Println("read:", err)
		return
	}
	fmt.Printf("read %d bytes\n", len(data))
}

func ExampleSCSIOps_WriteBlocks() {
	// Show write + readback verification pattern.
	ctx := context.Background()
	sess, err := uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
	)
	if err != nil {
		return
	}
	defer sess.Close()

	data := make([]byte, 512)
	copy(data, []byte("hello iSCSI"))
	if err := sess.SCSI().WriteBlocks(ctx, 0, 0, 1, 512, data); err != nil {
		fmt.Println("write:", err)
	}
}

func ExampleRawOps_Execute() {
	// Raw CDB pass-through: TEST UNIT READY.
	ctx := context.Background()
	sess, err := uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
	)
	if err != nil {
		return
	}
	defer sess.Close()

	turCDB := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00} // TEST UNIT READY
	result, err := sess.Raw().Execute(ctx, 0, turCDB)
	if err != nil {
		fmt.Println("execute:", err)
		return
	}
	fmt.Printf("status: 0x%02X\n", result.Status)
}

func ExampleWithLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()
	_, _ = uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
		uiscsi.WithLogger(logger),
	)
}

func ExampleWithCHAP() {
	ctx := context.Background()
	_, _ = uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:storage"),
		uiscsi.WithCHAP("initiator-user", "s3cret"),
	)
}

func ExampleRawOps_StreamExecute() {
	// Streaming raw CDB: read one block from a tape drive.
	ctx := context.Background()
	sess, err := uiscsi.Dial(ctx, "192.168.1.100:3260",
		uiscsi.WithTarget("iqn.2026-03.com.example:tape"),
		uiscsi.WithMaxRecvDataSegmentLength(262144), // 256KB PDUs for tape
	)
	if err != nil {
		return
	}
	defer sess.Close()

	// SSC READ(6): opcode 0x08, read 1 block of 65536 bytes.
	const blockSize = 65536
	cdb := []byte{0x08, 0x00, 0x00, 0x01, 0x00, 0x00}
	sr, err := sess.Raw().StreamExecute(ctx, 0, cdb, uiscsi.WithDataIn(blockSize))
	if err != nil {
		fmt.Println("stream execute:", err)
		return
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, sr.Data); err != nil {
		fmt.Println("stream read:", err)
		return
	}
	status, _, err := sr.Wait()
	if err != nil {
		fmt.Println("wait:", err)
		return
	}
	fmt.Printf("status: 0x%02X, bytes: %d\n", status, buf.Len())
}
