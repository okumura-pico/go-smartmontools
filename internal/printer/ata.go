// Package printer formats and prints smartctl --all compatible output.
package printer

import (
	"fmt"
	"io"

	"github.com/okumura-pico/go-smartmontools/internal/ata"
)

const version = "gosmart 0.1"

// PrintATAAll writes the equivalent of "smartctl --all" output for an ATA device to w.
func PrintATAAll(w io.Writer, path string, id *ata.IdentifyInfo, sv *ata.SmartValues,
	st *ata.SmartThresholds, smartFailing bool,
	errLog *ata.SmartErrorLog, selfTestLog *ata.SmartSelfTestLog,
	selTestLog *ata.SmartSelectiveSelfTestLog) {

	printHeader(w, path)
	printATAInfo(w, id)
	printSMARTSection(w, id, sv, st, smartFailing, errLog, selfTestLog, selTestLog)
}

func printHeader(w io.Writer, path string) {
	fmt.Fprintf(w, "%s\n", version)
	fmt.Fprintf(w, "Copyright (C) 2002-26, smartmontools contributors\n\n")
}

func printATAInfo(w io.Writer, id *ata.IdentifyInfo) {
	fmt.Fprintf(w, "=== START OF INFORMATION SECTION ===\n")
	fmt.Fprintf(w, "Device Model:     %s\n", id.ModelNumber)
	fmt.Fprintf(w, "Serial Number:    %s\n", id.SerialNumber)
	fmt.Fprintf(w, "Firmware Version: %s\n", id.FirmwareVersion)

	cap := id.UserCapacity
	fmt.Fprintf(w, "User Capacity:    %s bytes [%s]\n",
		formatComma(cap), formatCapacity(cap))

	if id.LogicalSectorSize == id.PhysicalSectorSize {
		fmt.Fprintf(w, "Sector Size:      %d bytes logical/physical\n", id.LogicalSectorSize)
	} else {
		fmt.Fprintf(w, "Sector Sizes:     %d bytes logical, %d bytes physical\n",
			id.LogicalSectorSize, id.PhysicalSectorSize)
	}

	switch id.RPM {
	case 0:
		// unknown
	case 1:
		fmt.Fprintf(w, "Rotation Rate:    Solid State Device\n")
	default:
		fmt.Fprintf(w, "Rotation Rate:    %d rpm\n", id.RPM)
	}

	if id.SMARTSupported {
		fmt.Fprintf(w, "SMART support is: Available - device has SMART capability.\n")
		if id.SMARTEnabled {
			fmt.Fprintf(w, "SMART support is: Enabled\n")
		} else {
			fmt.Fprintf(w, "SMART support is: Disabled\n")
		}
	} else {
		fmt.Fprintf(w, "SMART support is: Unavailable - device lacks SMART capability.\n")
	}
	fmt.Fprintln(w)
}

func printSMARTSection(w io.Writer, id *ata.IdentifyInfo, sv *ata.SmartValues,
	st *ata.SmartThresholds, smartFailing bool,
	errLog *ata.SmartErrorLog, selfTestLog *ata.SmartSelfTestLog,
	selTestLog *ata.SmartSelectiveSelfTestLog) {

	fmt.Fprintf(w, "=== START OF READ SMART DATA SECTION ===\n")

	if smartFailing {
		fmt.Fprintf(w, "SMART overall-health self-assessment test result: FAILED!\n")
		fmt.Fprintf(w, "Drive failure expected in less than 24 hours. SAVE ALL DATA.\n")
	} else {
		fmt.Fprintf(w, "SMART overall-health self-assessment test result: PASSED\n")
	}
	fmt.Fprintln(w)

	if sv != nil {
		printGeneralValues(w, sv)
		fmt.Fprintln(w)

		if st != nil {
			printAttributes(w, sv, st, id.RPM)
			fmt.Fprintln(w)
		}
	}

	if errLog != nil {
		printErrorLog(w, errLog)
		fmt.Fprintln(w)
	}

	if selfTestLog != nil {
		printSelfTestLog(w, selfTestLog)
		fmt.Fprintln(w)
	}

	if selTestLog != nil {
		printSelectiveSelfTestLog(w, selTestLog)
	}
}

