package ata

import (
	"encoding/binary"
	"fmt"
)

// Attribute holds one parsed SMART attribute entry (12 bytes in the 512-byte response).
type Attribute struct {
	ID      uint8
	Flags   uint16
	Current uint8
	Worst   uint8
	// Raw is the 6-byte vendor-specific raw value in little-endian order.
	Raw [6]byte
}

// RawValue48 interprets the 6 raw bytes as a 48-bit little-endian unsigned integer.
func (a *Attribute) RawValue48() uint64 {
	return uint64(a.Raw[0]) |
		uint64(a.Raw[1])<<8 |
		uint64(a.Raw[2])<<16 |
		uint64(a.Raw[3])<<24 |
		uint64(a.Raw[4])<<32 |
		uint64(a.Raw[5])<<40
}

// IsPrefailure reports whether this is a pre-failure (vs advisory) attribute.
func (a *Attribute) IsPrefailure() bool { return a.Flags&AttrFlagPrefailure != 0 }

// IsOnline reports whether the attribute is updated during normal operation.
func (a *Attribute) IsOnline() bool { return a.Flags&AttrFlagOnline != 0 }

// SmartValues holds the parsed result of SMART READ DATA (512 bytes).
type SmartValues struct {
	RevNumber  uint16
	Attributes [NumAttributes]Attribute

	OfflineDataCollectionStatus uint8
	SelfTestExecStatus          uint8
	TotalTimeOfflineSeconds     uint16
	OfflineDataCollectionCap    uint8
	SmartCapability             uint16
	ErrorlogCapability          uint8
	ShortTestCompletionTime     uint8   // minutes
	ExtendTestCompletionTimeB   uint8   // minutes (0xFF → use Word)
	ConveyanceTestCompletionTime uint8  // minutes
	ExtendTestCompletionTimeW   uint16  // minutes (when B == 0xFF)
}

// ParseSmartValues parses a 512-byte SMART READ DATA response.
func ParseSmartValues(data []byte) (*SmartValues, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("smart values: expected 512 bytes, got %d", len(data))
	}
	if !validateChecksum(data) {
		return nil, fmt.Errorf("smart values: checksum error")
	}

	sv := &SmartValues{}
	sv.RevNumber = le16(data, 0)

	for i := 0; i < NumAttributes; i++ {
		off := 2 + i*12
		a := &sv.Attributes[i]
		a.ID = data[off]
		a.Flags = le16(data, off+1)
		a.Current = data[off+3]
		a.Worst = data[off+4]
		copy(a.Raw[:], data[off+5:off+11])
	}

	sv.OfflineDataCollectionStatus = data[362]
	sv.SelfTestExecStatus = data[363]
	sv.TotalTimeOfflineSeconds = le16(data, 364)
	sv.OfflineDataCollectionCap = data[367]
	sv.SmartCapability = le16(data, 368)
	sv.ErrorlogCapability = data[370]
	sv.ShortTestCompletionTime = data[372]
	sv.ExtendTestCompletionTimeB = data[373]
	sv.ConveyanceTestCompletionTime = data[374]
	sv.ExtendTestCompletionTimeW = le16(data, 375)

	return sv, nil
}

// SmartThresholds holds the parsed result of SMART READ THRESHOLDS (512 bytes).
type SmartThresholds struct {
	RevNumber  uint16
	Thresholds [NumAttributes]uint8
}

// ParseSmartThresholds parses a 512-byte SMART READ THRESHOLDS response.
func ParseSmartThresholds(data []byte) (*SmartThresholds, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("smart thresholds: expected 512 bytes, got %d", len(data))
	}
	if !validateChecksum(data) {
		return nil, fmt.Errorf("smart thresholds: checksum error")
	}

	st := &SmartThresholds{}
	st.RevNumber = le16(data, 0)
	for i := 0; i < NumAttributes; i++ {
		off := 2 + i*12
		st.Thresholds[i] = data[off+1]
	}
	return st, nil
}

