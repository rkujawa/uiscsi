// Package transport implements the iSCSI TCP transport layer: connection
// management, PDU framing over TCP streams, concurrent read/write pumps,
// ITT-based response routing, and buffer pool management.
package transport

import (
	"sync"
	"sync/atomic"

	"github.com/uiscsi/uiscsi/internal/pdu"
)

// bhsPool reuses 48-byte BHS buffers to reduce GC pressure during PDU framing.
var bhsPool = sync.Pool{
	New: func() any { return new([pdu.BHSLength]byte) },
}

// GetBHS returns a reusable 48-byte BHS buffer from the pool.
// The caller must call PutBHS when done.
func GetBHS() *[pdu.BHSLength]byte {
	return bhsPool.Get().(*[pdu.BHSLength]byte)
}

// PutBHS returns a BHS buffer to the pool for reuse.
func PutBHS(b *[pdu.BHSLength]byte) {
	bhsPool.Put(b)
}

// Size classes for data segment buffer pooling. Tiers match common
// MaxRecvDataSegmentLength values: 4KB (small responses), 64KB (typical
// high-throughput MRDSL), 16MB (RFC 7143 maximum, 24-bit DS length field).
const (
	smallBufSize  = 4096    // <= 4KB: status responses, sense data
	mediumBufSize = 65536   // <= 64KB: default MRDSL range
	largeBufSize  = 1 << 24 // <= 16MB: RFC 7143 max data segment length
)

// poolTierMax is the soft upper bound on the number of buffers retained per
// tier. When a tier exceeds this count, PutBuffer drops the buffer and lets
// the GC collect it, preventing unbounded memory retention after burst traffic.
// 256 buffers × 16MB per large buffer = 4GB theoretical maximum for the large
// tier — in practice the soft bound is a safety valve, not a steady-state limit.
const poolTierMax = 256

// Per-tier atomic counters track the approximate number of buffers currently
// held in each pool tier. GetBuffer decrements, PutBuffer increments.
// Counters may go slightly negative (sync.Pool.New created fresh buffer with
// no matching Put); this is acceptable for a soft bound.
var (
	smallCount  atomic.Int64
	mediumCount atomic.Int64
	largeCount  atomic.Int64
)

// SA6002: store *[]byte (pointer to slice header) in sync.Pool instead of
// []byte (slice header value). A []byte is a 3-word struct; passing it to
// Pool.Put boxes it into an interface, causing an allocation on every Put and
// defeating the pool's purpose. Storing a pointer avoids this allocation.
var (
	smallPool = sync.Pool{
		New: func() any {
			b := make([]byte, smallBufSize)
			return &b
		},
	}
	mediumPool = sync.Pool{
		New: func() any {
			b := make([]byte, mediumBufSize)
			return &b
		},
	}
	largePool = sync.Pool{
		New: func() any {
			b := make([]byte, largeBufSize)
			return &b
		},
	}
)

// GetBuffer returns a *[]byte from a size-class pool whose capacity is at
// least size bytes. The returned pointer is a pool-owned value — callers must
// call PutBuffer when done. The underlying slice is NOT zeroed on return.
//
// For oversized allocations (> largeBufSize), a fresh slice is allocated and
// returned as &b; it is not pooled and PutBuffer is a no-op for it.
func GetBuffer(size int) *[]byte {
	switch {
	case size <= smallBufSize:
		smallCount.Add(-1)
		bp := smallPool.Get().(*[]byte)
		*bp = (*bp)[:size]
		return bp
	case size <= mediumBufSize:
		mediumCount.Add(-1)
		bp := mediumPool.Get().(*[]byte)
		*bp = (*bp)[:size]
		return bp
	case size <= largeBufSize:
		largeCount.Add(-1)
		bp := largePool.Get().(*[]byte)
		*bp = (*bp)[:size]
		return bp
	default:
		// Oversized: allocate directly, not pooled.
		b := make([]byte, size)
		return &b
	}
}

// PutBuffer returns a *[]byte to the appropriate size-class pool.
// Buffers smaller than smallBufSize or larger than largeBufSize are dropped
// silently. If the tier is at or above poolTierMax, the buffer is dropped and
// the GC collects it (soft bound per D-03).
func PutBuffer(bp *[]byte) {
	if bp == nil {
		return
	}
	c := cap(*bp)
	// Reset slice length to full capacity before returning to pool.
	*bp = (*bp)[:c]
	switch {
	case c >= largeBufSize:
		if largeCount.Add(1) <= poolTierMax {
			largePool.Put(bp)
		} else {
			largeCount.Add(-1) // over limit, let GC collect
		}
	case c >= mediumBufSize:
		if mediumCount.Add(1) <= poolTierMax {
			mediumPool.Put(bp)
		} else {
			mediumCount.Add(-1)
		}
	case c >= smallBufSize:
		if smallCount.Add(1) <= poolTierMax {
			smallPool.Put(bp)
		} else {
			smallCount.Add(-1)
		}
	// Smaller than smallest pool class: drop it silently.
	}
}
