// types.go defines the public wrapper types for the uiscsi package.
// All types are defined in the root package to avoid exposing internal types.
package uiscsi

import (
	"time"

	"github.com/rkujawa/uiscsi/internal/scsi"
	"github.com/rkujawa/uiscsi/internal/session"
)

// Target represents a discovered iSCSI target.
type Target struct {
	Name    string
	Portals []Portal
}

// Portal represents a target portal address.
type Portal struct {
	Address  string
	Port     int
	GroupTag int
}

// Result carries a SCSI command outcome. Data is already consumed into []byte
// by the typed Session methods.
type Result struct {
	Status        uint8
	Data          []byte
	SenseData     []byte
	Overflow      bool
	Underflow     bool
	ResidualCount uint32
}

// RawResult carries the raw SCSI command outcome for Execute().
type RawResult struct {
	Status    uint8
	Data      []byte
	SenseData []byte
}

// InquiryData holds a parsed INQUIRY response.
type InquiryData struct {
	DeviceType uint8
	VendorID   string
	ProductID  string
	Revision   string
}

// Capacity holds a parsed READ CAPACITY response (unified for 10/16).
type Capacity struct {
	LBA           uint64
	BlockSize     uint32
	LogicalBlocks uint64
}

// SenseInfo holds parsed sense data.
type SenseInfo struct {
	Key         uint8
	ASC         uint8
	ASCQ        uint8
	Description string
}

// TMFResult carries a task management function outcome.
type TMFResult struct {
	Response uint8
}

// AsyncEvent carries an asynchronous event from the target.
type AsyncEvent struct {
	EventCode  uint8
	VendorCode uint8
	Parameter1 uint16
	Parameter2 uint16
	Parameter3 uint16
	Data       []byte
}

// MetricEventType discriminates metric event kinds.
type MetricEventType uint8

// Metric event type constants, mirroring internal session values.
const (
	MetricPDUSent         MetricEventType = MetricEventType(session.MetricPDUSent)
	MetricPDUReceived     MetricEventType = MetricEventType(session.MetricPDUReceived)
	MetricCommandComplete MetricEventType = MetricEventType(session.MetricCommandComplete)
	MetricBytesIn         MetricEventType = MetricEventType(session.MetricBytesIn)
	MetricBytesOut        MetricEventType = MetricEventType(session.MetricBytesOut)
)

// MetricEvent carries a single metric observation.
type MetricEvent struct {
	Type    MetricEventType
	OpCode  uint8
	Bytes   uint64
	Latency time.Duration
}

// PDUDirection indicates whether a PDU was sent or received.
type PDUDirection uint8

// PDU direction constants, mirroring internal session values.
const (
	PDUSend    PDUDirection = PDUDirection(session.PDUSend)
	PDUReceive PDUDirection = PDUDirection(session.PDUReceive)
)

// UnmapBlockDescriptor describes a single LBA range to deallocate.
type UnmapBlockDescriptor struct {
	LBA    uint64
	Blocks uint32
}

// convertTarget converts an internal DiscoveryTarget to a public Target.
func convertTarget(dt session.DiscoveryTarget) Target {
	t := Target{Name: dt.Name}
	for _, p := range dt.Portals {
		t.Portals = append(t.Portals, Portal{
			Address:  p.Address,
			Port:     p.Port,
			GroupTag: p.GroupTag,
		})
	}
	return t
}

// convertInquiry converts an internal InquiryResponse to a public InquiryData.
func convertInquiry(r *scsi.InquiryResponse) *InquiryData {
	return &InquiryData{
		DeviceType: r.PeripheralDeviceType,
		VendorID:   r.Vendor,
		ProductID:  r.Product,
		Revision:   r.Revision,
	}
}

// convertCapacity16 converts a ReadCapacity16Response to a public Capacity.
func convertCapacity16(r *scsi.ReadCapacity16Response) *Capacity {
	return &Capacity{
		LBA:           r.LastLBA,
		BlockSize:     r.BlockSize,
		LogicalBlocks: r.LastLBA + 1,
	}
}

// convertCapacity10 converts a ReadCapacity10Response to a public Capacity.
func convertCapacity10(r *scsi.ReadCapacity10Response) *Capacity {
	return &Capacity{
		LBA:           uint64(r.LastLBA),
		BlockSize:     r.BlockSize,
		LogicalBlocks: uint64(r.LastLBA) + 1,
	}
}

// convertSense converts internal SenseData to a public SenseInfo.
func convertSense(sd *scsi.SenseData) *SenseInfo {
	return &SenseInfo{
		Key:         uint8(sd.Key),
		ASC:         sd.ASC,
		ASCQ:        sd.ASCQ,
		Description: sd.String(),
	}
}

// convertTMFResult converts an internal TMFResult to a public TMFResult.
func convertTMFResult(r *session.TMFResult) *TMFResult {
	return &TMFResult{Response: r.Response}
}

// convertAsyncEvent converts an internal AsyncEvent to a public AsyncEvent.
func convertAsyncEvent(ae session.AsyncEvent) AsyncEvent {
	return AsyncEvent{
		EventCode:  ae.EventCode,
		VendorCode: ae.VendorCode,
		Parameter1: ae.Parameter1,
		Parameter2: ae.Parameter2,
		Parameter3: ae.Parameter3,
		Data:       ae.Data,
	}
}

// convertMetricEvent converts an internal MetricEvent to a public MetricEvent.
func convertMetricEvent(me session.MetricEvent) MetricEvent {
	return MetricEvent{
		Type:    MetricEventType(me.Type),
		OpCode:  uint8(me.OpCode),
		Bytes:   me.Bytes,
		Latency: me.Latency,
	}
}
