package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// PortalResult holds discovery results for a single iSCSI portal.
type PortalResult struct {
	Portal  string         `json:"address"`
	Targets []TargetResult `json:"targets"`
	Err     error          `json:"-"`
}

// TargetResult holds discovery results for a single iSCSI target.
type TargetResult struct {
	IQN  string      `json:"iqn"`
	LUNs []LUNResult `json:"luns"`
	Err  error       `json:"-"`
}

// LUNResult holds SCSI inquiry and capacity data for a single LUN.
type LUNResult struct {
	LUN           uint64 `json:"lun"`
	DeviceType    uint8  `json:"device_type_code"`
	DeviceTypeS   string `json:"device_type"`
	Vendor        string `json:"vendor"`
	Product       string `json:"product"`
	Revision      string `json:"revision"`
	CapacityBytes uint64 `json:"capacity_bytes,omitempty"`
	BlockSize     uint32 `json:"block_size,omitempty"`
	LogicalBlocks uint64 `json:"capacity_blocks,omitempty"`
	CapacityStr   string `json:"-"`
}

// formatCapacity returns a human-readable SI capacity string from a
// block count and block size. Uses SI (decimal) units per lsscsi convention.
func formatCapacity(blocks uint64, blockSize uint32) string {
	b := blocks * uint64(blockSize)
	switch {
	case b >= 1e12:
		return fmt.Sprintf("%.2fTB", float64(b)/1e12)
	case b >= 1e9:
		return fmt.Sprintf("%.2fGB", float64(b)/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.2fMB", float64(b)/1e6)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// outputColumnar writes lsscsi-style tab-aligned output to w. Each LUN
// produces one line with target IQN, portal address, LUN number, device
// type, vendor, product, revision, and capacity. Error portals and targets
// are reported on stderr and produce no stdout lines.
func outputColumnar(w io.Writer, results []PortalResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, pr := range results {
		if pr.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", pr.Portal, pr.Err)
			continue
		}
		for _, tr := range pr.Targets {
			if tr.Err != nil {
				fmt.Fprintf(os.Stderr, "error: %s on %s: %v\n", tr.IQN, pr.Portal, tr.Err)
				continue
			}
			for _, lr := range tr.LUNs {
				fmt.Fprintf(tw, "%s\t%s\tLUN %d\t%s\t%-8s\t%-16s\t%-4s\t%s\n",
					tr.IQN,
					pr.Portal,
					lr.LUN,
					lr.DeviceTypeS,
					strings.TrimSpace(lr.Vendor),
					strings.TrimSpace(lr.Product),
					strings.TrimSpace(lr.Revision),
					lr.CapacityStr,
				)
			}
		}
	}
	tw.Flush()
}

// outputJSON writes machine-parseable JSON output to w with indentation.
// The top-level object has a "portals" key containing the result array.
func outputJSON(w io.Writer, results []PortalResult) {
	wrapper := struct {
		Portals []PortalResult `json:"portals"`
	}{
		Portals: results,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(wrapper)
}
