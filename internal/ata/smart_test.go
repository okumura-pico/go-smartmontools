package ata

import (
	"encoding/binary"
	"testing"
)

// makeSmartValuesData builds a 512-byte SMART READ DATA buffer with the given attributes.
// attrs: slice of (id, flags, current, worst, raw48) tuples.
func makeSmartValuesData(revNum uint16, attrs []testAttr) []byte {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint16(data[0:2], revNum)
	for i, a := range attrs {
		if i >= NumAttributes {
			break
		}
		off := 2 + i*12
		data[off] = a.id
		binary.LittleEndian.PutUint16(data[off+1:off+3], a.flags)
		data[off+3] = a.current
		data[off+4] = a.worst
		// 6-byte raw value in little-endian
		binary.LittleEndian.PutUint64(data[off+5:off+5+6+2], a.raw) // safe: next attr overwrites extra bytes
		// Actually write only 6 bytes:
		for j := 0; j < 6; j++ {
			data[off+5+j] = byte(a.raw >> (8 * uint(j)))
		}
	}
	// Fix checksum: byte 511 = -(sum of bytes 0–510)
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum
	return data
}

type testAttr struct {
	id      uint8
	flags   uint16
	current uint8
	worst   uint8
	raw     uint64
}

func TestParseSmartValues_Basic(t *testing.T) {
	attrs := []testAttr{
		{id: 1, flags: AttrFlagPrefailure | AttrFlagOnline, current: 100, worst: 100, raw: 0},
		{id: 9, flags: AttrFlagOnline, current: 98, worst: 98, raw: 1765},
		{id: 194, flags: AttrFlagOnline, current: 73, worst: 47, raw: 27},
	}
	data := makeSmartValuesData(1, attrs)

	sv, err := ParseSmartValues(data)
	if err != nil {
		t.Fatalf("ParseSmartValues: %v", err)
	}

	if sv.RevNumber != 1 {
		t.Errorf("RevNumber: got %d, want 1", sv.RevNumber)
	}
	if sv.Attributes[0].ID != 1 {
		t.Errorf("Attributes[0].ID: got %d, want 1", sv.Attributes[0].ID)
	}
	if sv.Attributes[0].Current != 100 {
		t.Errorf("Attributes[0].Current: got %d, want 100", sv.Attributes[0].Current)
	}
	if sv.Attributes[1].ID != 9 {
		t.Errorf("Attributes[1].ID: got %d, want 9", sv.Attributes[1].ID)
	}
	if sv.Attributes[1].RawValue48() != 1765 {
		t.Errorf("Attributes[1].RawValue48(): got %d, want 1765", sv.Attributes[1].RawValue48())
	}
	if sv.Attributes[2].Worst != 47 {
		t.Errorf("Attributes[2].Worst: got %d, want 47", sv.Attributes[2].Worst)
	}
}

func TestParseSmartValues_ChecksumError(t *testing.T) {
	data := makeSmartValuesData(1, nil)
	data[511] ^= 0xff // corrupt checksum
	_, err := ParseSmartValues(data)
	if err == nil {
		t.Error("expected checksum error")
	}
}

func TestParseSmartValues_WrongSize(t *testing.T) {
	_, err := ParseSmartValues(make([]byte, 100))
	if err == nil {
		t.Error("expected size error")
	}
}

func TestAttribute_IsPrefailure(t *testing.T) {
	a := Attribute{Flags: AttrFlagPrefailure}
	if !a.IsPrefailure() {
		t.Error("IsPrefailure: want true")
	}
	a.Flags = AttrFlagOnline
	if a.IsPrefailure() {
		t.Error("IsPrefailure: want false")
	}
}

func TestAttribute_IsOnline(t *testing.T) {
	a := Attribute{Flags: AttrFlagOnline | AttrFlagPrefailure}
	if !a.IsOnline() {
		t.Error("IsOnline: want true")
	}
	a.Flags = AttrFlagPrefailure
	if a.IsOnline() {
		t.Error("IsOnline: want false")
	}
}

func TestAttribute_RawValue48(t *testing.T) {
	cases := []struct {
		raw  [6]byte
		want uint64
	}{
		{[6]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00}, 1},
		{[6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0xffffffffffff},
		{[6]byte{0xe5, 0x06, 0x00, 0x00, 0x00, 0x00}, 1765},
	}
	for _, c := range cases {
		a := Attribute{Raw: c.raw}
		if got := a.RawValue48(); got != c.want {
			t.Errorf("RawValue48(%v) = %d, want %d", c.raw, got, c.want)
		}
	}
}