func printGeneralValues(w io.Writer, sv *ata.SmartValues) {
	fmt.Fprintf(w, "General SMART Values:\n")

	// Offline data collection status
	offStatus := sv.OfflineDataCollectionStatus
	fmt.Fprintf(w, "Offline data collection status:  (0x%02x)\t", offStatus)
	switch offStatus & 0x7f {
	case 0x00:
		fmt.Fprintf(w, "Offline data collection activity\n\t\t\t\t\twas never started.\n")
	case 0x02:
		fmt.Fprintf(w, "Offline data collection activity\n\t\t\t\t\twas completed without error.\n")
	case 0x04:
		fmt.Fprintf(w, "Offline data collection activity\n\t\t\t\t\twas suspended by an interrupting command from host.\n")
	case 0x05:
		fmt.Fprintf(w, "Offline data collection activity\n\t\t\t\t\twas aborted by an interrupting command from host.\n")
	case 0x06:
		fmt.Fprintf(w, "Offline data collection activity\n\t\t\t\t\twas aborted by the device with a fatal error.\n")
	default:
		fmt.Fprintf(w, "Vendor specific.\n")
	}
	if offStatus&0x80 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tAuto Offline Data Collection: Enabled.\n")
	} else {
		fmt.Fprintf(w, "\t\t\t\t\tAuto Offline Data Collection: Disabled.\n")
	}

	// Self-test execution status
	stStatus := sv.SelfTestExecStatus
	stResult := stStatus >> 4
	stPerc := stStatus & 0x0f
	fmt.Fprintf(w, "Self-test execution status:      (%4d)\t", stStatus)
	switch stResult {
	case 0:
		fmt.Fprintf(w, "The previous self-test routine completed\n\t\t\t\t\twithout error or no self-test has ever\n\t\t\t\t\tbeen run.\n")
	case 1:
		fmt.Fprintf(w, "The self-test routine was aborted by\n\t\t\t\t\tthe host.\n")
	case 2:
		fmt.Fprintf(w, "The self-test routine was interrupted\n\t\t\t\t\tby the host with a hard or soft reset.\n")
	case 3:
		fmt.Fprintf(w, "A fatal error or unknown test error\n\t\t\t\t\toccurred while the device was executing\n\t\t\t\t\tits self-test routine and the device\n\t\t\t\t\twas unable to complete the self-test\n\t\t\t\t\troutine.\n")
	case 4:
		fmt.Fprintf(w, "The previous self-test completed having\n\t\t\t\t\ta test element that failed and the test\n\t\t\t\t\telement that failed is not known.\n")
	case 5:
		fmt.Fprintf(w, "The previous self-test completed having\n\t\t\t\t\tthe electrical element of the test\n\t\t\t\t\tfailed.\n")
	case 6:
		fmt.Fprintf(w, "The previous self-test completed having\n\t\t\t\t\tthe servo (and/or seek) element of the\n\t\t\t\t\ttest failed.\n")
	case 7:
		fmt.Fprintf(w, "The previous self-test completed having\n\t\t\t\t\tthe read element of the test failed.\n")
	case 8:
		fmt.Fprintf(w, "The previous self-test completed having\n\t\t\t\t\ta test element that failed and the\n\t\t\t\t\tdevice is suspected of having handling\n\t\t\t\t\tdamage.\n")
	case 15:
		fmt.Fprintf(w, "Self-test routine in progress...\n\t\t\t\t\t%d%% of test remaining.\n", stPerc*10)
	default:
		fmt.Fprintf(w, "Unknown/reserved test status.\n")
	}

	fmt.Fprintf(w, "Total time to complete Offline\ndata collection:\t\t(%5d) seconds.\n",
		sv.TotalTimeOfflineSeconds)

	// Offline data collection capabilities
	cap := sv.OfflineDataCollectionCap
	fmt.Fprintf(w, "Offline data collection\ncapabilities:\t\t\t (0x%02x)\t", cap)
	if cap&0x01 != 0 {
		fmt.Fprintf(w, "SMART execute Offline immediate.\n")
	} else {
		fmt.Fprintf(w, "No SMART execute Offline immediate.\n")
	}
	if cap&0x04 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tAuto Offline data collection on/off support.\n")
	} else {
		fmt.Fprintf(w, "\t\t\t\t\tNo Auto Offline data collection support.\n")
	}
	if cap&0x08 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tSuspend Offline collection upon new command.\n")
	} else {
		fmt.Fprintf(w, "\t\t\t\t\tAbort Offline collection upon new command.\n")
	}
	if cap&0x10 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tOffline surface scan supported.\n")
	}
	if cap&0x20 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tSelf-test supported.\n")
	}
	if cap&0x40 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tConveyance Self-test supported.\n")
	}
	if cap&0x80 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tSelective Self-test supported.\n")
	}

	// SMART capabilities
	scap := sv.SmartCapability
	fmt.Fprintf(w, "SMART capabilities:            (0x%04x)\t", scap)
	if scap&0x01 != 0 {
		fmt.Fprintf(w, "Saves SMART data before entering\n\t\t\t\t\tpower-saving mode.\n")
	}
	if scap&0x02 != 0 {
		fmt.Fprintf(w, "\t\t\t\t\tSupports SMART auto save timer.\n")
	}

	// Error log capability
	elcap := sv.ErrorlogCapability
	fmt.Fprintf(w, "Error logging capability:        (0x%02x)\t", elcap)
	if elcap&0x01 != 0 {
		fmt.Fprintf(w, "Error logging supported.\n")
		fmt.Fprintf(w, "\t\t\t\t\tGeneral Purpose Logging supported.\n")
	} else {
		fmt.Fprintf(w, "Error logging NOT supported.\n")
	}

	// Test completion times
	fmt.Fprintf(w, "Short self-test routine\nrecommended polling time:\t (%4d) minutes.\n",
		sv.ShortTestCompletionTime)

	extTime := int(sv.ExtendTestCompletionTimeB)
	if sv.ExtendTestCompletionTimeB == 0xff {
		extTime = int(sv.ExtendTestCompletionTimeW)
	}
	fmt.Fprintf(w, "Extended self-test routine\nrecommended polling time:\t (%4d) minutes.\n", extTime)

	if sv.ConveyanceTestCompletionTime > 0 {
		fmt.Fprintf(w, "Conveyance self-test routine\nrecommended polling time:\t (%4d) minutes.\n",
			sv.ConveyanceTestCompletionTime)
	}
}

