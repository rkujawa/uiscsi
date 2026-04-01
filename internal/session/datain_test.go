package session

import (
	"io"
	"testing"

	"github.com/rkujawa/uiscsi/internal/pdu"
)

func TestTaskSingleDataIn(t *testing.T) {
	tk := newTask(1, true, false)

	data := []byte("hello iSCSI")
	din := &pdu.DataIn{
		HasStatus:    true,
		Status:       0x00, // GOOD
		DataSN:       0,
		BufferOffset: 0,
		Data:         data,
	}
	tk.handleDataIn(din)

	result := <-tk.resultCh
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != 0x00 {
		t.Fatalf("status: got 0x%02X, want 0x00", result.Status)
	}
	if result.Data == nil {
		t.Fatal("Data is nil for read command")
	}
	got, err := io.ReadAll(result.Data)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data: got %q, want %q", got, data)
	}
}

func TestTaskMultiDataIn(t *testing.T) {
	tk := newTask(1, true, false)

	// 3 Data-In PDUs without status, then a SCSIResponse.
	chunks := [][]byte{
		[]byte("chunk1"),
		[]byte("chunk2"),
		[]byte("chunk3"),
	}
	offset := uint32(0)
	for i, chunk := range chunks {
		din := &pdu.DataIn{
			DataSN:       uint32(i),
			BufferOffset: offset,
			Data:         chunk,
		}
		tk.handleDataIn(din)
		offset += uint32(len(chunk))
	}

	resp := &pdu.SCSIResponse{
		Status: 0x00,
	}
	tk.handleSCSIResponse(resp)

	result := <-tk.resultCh
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != 0x00 {
		t.Fatalf("status: got 0x%02X, want 0x00", result.Status)
	}

	got, err := io.ReadAll(result.Data)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := "chunk1chunk2chunk3"
	if string(got) != want {
		t.Fatalf("data: got %q, want %q", got, want)
	}
}

func TestTaskDataSNGap(t *testing.T) {
	tk := newTask(1, true, false)

	// First Data-In is fine.
	tk.handleDataIn(&pdu.DataIn{
		DataSN:       0,
		BufferOffset: 0,
		Data:         []byte("ok"),
	})

	// Second Data-In has wrong DataSN (skip to 5).
	tk.handleDataIn(&pdu.DataIn{
		DataSN:       5, // expected 1
		BufferOffset: 2,
		Data:         []byte("bad"),
	})

	result := <-tk.resultCh
	if result.Err == nil {
		t.Fatal("expected error from DataSN gap")
	}
}

func TestTaskOffsetMismatch(t *testing.T) {
	tk := newTask(1, true, false)

	// First Data-In.
	tk.handleDataIn(&pdu.DataIn{
		DataSN:       0,
		BufferOffset: 0,
		Data:         []byte("ab"),
	})

	// Second Data-In with wrong offset.
	tk.handleDataIn(&pdu.DataIn{
		DataSN:       1,
		BufferOffset: 99, // expected 2
		Data:         []byte("cd"),
	})

	result := <-tk.resultCh
	if result.Err == nil {
		t.Fatal("expected error from offset mismatch")
	}
}

func TestTaskNonReadCommand(t *testing.T) {
	tk := newTask(1, false, false) // non-read

	resp := &pdu.SCSIResponse{
		Status: 0x00,
	}
	tk.handleSCSIResponse(resp)

	result := <-tk.resultCh
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Data != nil {
		t.Fatal("Data should be nil for non-read command")
	}
	if result.Status != 0x00 {
		t.Fatalf("status: got 0x%02X, want 0x00", result.Status)
	}
}