// ErrorLogEntry holds one entry from the SMART error log (5 commands + 1 error record = 90 bytes).
type ErrorLogEntry struct {
	// Commands is the sequence of up to 5 commands leading to the error.
	Commands [5]ErrorCommand
	// Error describes the resulting error.
	Error ErrorRecord
}

// ErrorCommand is one ATA command descriptor (12 bytes).
type ErrorCommand struct {
	DeviceControlReg uint8
	FeaturesReg      uint8
	SectorCount      uint8
	SectorNumber     uint8
	CylinderLow      uint8
	CylinderHigh     uint8
	DriveHead        uint8
	CommandReg       uint8
	// TimestampMs is power-on time in milliseconds when command was issued.
	TimestampMs uint32
}

// ErrorRecord is one ATA error descriptor (30 bytes).
type ErrorRecord struct {
	ErrorReg      uint8
	SectorCount   uint8
	SectorNumber  uint8
	CylinderLow   uint8
	CylinderHigh  uint8
	DriveHead     uint8
	Status        uint8
	ExtendedError [19]byte
	State         uint8
	// TimestampMin is power-on time in minutes when the error occurred.
	TimestampMin uint16
}

// SmartErrorLog holds the parsed SMART Summary Error Log (512 bytes, log address 0x01).
type SmartErrorLog struct {
	RevNumber      uint8
	LogPointer     uint8 // index of most recent entry (1-based, wraps at 5)
	Entries        [5]ErrorLogEntry
	ATAErrorCount  uint16 // total error count (may wrap)
}

// ParseSmartErrorLog parses a 512-byte SMART error log (log address 0x01).
func ParseSmartErrorLog(data []byte) (*SmartErrorLog, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("error log: expected 512 bytes, got %d", len(data))
	}
	if !validateChecksum(data) {
		return nil, fmt.Errorf("error log: checksum error")
	}

	el := &SmartErrorLog{}
	el.RevNumber = data[0]
	el.LogPointer = data[1]

	for i := 0; i < 5; i++ {
		off := 2 + i*90
		entry := &el.Entries[i]
		for c := 0; c < 5; c++ {
			cOff := off + c*12
			entry.Commands[c] = ErrorCommand{
				DeviceControlReg: data[cOff+0],
				FeaturesReg:      data[cOff+1],
				SectorCount:      data[cOff+2],
				SectorNumber:     data[cOff+3],
				CylinderLow:      data[cOff+4],
				CylinderHigh:     data[cOff+5],
				DriveHead:        data[cOff+6],
				CommandReg:       data[cOff+7],
				TimestampMs:      binary.LittleEndian.Uint32(data[cOff+8 : cOff+12]),
			}
		}
		eOff := off + 60
		err := &entry.Error
		err.ErrorReg = data[eOff+1]
		err.SectorCount = data[eOff+2]
		err.SectorNumber = data[eOff+3]
		err.CylinderLow = data[eOff+4]
		err.CylinderHigh = data[eOff+5]
		err.DriveHead = data[eOff+6]
		err.Status = data[eOff+7]
		copy(err.ExtendedError[:], data[eOff+8:eOff+27])
		err.State = data[eOff+27]
		err.TimestampMin = le16(data, eOff+28)
	}

	el.ATAErrorCount = le16(data, 452)
	return el, nil
}

// SelfTestEntry is one entry in the self-test log (24 bytes).
type SelfTestEntry struct {
	// TestType encodes the type of test in bits [6:0] and result in bits [7:4].
	TestNumber         uint8
	SelfTestStatus     uint8
	// TimestampHours is the drive power-on hours at test completion.
	TimestampHours     uint16
	FailureCheckpoint  uint8
	LBAFirstFailure    uint32
	VendorSpecific     [15]byte
}