func printAttributes(w io.Writer, sv *ata.SmartValues, st *ata.SmartThresholds, rpm int) {
	fmt.Fprintf(w, "SMART Attributes Data Structure revision number: %d\n", sv.RevNumber)
	fmt.Fprintf(w, "Vendor Specific SMART Attributes with Thresholds:\n")
	fmt.Fprintf(w, "ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE\n")

	// Build threshold lookup by position (attributes and thresholds are parallel arrays).
	for i := 0; i < ata.NumAttributes; i++ {
		a := &sv.Attributes[i]
		if a.ID == 0 {
			continue
		}

		threshold := st.Thresholds[i]
		name := ata.AttrName(a.ID, rpm)

		typeStr := "Old_age"
		if a.IsPrefailure() {
			typeStr = "Pre-fail"
		}
		updStr := "Always"
		if !a.IsOnline() {
			updStr = "Offline"
		}

		whenFailed := "-"
		if threshold > 0 && a.Current <= threshold {
			whenFailed = "FAILING_NOW"
		} else if threshold > 0 && a.Worst <= threshold {
			whenFailed = "In_the_past"
		}

		raw := a.RawValue48()
		fmt.Fprintf(w, "%3d %-24s 0x%04x   %3d   %3d   %3d    %-10s%-9s%-12s %d\n",
			a.ID, name, a.Flags,
			a.Current, a.Worst, threshold,
			typeStr, updStr, whenFailed, raw)
	}
}

func printErrorLog(w io.Writer, el *ata.SmartErrorLog) {
	fmt.Fprintf(w, "SMART Error Log Version: %d\n", el.RevNumber)
	if el.ATAErrorCount == 0 {
		fmt.Fprintf(w, "No Errors Logged\n")
		return
	}

	fmt.Fprintf(w, "ATA Error Count: %d\n", el.ATAErrorCount)
	fmt.Fprintf(w, "\tDCR  FR  SC  SN  CL  CH  DH  CMD     TIMESTAMP  Err-Desc\n")

	count := int(el.ATAErrorCount)
	if count > 5 {
		count = 5
	}
	ptr := int(el.LogPointer)
	for n := 0; n < count; n++ {
		// Entries are in circular order; most recent first.
		idx := (ptr - 1 - n + 5) % 5
		entry := &el.Entries[idx]
		errRec := &entry.Error
		fmt.Fprintf(w, "Error %d occurred at disk power-on lifetime: %d hours\n",
			el.ATAErrorCount-uint16(n), errRec.TimestampMin/60)
		for c := 4; c >= 0; c-- {
			cmd := &entry.Commands[c]
			if cmd.CommandReg == 0 {
				continue
			}
			fmt.Fprintf(w, "\t0x%02x 0x%02x 0x%02x 0x%02x 0x%02x 0x%02x 0x%02x 0x%02x %10d  -\n",
				cmd.DeviceControlReg, cmd.FeaturesReg, cmd.SectorCount,
				cmd.SectorNumber, cmd.CylinderLow, cmd.CylinderHigh,
				cmd.DriveHead, cmd.CommandReg, cmd.TimestampMs)
		}
		fmt.Fprintf(w, "\tError: 0x%02x  LBA = 0x%02x%02x%02x%02x  Count = 0x%02x\n",
			errRec.ErrorReg,
			errRec.CylinderHigh, errRec.CylinderLow,
			errRec.SectorNumber, errRec.SectorCount,
			errRec.SectorCount)
	}
}

