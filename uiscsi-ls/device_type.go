package main

// deviceTypeNames maps SCSI peripheral device type codes (SPC-4 Table 49)
// to lsscsi-standard short names. Indexed by code (0x00-0x1F). Blank
// entries map to "unknown".
var deviceTypeNames = [32]string{
	0x00: "disk",
	0x01: "tape",
	0x02: "printer",
	0x03: "processor",
	0x04: "worm",
	0x05: "cd/dvd",
	0x06: "scanner",
	0x07: "optical",
	0x08: "medchgr",
	0x09: "comms",
	0x0C: "storage",
	0x0D: "enclosu",
	0x0E: "disk",
	0x0F: "osd",
	0x11: "osd",
	0x1E: "wlun",
	0x1F: "unknown",
}

// deviceTypeName returns the lsscsi-standard short name for a SCSI
// peripheral device type code (SPC-4 Table 49). Unknown or unmapped
// codes return "unknown".
func deviceTypeName(code uint8) string {
	if int(code) >= len(deviceTypeNames) {
		return "unknown"
	}
	name := deviceTypeNames[code]
	if name == "" {
		return "unknown"
	}
	return name
}