func TestParseSmartThresholds_Basic(t *testing.T) {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	// Set threshold for attribute slot 0: id=1, threshold=10
	data[2] = 1   // id
	data[3] = 10  // threshold
	// Fix checksum
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum

	st, err := ParseSmartThresholds(data)
	if err != nil {
		t.Fatalf("ParseSmartThresholds: %v", err)
	}
	if st.RevNumber != 1 {
		t.Errorf("RevNumber: got %d, want 1", st.RevNumber)
	}
	if st.Thresholds[0] != 10 {
		t.Errorf("Thresholds[0]: got %d, want 10", st.Thresholds[0])
	}
}

func TestValidateChecksum_Valid(t *testing.T) {
	data := make([]byte, 512)
	data[0] = 0x55
	data[1] = 0xaa
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum
	if !validateChecksum(data) {
		t.Error("validateChecksum: want true for valid data")
	}
}

func TestValidateChecksum_Invalid(t *testing.T) {
	data := make([]byte, 512)
	data[0] = 0x01 // sum != 0
	if validateChecksum(data) {
		t.Error("validateChecksum: want false for invalid data")
	}
}

func TestParseSelfTestLog_NoEntries(t *testing.T) {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	// Fix checksum
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum

	sl, err := ParseSmartSelfTestLog(data)
	if err != nil {
		t.Fatalf("ParseSmartSelfTestLog: %v", err)
	}
	if sl.RevNumber != 1 {
		t.Errorf("RevNumber: got %d, want 1", sl.RevNumber)
	}
	for i, e := range sl.Entries {
		if e.TestNumber != 0 {
			t.Errorf("Entries[%d].TestNumber: got %d, want 0", i, e.TestNumber)
		}
	}
}

func TestParseSelfTestLog_OneEntry(t *testing.T) {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint16(data[0:2], 1)

	// Entry 0 at offset 2: TestNumber=1, Status=0x00, Timestamp=100h
	data[2] = 0x01 // TestNumber (short offline)
	data[3] = 0x00 // SelfTestStatus: result=0 (completed OK), remaining=0
	binary.LittleEndian.PutUint16(data[4:6], 100) // TimestampHours

	// Fix checksum
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum

	sl, err := ParseSmartSelfTestLog(data)
	if err != nil {
		t.Fatalf("ParseSmartSelfTestLog: %v", err)
	}
	if sl.Entries[0].TestNumber != 1 {
		t.Errorf("Entries[0].TestNumber: got %d, want 1", sl.Entries[0].TestNumber)
	}
	if sl.Entries[0].TimestampHours != 100 {
		t.Errorf("Entries[0].TimestampHours: got %d, want 100", sl.Entries[0].TimestampHours)
	}
	if sl.Entries[0].SelfTestResult() != 0 {
		t.Errorf("SelfTestResult: got %d, want 0", sl.Entries[0].SelfTestResult())
	}
}

func TestParseErrorLog_NoErrors(t *testing.T) {
	data := make([]byte, 512)
	data[0] = 1 // RevNumber
	// ATAErrorCount at offset 452 = 0
	// Fix checksum
	var sum uint8
	for i := 0; i < 511; i++ {
		sum += data[i]
	}
	data[511] = -sum

	el, err := ParseSmartErrorLog(data)
	if err != nil {
		t.Fatalf("ParseSmartErrorLog: %v", err)
	}
	if el.ATAErrorCount != 0 {
		t.Errorf("ATAErrorCount: got %d, want 0", el.ATAErrorCount)
	}
}

func TestParseSelectiveSelfTestLog(t *testing.T) {
	data := make([]byte, 512)
	binary.LittleEndian.PutUint16(data[0:2], 1) // RevNumber

	// Span 0: MinLBA=0, MaxLBA=0, Status=0
	// (all zeros already)

	sl, err := ParseSmartSelectiveSelfTestLog(data)
	if err != nil {
		t.Fatalf("ParseSmartSelectiveSelfTestLog: %v", err)
	}
	if sl.RevNumber != 1 {
		t.Errorf("RevNumber: got %d, want 1", sl.RevNumber)
	}
	for i, span := range sl.Spans {
		if span.MinLBA != 0 || span.MaxLBA != 0 {
			t.Errorf("Spans[%d]: got min=%d max=%d, want 0", i, span.MinLBA, span.MaxLBA)
		}
	}
}
