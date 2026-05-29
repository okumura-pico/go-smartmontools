package nvme

import (
	"encoding/binary"
	"strings"
	"testing"
)

func makeIdentifyControllerData(model, serial, fw string, version uint32, totalCap uint64) []byte {
	data := make([]byte, 4096)

	// SN: bytes 4–23
	copy(data[4:24], padRight(serial, 20))
	// MN: bytes 24–63
	copy(data[24:64], padRight(model, 40))
	// FR: bytes 64–71
	copy(data[64:72], padRight(fw, 8))
	// VER: bytes 80–83
	binary.LittleEndian.PutUint32(data[80:84], version)
	// TNVMCAP: bytes 280–295 (lower 64 bits)
	binary.LittleEndian.PutUint64(data[280:288], totalCap)
	// NPSS: byte 263 (0-based)
	data[263] = 3 // 4 power states

	return data
}

func padRight(s string, n int) []byte {
	b := make([]byte, n)
	copy(b, s)
	for i := len(s); i < n; i++ {
		b[i] = ' '
	}
	return b
}

func TestParseIdentifyController_BasicFields(t *testing.T) {
	const (
		model   = "Test NVMe SSD 1TB"
		serial  = "NVMe001"
		fw      = "FW2.0"
		version = (1 << 16) | (4 << 8) // NVMe 1.4
		cap     = uint64(1_000_204_886_016) // 1 TB
	)

	data := makeIdentifyControllerData(model, serial, fw, version, cap)
	ic, err := ParseIdentifyController(data)
	if err != nil {
		t.Fatalf("ParseIdentifyController: %v", err)
	}

	if ic.ModelNumber != model {
		t.Errorf("ModelNumber: got %q, want %q", ic.ModelNumber, model)
	}
	if ic.SerialNumber != serial {
		t.Errorf("SerialNumber: got %q, want %q", ic.SerialNumber, serial)
	}
	if !strings.HasPrefix(ic.FWRevision, "FW2.0") {
		t.Errorf("FWRevision: got %q, want prefix %q", ic.FWRevision, "FW2.0")
	}
	if ic.Version != version {
		t.Errorf("Version: got 0x%08x, want 0x%08x", ic.Version, version)
	}
	if ic.TotalCapacityBytes != cap {
		t.Errorf("TotalCapacityBytes: got %d, want %d", ic.TotalCapacityBytes, cap)
	}
	if ic.MaxPowerStates != 4 {
		t.Errorf("MaxPowerStates: got %d, want 4", ic.MaxPowerStates)
	}
}

func TestParseIdentifyController_WrongSize(t *testing.T) {
	_, err := ParseIdentifyController(make([]byte, 512))
	if err == nil {
		t.Error("expected error for wrong size")
	}
}

func makeSmartLogData(critWarn uint8, tempK uint16, spare, spareThresh, percUsed uint8,
	dataUnitsRead, dataUnitsWritten, powerOnHours uint64) []byte {

	data := make([]byte, 512)
	data[0] = critWarn
	binary.LittleEndian.PutUint16(data[1:3], tempK)
	data[3] = spare
	data[4] = spareThresh
	data[5] = percUsed

	// 128-bit LE fields: store in lower 8 bytes
	binary.LittleEndian.PutUint64(data[32:40], dataUnitsRead)
	binary.LittleEndian.PutUint64(data[48:56], dataUnitsWritten)
	binary.LittleEndian.PutUint64(data[128:136], powerOnHours)

	return data
}