// SelfTestResult decodes the status nibble.
func (e *SelfTestEntry) SelfTestResult() uint8 { return e.SelfTestStatus >> 4 }

// SelfTestPercRemaining returns percent of test remaining (0 when complete).
func (e *SelfTestEntry) SelfTestPercRemaining() uint8 { return e.SelfTestStatus & 0x0f }

// SmartSelfTestLog holds the parsed SMART Self-Test Log (512 bytes, log address 0x06).
type SmartSelfTestLog struct {
	RevNumber      uint16
	Entries        [21]SelfTestEntry
	MostRecentTest uint8
}

// ParseSmartSelfTestLog parses a 512-byte SMART self-test log (log address 0x06).
func ParseSmartSelfTestLog(data []byte) (*SmartSelfTestLog, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("self-test log: expected 512 bytes, got %d", len(data))
	}
	if !validateChecksum(data) {
		return nil, fmt.Errorf("self-test log: checksum error")
	}

	sl := &SmartSelfTestLog{}
	sl.RevNumber = le16(data, 0)

	for i := 0; i < 21; i++ {
		off := 2 + i*24
		e := &sl.Entries[i]
		e.TestNumber = data[off+0]
		e.SelfTestStatus = data[off+1]
		e.TimestampHours = le16(data, off+2)
		e.FailureCheckpoint = data[off+4]
		e.LBAFirstFailure = binary.LittleEndian.Uint32(data[off+5 : off+9])
		copy(e.VendorSpecific[:], data[off+9:off+24])
	}
	sl.MostRecentTest = data[506]
	return sl, nil
}

// SelectiveTestSpan is one span in the selective self-test log.
type SelectiveTestSpan struct {
	MinLBA uint64
	MaxLBA uint64
	Status uint16
}

// SmartSelectiveSelfTestLog holds parsed data from log address 0x09 (512 bytes).
type SmartSelectiveSelfTestLog struct {
	RevNumber uint16
	Spans     [5]SelectiveTestSpan
	Flags     uint16
}

// ParseSmartSelectiveSelfTestLog parses a 512-byte selective self-test log (log 0x09).
func ParseSmartSelectiveSelfTestLog(data []byte) (*SmartSelectiveSelfTestLog, error) {
	if len(data) != 512 {
		return nil, fmt.Errorf("selective self-test log: expected 512 bytes, got %d", len(data))
	}

	sl := &SmartSelectiveSelfTestLog{}
	sl.RevNumber = le16(data, 0)

	for i := 0; i < 5; i++ {
		off := 2 + i*20
		sl.Spans[i].MinLBA = binary.LittleEndian.Uint64(data[off : off+8])
		sl.Spans[i].MaxLBA = binary.LittleEndian.Uint64(data[off+8 : off+16])
		sl.Spans[i].Status = le16(data, off+16)
	}
	sl.Flags = le16(data, 102)
	return sl, nil
}

// AttrNames returns the names of all present SMART attributes in order.
func (sv *SmartValues) AttrNames(rpm int) []string {
	var names []string
	for _, a := range sv.Attributes {
		if a.ID != 0 {
			names = append(names, AttrName(a.ID, rpm))
		}
	}
	return names
}

// AttrByName returns a pointer to the first attribute whose name matches, or nil.
func (sv *SmartValues) AttrByName(name string, rpm int) *Attribute {
	for i := range sv.Attributes {
		a := &sv.Attributes[i]
		if a.ID != 0 && AttrName(a.ID, rpm) == name {
			return a
		}
	}
	return nil
}

// --- helpers ---

func le16(data []byte, off int) uint16 {
	return binary.LittleEndian.Uint16(data[off : off+2])
}

// validateChecksum verifies the ATA data-structure checksum (byte 511).
// The sum of all 512 bytes must be 0x00.
func validateChecksum(data []byte) bool {
	var sum uint8
	for _, b := range data {
		sum += b
	}
	return sum == 0
}
