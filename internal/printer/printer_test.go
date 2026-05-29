package printer

import (
	"strings"
	"testing"

	"github.com/okumura-pico/go-smartmontools/internal/ata"
	"github.com/okumura-pico/go-smartmontools/internal/nvme"
)

func TestFormatComma(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{500107862016, "500,107,862,016"},
	}
	for _, c := range cases {
		got := formatComma(c.in)
		if got != c.want {
			t.Errorf("formatComma(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCapacity(t *testing.T) {
	cases := []struct {
		bytes   uint64
		contain string
	}{
		{500_000_000_000, "500 GB"},
		{1_000_000_000_000, "1.0 TB"},
		{2_000_000_000_000, "2.0 TB"},
		{256_000_000, "256 MB"},
	}
	for _, c := range cases {
		got := formatCapacity(c.bytes)
		if !strings.Contains(got, c.contain) {
			t.Errorf("formatCapacity(%d) = %q, want to contain %q", c.bytes, got, c.contain)
		}
	}
}

func TestPrintATAAll_OutputStructure(t *testing.T) {
	var sb strings.Builder
	PrintATAAll(&sb, "/dev/sda",
		newTestIdentifyInfo(1),
		newTestSmartValues(),
		newTestSmartThresholds(),
		false, nil, nil, nil)

	out := sb.String()
	for _, want := range []string{
		"INFORMATION SECTION",
		"Device Model:",
		"Test SSD 500GB",
		"SMART DATA SECTION",
		"SMART overall-health",
		"PASSED",
		"General SMART Values",
		"SMART Attributes",
		"ID# ATTRIBUTE_NAME",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintATAAll_FailingStatus(t *testing.T) {
	var sb strings.Builder
	PrintATAAll(&sb, "/dev/sda",
		newTestIdentifyInfo(1),
		newTestSmartValues(),
		newTestSmartThresholds(),
		true, nil, nil, nil)
	if !strings.Contains(sb.String(), "FAILED") {
		t.Error("output missing FAILED for smartFailing=true")
	}
}

func TestPrintATAAll_SSDLabel(t *testing.T) {
	var sb strings.Builder
	id := newTestIdentifyInfo(1) // RPM=1 → SSD
	PrintATAAll(&sb, "/dev/sda", id, newTestSmartValues(), newTestSmartThresholds(), false, nil, nil, nil)
	if !strings.Contains(sb.String(), "Solid State Device") {
		t.Error("output missing 'Solid State Device' for SSD")
	}
}

func TestPrintATAAll_HDDLabel(t *testing.T) {
	var sb strings.Builder
	id := newTestIdentifyInfo(7200) // HDD 7200 RPM
	PrintATAAll(&sb, "/dev/sda", id, newTestSmartValues(), newTestSmartThresholds(), false, nil, nil, nil)
	if !strings.Contains(sb.String(), "7200 rpm") {
		t.Error("output missing '7200 rpm' for HDD")
	}
}

func TestPrintATAAll_AttributeLine(t *testing.T) {
	var sb strings.Builder
	PrintATAAll(&sb, "/dev/sda",
		newTestIdentifyInfo(1),
		newTestSmartValues(),
		newTestSmartThresholds(),
		false, nil, nil, nil)
	out := sb.String()
	// Attribute 9 = Power_On_Hours, raw=1765
	if !strings.Contains(out, "Power_On_Hours") {
		t.Error("output missing Power_On_Hours attribute")
	}
	if !strings.Contains(out, "1765") {
		t.Error("output missing raw value 1765 for Power_On_Hours")
	}
}

func TestPrintNVMeAll_OutputStructure(t *testing.T) {
	var sb strings.Builder
	PrintNVMeAll(&sb, "/dev/nvme0", newTestNVMeIC(), newTestNVMeSmartLog(), nil)

	out := sb.String()
	for _, want := range []string{
		"INFORMATION SECTION",
		"Model Number:",
		"Test NVMe 1TB",
		"SMART DATA SECTION",
		"SMART overall-health",
		"PASSED",
		"SMART/Health Information",
		"Temperature:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("NVMe output missing %q", want)
		}
	}
}

func TestPrintNVMeAll_CriticalWarning(t *testing.T) {
	var sb strings.Builder
	sl := newTestNVMeSmartLog()
	sl.CriticalWarning = 0x01 // spare below threshold
	PrintNVMeAll(&sb, "/dev/nvme0", newTestNVMeIC(), sl, nil)
	out := sb.String()
	if !strings.Contains(out, "FAILED") {
		t.Error("NVMe output missing FAILED for critical warning")
	}
}

// --- test fixture builders ---

func newTestIdentifyInfo(rpm int) *ata.IdentifyInfo {
	return &ata.IdentifyInfo{
		ModelNumber:        "Test SSD 500GB",
		SerialNumber:       "SN12345678",
		FirmwareVersion:    "FW1.0",
		UserCapacity:       500_107_862_016,
		LogicalSectorSize:  512,
		PhysicalSectorSize: 512,
		RPM:                rpm,
		SMARTSupported:     true,
		SMARTEnabled:       true,
	}
}

func newTestSmartValues() *ata.SmartValues {
	sv := &ata.SmartValues{RevNumber: 1}
	// Attribute 0: ID=9 (Power_On_Hours), online, old_age, current=98, worst=98, raw=1765
	sv.Attributes[0] = ata.Attribute{
		ID:      9,
		Flags:   ata.AttrFlagOnline,
		Current: 98,
		Worst:   98,
		Raw:     [6]byte{0xe5, 0x06, 0x00, 0x00, 0x00, 0x00}, // 1765 LE
	}
	// Attribute 1: ID=5 (Reallocated_Sector_Ct), prefailure
	sv.Attributes[1] = ata.Attribute{
		ID:      5,
		Flags:   ata.AttrFlagPrefailure | ata.AttrFlagOnline,
		Current: 100,
		Worst:   100,
	}
	sv.OfflineDataCollectionCap = 0x79
	sv.SmartCapability = 0x0003
	sv.ErrorlogCapability = 0x01
	sv.ShortTestCompletionTime = 2
	sv.ExtendTestCompletionTimeB = 0x4a // 74 minutes
	return sv
}

func newTestSmartThresholds() *ata.SmartThresholds {
	st := &ata.SmartThresholds{RevNumber: 1}
	// threshold for slot 0 (attr 9) = 0
	st.Thresholds[0] = 0
	// threshold for slot 1 (attr 5) = 10
	st.Thresholds[1] = 10
	return st
}

func newTestNVMeIC() *nvme.IdentifyController {
	return &nvme.IdentifyController{
		ModelNumber:        "Test NVMe 1TB",
		SerialNumber:       "NVMe001",
		FWRevision:         "FW2.0",
		Version:            (1 << 16) | (4 << 8),
		TotalCapacityBytes: 1_000_204_886_016,
		MaxPowerStates:     4,
	}
}

func newTestNVMeSmartLog() *nvme.SmartLog {
	return &nvme.SmartLog{
		CriticalWarning:   0,
		TemperatureKelvin: 300, // 27°C
		AvailableSpare:    100,
		SpareThreshold:    10,
		PercentageUsed:    5,
		PowerOnHours:      1765,
		DataUnitsRead:     1000,
		DataUnitsWritten:  2000,
	}
}