func printSelfTestLog(w io.Writer, sl *ata.SmartSelfTestLog) {
	fmt.Fprintf(w, "SMART Self-test log structure revision number %d\n", sl.RevNumber)

	hasEntries := false
	for _, e := range sl.Entries {
		if e.TestNumber != 0 {
			hasEntries = true
			break
		}
	}
	if !hasEntries {
		fmt.Fprintf(w, "No self-tests have been logged.  [To run self-tests, use: smartctl -t]\n")
		return
	}

	fmt.Fprintf(w, "Num  Test_Description    Status                  Remaining  LifeTime(hours)  LBA_of_first_error\n")
	for i, e := range sl.Entries {
		if e.TestNumber == 0 {
			continue
		}
		testType := selfTestTypeName(e.TestNumber)
		statusStr := selfTestStatusString(e.SelfTestResult())
		remaining := e.SelfTestPercRemaining() * 10
		lba := "-"
		if e.SelfTestResult() != 0 && e.LBAFirstFailure != 0xffffffff {
			lba = fmt.Sprintf("0x%08x", e.LBAFirstFailure)
		}
		fmt.Fprintf(w, "#%2d  %-19s %-26s %3d%%   %8d         %s\n",
			i+1, testType, statusStr, remaining, e.TimestampHours, lba)
	}
}

func selfTestTypeName(num uint8) string {
	switch num & 0x7f {
	case 0x01:
		return "Short offline"
	case 0x02:
		return "Extended offline"
	case 0x03:
		return "Conveyance offline"
	case 0x04:
		return "Selective offline"
	case 0x81:
		return "Short captive"
	case 0x82:
		return "Extended captive"
	case 0x83:
		return "Conveyance captive"
	case 0x84:
		return "Selective captive"
	default:
		return "Unknown"
	}
}

func selfTestStatusString(result uint8) string {
	switch result {
	case 0:
		return "Completed without error"
	case 1:
		return "Aborted by host"
	case 2:
		return "Interrupted (host reset)"
	case 3:
		return "Fatal or unknown error"
	case 4:
		return "Completed: unknown failure"
	case 5:
		return "Completed: electrical failure"
	case 6:
		return "Completed: servo/seek failure"
	case 7:
		return "Completed: read failure"
	case 8:
		return "Completed: handling damage"
	case 15:
		return "Self-test in progress"
	default:
		return "Unknown status"
	}
}

func printSelectiveSelfTestLog(w io.Writer, sl *ata.SmartSelectiveSelfTestLog) {
	fmt.Fprintf(w, "SMART Selective self-test log data structure revision number %d\n", sl.RevNumber)
	fmt.Fprintf(w, " SPAN  MIN_LBA  MAX_LBA  CURRENT_TEST_STATUS\n")
	for i, span := range sl.Spans {
		status := "Not_testing"
		if span.Status != 0 {
			status = fmt.Sprintf("0x%04x", span.Status)
		}
		fmt.Fprintf(w, "    %d %8d %8d  %s\n", i+1, span.MinLBA, span.MaxLBA, status)
	}
	fmt.Fprintf(w, "Selective self-test flags (0x%x):\n", sl.Flags)
	if sl.Flags&0x01 != 0 {
		fmt.Fprintf(w, "  After scanning selected spans, read-scan remainder of disk.\n")
	} else {
		fmt.Fprintf(w, "  After scanning selected spans, do NOT read-scan remainder of disk.\n")
	}
	fmt.Fprintf(w, "If Selective self-test is pending on power-up, resume after 0 minute delay.\n")
}

// formatComma formats a uint64 with comma-separated thousands.
func formatComma(n uint64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// formatCapacity formats bytes as human-readable SI capacity (GB, TB).
func formatCapacity(bytes uint64) string {
	const (
		gb = 1_000_000_000
		tb = 1_000_000_000_000
		pb = 1_000_000_000_000_000
	)
	switch {
	case bytes >= pb:
		return fmt.Sprintf("%.1f PB", float64(bytes)/pb)
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/tb)
	case bytes >= gb:
		return fmt.Sprintf("%.0f GB", float64(bytes)/gb)
	default:
		return fmt.Sprintf("%d MB", bytes/1_000_000)
	}
}
