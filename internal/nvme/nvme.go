// Package nvme provides NVMe Admin Command data parsing for smartctl --all.
package nvme

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// IdentifyController holds the parsed result of NVMe Identify Controller (4096 bytes).
type IdentifyController struct {
	VendorID     uint16
	Subsys       uint16
	SerialNumber string
	ModelNumber  string
	FWRevision   string

	// TotalCapacityBytes is the total NVM capacity in bytes (field TNVMCAP, 128-bit LE).
	// We store the lower 64 bits, which covers up to 16 EiB.
	TotalCapacityBytes uint64

	// Version encodes the NVMe spec version: [31:16] major, [15:8] minor, [7:0] tertiary.
	Version uint32

	// MaxPowerStates is the number of power states supported (NPSS field + 1).
	MaxPowerStates uint8
}

// ParseIdentifyController parses a 4096-byte NVMe Identify Controller response.
func ParseIdentifyController(data []byte) (*IdentifyController, error) {
	if len(data) != 4096 {
		return nil, fmt.Errorf("identify controller: expected 4096 bytes, got %d", len(data))
	}

	ic := &IdentifyController{}
	ic.VendorID = binary.LittleEndian.Uint16(data[0:2])
	ic.Subsys = binary.LittleEndian.Uint16(data[2:4])

	// SN: bytes 4–23 (20 ASCII bytes, space padded)
	ic.SerialNumber = strings.TrimRight(string(data[4:24]), " ")
	// MN: bytes 24–63 (40 ASCII bytes)
	ic.ModelNumber = strings.TrimRight(string(data[24:64]), " ")
	// FR: bytes 64–71 (8 ASCII bytes)
	ic.FWRevision = strings.TrimRight(string(data[64:72]), " ")

	// VER: bytes 80–83
	ic.Version = binary.LittleEndian.Uint32(data[80:84])

	// TNVMCAP: bytes 280–295 (128-bit LE), store lower 64 bits.
	ic.TotalCapacityBytes = binary.LittleEndian.Uint64(data[280:288])

	// NPSS: byte 263 (0-based count), so total = NPSS+1
	ic.MaxPowerStates = data[263] + 1

	return ic, nil
}

// SmartLog holds the parsed NVMe SMART/Health Information Log (512 bytes, log ID 0x02).
type SmartLog struct {
	CriticalWarning    uint8
	// TemperatureKelvin is the composite temperature in Kelvin.
	TemperatureKelvin  uint16
	AvailableSpare     uint8
	SpareThreshold     uint8
	PercentageUsed     uint8

	// The following are 128-bit little-endian counters; we use uint64 (lower bits).
	DataUnitsRead    uint64
	DataUnitsWritten uint64
	HostReads        uint64
	HostWrites       uint64
	ControllerBusyTime uint64
	PowerCycles      uint64
	PowerOnHours     uint64
	UnsafeShutdowns  uint64
	MediaErrors      uint64
	NumErrLogEntries uint64

	WarningTempTime  uint32
	CriticalCompTime uint32
	TempSensor       [8]uint16
}

// TemperatureCelsius converts the composite temperature to Celsius.
func (s *SmartLog) TemperatureCelsius() int {
	if s.TemperatureKelvin == 0 {
		return 0
	}
	return int(s.TemperatureKelvin) - 273
}

// ParseSmartLog parses a 512-byte NVMe SMART/Health Information Log.
func ParseSmartLog(data []byte) (*SmartLog, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("NVMe SMART log: expected 512 bytes, got %d", len(data))
	}

	s := &SmartLog{}
	s.CriticalWarning = data[0]
	s.TemperatureKelvin = binary.LittleEndian.Uint16(data[1:3])
	s.AvailableSpare = data[3]
	s.SpareThreshold = data[4]
	s.PercentageUsed = data[5]

	// 128-bit fields starting at byte 32; store lower 64 bits.
	s.DataUnitsRead = le128lo64(data[32:48])
	s.DataUnitsWritten = le128lo64(data[48:64])
	s.HostReads = le128lo64(data[64:80])
	s.HostWrites = le128lo64(data[80:96])
	s.ControllerBusyTime = le128lo64(data[96:112])
	s.PowerCycles = le128lo64(data[112:128])
	s.PowerOnHours = le128lo64(data[128:144])
	s.UnsafeShutdowns = le128lo64(data[144:160])
	s.MediaErrors = le128lo64(data[160:176])
	s.NumErrLogEntries = le128lo64(data[176:192])

	s.WarningTempTime = binary.LittleEndian.Uint32(data[192:196])
	s.CriticalCompTime = binary.LittleEndian.Uint32(data[196:200])
	for i := 0; i < 8; i++ {
		s.TempSensor[i] = binary.LittleEndian.Uint16(data[200+i*2 : 202+i*2])
	}

	return s, nil
}

// ErrorLogEntry is one NVMe Error Information Log entry (64 bytes).
type ErrorLogEntry struct {
	ErrorCount         uint64
	SQID               uint16
	CMDID              uint16
	StatusField        uint16
	ParamErrorLocation uint16
	LBA                uint64
	NSID               uint32
	VendorSpecific     uint8
}

// ParseErrorLog parses one or more 64-byte NVMe Error Information Log entries.
func ParseErrorLog(data []byte) ([]ErrorLogEntry, error) {
	if len(data)%64 != 0 {
		return nil, fmt.Errorf("NVMe error log: data length %d not a multiple of 64", len(data))
	}
	entries := make([]ErrorLogEntry, len(data)/64)
	for i := range entries {
		off := i * 64
		e := &entries[i]
		e.ErrorCount = binary.LittleEndian.Uint64(data[off : off+8])
		e.SQID = binary.LittleEndian.Uint16(data[off+8 : off+10])
		e.CMDID = binary.LittleEndian.Uint16(data[off+10 : off+12])
		e.StatusField = binary.LittleEndian.Uint16(data[off+12 : off+14])
		e.ParamErrorLocation = binary.LittleEndian.Uint16(data[off+14 : off+16])
		e.LBA = binary.LittleEndian.Uint64(data[off+16 : off+24])
		e.NSID = binary.LittleEndian.Uint32(data[off+24 : off+28])
		e.VendorSpecific = data[off+28]
	}
	return entries, nil
}

// le128lo64 reads a 128-bit little-endian integer and returns the lower 64 bits.
func le128lo64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b[:8])
}
