package ata

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// IdentifyInfo holds the parsed result of ATA IDENTIFY DEVICE (512 bytes).
type IdentifyInfo struct {
	ModelNumber     string
	SerialNumber    string
	FirmwareVersion string

	// UserCapacity is total addressable sectors × sector size in bytes.
	UserCapacity uint64
	// LogicalSectorSize is the logical sector size in bytes (usually 512).
	LogicalSectorSize uint32
	// PhysicalSectorSize is the physical sector size in bytes.
	PhysicalSectorSize uint32

	// RPM is the nominal media rotation rate: 1 = SSD, >1 = HDD RPM, 0 = unknown.
	RPM int

	// SMARTSupported is true when the device supports SMART.
	SMARTSupported bool
	// SMARTEnabled is true when SMART is currently enabled.
	SMARTEnabled bool

	// LBA48 is true when the device supports 48-bit LBA.
	LBA48 bool

	// ATAVersionMajor is the highest supported ATA major version (bit position).
	ATAVersionMajor uint16
	// SATACapabilities is word 76 (SATA capabilities).
	SATACapabilities uint16
	// SATACurrentSpeed is the current negotiated SATA speed indicator (bits [3:1] of word 77).
	SATACurrentSpeed uint8

	// Raw holds the original 512-byte response for fields not individually parsed.
	Raw []byte
}

// ParseIdentify parses a 512-byte ATA IDENTIFY DEVICE response.
func ParseIdentify(data []byte) (*IdentifyInfo, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("identify: expected 512 bytes, got %d", len(data))
	}

	word := func(n int) uint16 {
		return binary.LittleEndian.Uint16(data[n*2 : n*2+2])
	}

	info := &IdentifyInfo{Raw: append([]byte(nil), data...)}

	// Words 27–46: model number (40 bytes, big-endian character pairs).
	info.ModelNumber = ataString(data[54:94])
	// Words 10–19: serial number (20 bytes).
	info.SerialNumber = ataString(data[20:40])
	// Words 23–26: firmware revision (8 bytes).
	info.FirmwareVersion = ataString(data[46:54])

	// Word 217: nominal media rotation rate.
	rotRate := word(217)
	if rotRate == 0x0001 {
		info.RPM = 1 // SSD
	} else if rotRate >= 0x0401 && rotRate <= 0xfffe {
		info.RPM = int(rotRate)
	}

	// Capacity: prefer 48-bit LBA (words 100–103), fall back to 28-bit (words 60–61).
	w82 := word(82)
	info.SMARTSupported = (w82 & 0x0001) != 0
	info.LBA48 = (word(83) & 0x0400) != 0

	var totalSectors uint64
	if info.LBA48 {
		lo := uint64(word(100)) | uint64(word(101))<<16
		hi := uint64(word(102)) | uint64(word(103))<<16
		totalSectors = lo | hi<<32
	} else {
		totalSectors = uint64(word(60)) | uint64(word(61))<<16
	}

	// Logical sector size: word 117-118 if word 106 bit 12 is set.
	info.LogicalSectorSize = 512
	if word(106)&0x1000 != 0 {
		ls := uint32(word(117)) | uint32(word(118))<<16
		if ls > 0 {
			info.LogicalSectorSize = ls * 2
		}
	}
	info.UserCapacity = totalSectors * uint64(info.LogicalSectorSize)

	// Physical sector size: word 106 bits [3:0] give log2(phys/logical).
	physShift := word(106) & 0x000f
	info.PhysicalSectorSize = info.LogicalSectorSize << physShift

	// SMART enabled: word 85 bit 0.
	info.SMARTEnabled = (word(85) & 0x0001) != 0

	// ATA version: word 80 (bit field, highest set bit is version).
	info.ATAVersionMajor = highestBit(word(80))

	// SATA: word 76 capabilities, word 77 additional capabilities.
	info.SATACapabilities = word(76)
	// Current speed: word 77 bits [3:1]
	info.SATACurrentSpeed = uint8((word(77) >> 1) & 0x7)

	return info, nil
}

// ataString converts an ATA character field (byte-pair swapped) to a trimmed string.
// ATA stores strings in words with the high byte holding the first character.
// In little-endian memory this appears as swapped pairs.
func ataString(b []byte) string {
	out := make([]byte, len(b))
	for i := 0; i+1 < len(b); i += 2 {
		out[i] = b[i+1]
		out[i+1] = b[i]
	}
	return strings.TrimRight(string(out), " ")
}

// highestBit returns the position (1-based) of the highest set bit, or 0.
func highestBit(v uint16) uint16 {
	var pos uint16
	for i := uint16(15); i > 0; i-- {
		if v&(1<<i) != 0 {
			pos = i
			break
		}
	}
	return pos
}