func TestParseSmartLog_Basic(t *testing.T) {
	const (
		critWarn   = uint8(0)
		tempK      = uint16(300) // 27°C
		spare      = uint8(100)
		spareThresh = uint8(10)
		percUsed   = uint8(5)
		dataRead   = uint64(1000)
		dataWrite  = uint64(2000)
		powOn      = uint64(1765)
	)

	data := makeSmartLogData(critWarn, tempK, spare, spareThresh, percUsed, dataRead, dataWrite, powOn)
	sl, err := ParseSmartLog(data)
	if err != nil {
		t.Fatalf("ParseSmartLog: %v", err)
	}

	if sl.CriticalWarning != critWarn {
		t.Errorf("CriticalWarning: got %d, want %d", sl.CriticalWarning, critWarn)
	}
	if sl.TemperatureCelsius() != 27 {
		t.Errorf("TemperatureCelsius: got %d, want 27", sl.TemperatureCelsius())
	}
	if sl.AvailableSpare != spare {
		t.Errorf("AvailableSpare: got %d, want %d", sl.AvailableSpare, spare)
	}
	if sl.SpareThreshold != spareThresh {
		t.Errorf("SpareThreshold: got %d, want %d", sl.SpareThreshold, spareThresh)
	}
	if sl.PercentageUsed != percUsed {
		t.Errorf("PercentageUsed: got %d, want %d", sl.PercentageUsed, percUsed)
	}
	if sl.DataUnitsRead != dataRead {
		t.Errorf("DataUnitsRead: got %d, want %d", sl.DataUnitsRead, dataRead)
	}
	if sl.DataUnitsWritten != dataWrite {
		t.Errorf("DataUnitsWritten: got %d, want %d", sl.DataUnitsWritten, dataWrite)
	}
	if sl.PowerOnHours != powOn {
		t.Errorf("PowerOnHours: got %d, want %d", sl.PowerOnHours, powOn)
	}
}

func TestParseSmartLog_WrongSize(t *testing.T) {
	_, err := ParseSmartLog(make([]byte, 128))
	if err == nil {
		t.Error("expected error for wrong size")
	}
}

func TestParseSmartLog_TemperatureCelsius_Zero(t *testing.T) {
	data := make([]byte, 512)
	// tempK = 0 → TemperatureCelsius should return 0
	sl, _ := ParseSmartLog(data)
	if sl.TemperatureCelsius() != 0 {
		t.Errorf("TemperatureCelsius with 0 Kelvin: got %d, want 0", sl.TemperatureCelsius())
	}
}

func TestParseErrorLog_Basic(t *testing.T) {
	// 2 entries (128 bytes)
	data := make([]byte, 128)

	// Entry 0: ErrorCount=5, SQID=1, CMDID=2, StatusField=0x200, LBA=0xdeadbeef
	binary.LittleEndian.PutUint64(data[0:8], 5)
	binary.LittleEndian.PutUint16(data[8:10], 1)
	binary.LittleEndian.PutUint16(data[10:12], 2)
	binary.LittleEndian.PutUint16(data[12:14], 0x200)
	binary.LittleEndian.PutUint64(data[16:24], 0xdeadbeef)
	// Entry 1: ErrorCount=0 (no error)

	entries, err := ParseErrorLog(data)
	if err != nil {
		t.Fatalf("ParseErrorLog: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries count: got %d, want 2", len(entries))
	}
	if entries[0].ErrorCount != 5 {
		t.Errorf("entries[0].ErrorCount: got %d, want 5", entries[0].ErrorCount)
	}
	if entries[0].SQID != 1 {
		t.Errorf("entries[0].SQID: got %d, want 1", entries[0].SQID)
	}
	if entries[0].LBA != 0xdeadbeef {
		t.Errorf("entries[0].LBA: got 0x%x, want 0xdeadbeef", entries[0].LBA)
	}
	if entries[1].ErrorCount != 0 {
		t.Errorf("entries[1].ErrorCount: got %d, want 0", entries[1].ErrorCount)
	}
}

func TestParseErrorLog_InvalidSize(t *testing.T) {
	_, err := ParseErrorLog(make([]byte, 100)) // not multiple of 64
	if err == nil {
		t.Error("expected error for invalid size")
	}
}

func TestLe128lo64(t *testing.T) {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b[0:8], 0x123456789abcdef0)
	binary.LittleEndian.PutUint64(b[8:16], 0xffffffffffffffff) // upper bits ignored
	got := le128lo64(b)
	if got != 0x123456789abcdef0 {
		t.Errorf("le128lo64: got 0x%x, want 0x123456789abcdef0", got)
	}
}
