package ata

import (
	"strings"
	"testing"
)

// makeIdentifyData returns a 512-byte IDENTIFY DEVICE buffer with the given fields set.
// All unspecified bytes are zero.
func makeIdentifyData(model, serial, firmware string, totalSectors uint64,
	smartSupported, smartEnabled bool, rpm uint16) []byte {

	data := make([]byte, 512)

	// Helper: write ATA string (byte-pair swapped) into data at byte offset off.
	writeATAStr := func(off, maxLen int, s string) {
		padded := s
		for len(padded) < maxLen {
			padded += " "
		}
		padded = padded[:maxLen]
		for i := 0; i+1 < len(padded); i += 2 {
			data[off+i] = padded[i+1]
			data[off+i+1] = padded[i]
		}
	}

	// Word 10–19 → bytes 20–39: serial (20 bytes)
	writeATAStr(20, 20, serial)
	// Word 23–26 → bytes 46–53: firmware (8 bytes)
	writeATAStr(46, 8, firmware)
	// Word 27–46 → bytes 54–93: model (40 bytes)
	writeATAStr(54, 40, model)

	// Word 60–61 (bytes 120–123): 28-bit LBA total sectors
	data[120] = byte(totalSectors)
	data[121] = byte(totalSectors >> 8)
	data[122] = byte(totalSectors >> 16)
	data[123] = byte(totalSectors >> 24)

	// Word 82 (bytes 164–165): bit 0 = SMART supported
	if smartSupported {
		data[164] |= 0x01
	}
	// Word 83 (bytes 166–167): bit 14 must be 1, bit 15 must be 0 for valid
	data[166] |= 0x40

	// Word 85 (bytes 170–171): bit 0 = SMART enabled
	if smartEnabled {
		data[170] |= 0x01
	}

	// Word 217 (bytes 434–435): nominal media rotation rate
	data[434] = byte(rpm)
	data[435] = byte(rpm >> 8)

	return data
}

func TestParseIdentify_BasicFields(t *testing.T) {
	const (
		model    = "Test SSD 500GB"
		serial   = "SN12345678"
		firmware = "FW1.0"
	)
	// 500 GB SSD: 976773168 sectors × 512 bytes ≈ 500 GB
	totalSectors := uint64(976773168)
	data := makeIdentifyData(model, serial, firmware, totalSectors, true, true, 1)

	id, err := ParseIdentify(data)
	if err != nil {
		t.Fatalf("ParseIdentify: %v", err)
	}

	if id.ModelNumber != model {
		t.Errorf("ModelNumber: got %q, want %q", id.ModelNumber, model)
	}
	if id.SerialNumber != serial {
		t.Errorf("SerialNumber: got %q, want %q", id.SerialNumber, serial)
	}
	if !strings.HasPrefix(id.FirmwareVersion, "FW1.0") {
		t.Errorf("FirmwareVersion: got %q, want prefix %q", id.FirmwareVersion, "FW1.0")
	}
	if !id.SMARTSupported {
		t.Error("SMARTSupported: want true")
	}
	if !id.SMARTEnabled {
		t.Error("SMARTEnabled: want true")
	}
	if id.RPM != 1 {
		t.Errorf("RPM: got %d, want 1 (SSD)", id.RPM)
	}
	wantCap := totalSectors * 512
	if id.UserCapacity != wantCap {
		t.Errorf("UserCapacity: got %d, want %d", id.UserCapacity, wantCap)
	}
}

func TestParseIdentify_HDDRotationRate(t *testing.T) {
	data := makeIdentifyData("HDD 2TB", "HDD001", "FIRM", 3907029168, true, true, 7200)
	id, err := ParseIdentify(data)
	if err != nil {
		t.Fatalf("ParseIdentify: %v", err)
	}
	if id.RPM != 7200 {
		t.Errorf("RPM: got %d, want 7200", id.RPM)
	}
}

func TestParseIdentify_SMARTDisabled(t *testing.T) {
	data := makeIdentifyData("Drive X", "SN000", "FW0", 1000000, true, false, 1)
	id, err := ParseIdentify(data)
	if err != nil {
		t.Fatalf("ParseIdentify: %v", err)
	}
	if id.SMARTEnabled {
		t.Error("SMARTEnabled: want false")
	}
}

func TestParseIdentify_WrongSize(t *testing.T) {
	_, err := ParseIdentify(make([]byte, 256))
	if err == nil {
		t.Error("expected error for wrong size")
	}
}

func TestATAString_SwapsPairs(t *testing.T) {
	// ATA strings have byte pairs swapped relative to memory order.
	// "AB" is stored as [B, A] in memory.
	b := []byte{'B', 'A', 'D', 'C', ' ', ' '}
	got := ataString(b)
	if got != "ABCD" {
		t.Errorf("ataString: got %q, want %q", got, "ABCD")
	}
}

func TestHighestBit(t *testing.T) {
	cases := []struct {
		in   uint16
		want uint16
	}{
		{0x0000, 0},
		{0x0001, 0},
		{0x0080, 7},
		{0x8000, 15},
		{0x01f0, 8},
	}
	for _, c := range cases {
		got := highestBit(c.in)
		if got != c.want {
			t.Errorf("highestBit(0x%04x) = %d, want %d", c.in, got, c.want)
		}
	}
}
