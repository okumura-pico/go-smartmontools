package printer

import (
	"fmt"
	"io"

	"github.com/okumura-pico/go-smartmontools/internal/nvme"
)

// PrintNVMeAll writes the equivalent of "smartctl --all" output for a NVMe device to w.
func PrintNVMeAll(w io.Writer, path string,
	ic *nvme.IdentifyController,
	sl *nvme.SmartLog,
	errLog []nvme.ErrorLogEntry) {

	printHeader(w, path)
	printNVMeInfo(w, ic)
	printNVMeSmartSection(w, sl, errLog)
}

func printNVMeInfo(w io.Writer, ic *nvme.IdentifyController) {
	fmt.Fprintf(w, "=== START OF INFORMATION SECTION ===\n")
	fmt.Fprintf(w, "Model Number:     %s\n", ic.ModelNumber)
	fmt.Fprintf(w, "Serial Number:    %s\n", ic.SerialNumber)
	fmt.Fprintf(w, "Firmware Version: %s\n", ic.FWRevision)
	if ic.TotalCapacityBytes > 0 {
		fmt.Fprintf(w, "Total NVM Capacity: %s bytes [%s]\n",
			formatComma(ic.TotalCapacityBytes), formatCapacity(ic.TotalCapacityBytes))
	}
	major := (ic.Version >> 16) & 0xffff
	minor := (ic.Version >> 8) & 0xff
	fmt.Fprintf(w, "NVMe Version:     %d.%d\n", major, minor)
	fmt.Fprintln(w)
}

func printNVMeSmartSection(w io.Writer, sl *nvme.SmartLog, errLog []nvme.ErrorLogEntry) {
	fmt.Fprintf(w, "=== START OF READ SMART DATA SECTION ===\n")

	if sl == nil {
		fmt.Fprintf(w, "SMART/Health Information not available\n")
		return
	}

	// Overall health
	if sl.CriticalWarning != 0 {
		fmt.Fprintf(w, "SMART overall-health self-assessment test result: FAILED!\n")
		printNVMeCriticalWarnings(w, sl.CriticalWarning)
	} else {
		fmt.Fprintf(w, "SMART overall-health self-assessment test result: PASSED\n")
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "SMART/Health Information (NVMe Log 0x02)\n")
	fmt.Fprintf(w, "Critical Warning:                   0x%02x\n", sl.CriticalWarning)
	fmt.Fprintf(w, "Temperature:                        %d Celsius\n", sl.TemperatureCelsius())
	fmt.Fprintf(w, "Available Spare:                    %d%%\n", sl.AvailableSpare)
	fmt.Fprintf(w, "Available Spare Threshold:          %d%%\n", sl.SpareThreshold)
	fmt.Fprintf(w, "Percentage Used:                    %d%%\n", sl.PercentageUsed)
	fmt.Fprintf(w, "Data Units Read:                    %s [%s]\n",
		formatComma(sl.DataUnitsRead), formatCapacity(sl.DataUnitsRead*512_000))
	fmt.Fprintf(w, "Data Units Written:                 %s [%s]\n",
		formatComma(sl.DataUnitsWritten), formatCapacity(sl.DataUnitsWritten*512_000))
	fmt.Fprintf(w, "Host Read Commands:                 %s\n", formatComma(sl.HostReads))
	fmt.Fprintf(w, "Host Write Commands:                %s\n", formatComma(sl.HostWrites))
	fmt.Fprintf(w, "Controller Busy Time:               %s\n", formatComma(sl.ControllerBusyTime))
	fmt.Fprintf(w, "Power Cycles:                       %s\n", formatComma(sl.PowerCycles))
	fmt.Fprintf(w, "Power On Hours:                     %s\n", formatComma(sl.PowerOnHours))
	fmt.Fprintf(w, "Unsafe Shutdowns:                   %s\n", formatComma(sl.UnsafeShutdowns))
	fmt.Fprintf(w, "Media and Data Integrity Errors:    %s\n", formatComma(sl.MediaErrors))
	fmt.Fprintf(w, "Error Information Log Entries:      %s\n", formatComma(sl.NumErrLogEntries))
	if sl.WarningTempTime > 0 {
		fmt.Fprintf(w, "Warning  Comp. Temperature Time:    %d\n", sl.WarningTempTime)
	}
	if sl.CriticalCompTime > 0 {
		fmt.Fprintf(w, "Critical Comp. Temperature Time:    %d\n", sl.CriticalCompTime)
	}
	for i, ts := range sl.TempSensor {
		if ts > 0 {
			fmt.Fprintf(w, "Temperature Sensor %d:               %d Celsius\n", i+1, int(ts)-273)
		}
	}
	fmt.Fprintln(w)

	printNVMeErrorLog(w, errLog)
}

func printNVMeCriticalWarnings(w io.Writer, cw uint8) {
	if cw&0x01 != 0 {
		fmt.Fprintf(w, "  Available spare capacity has fallen below threshold.\n")
	}
	if cw&0x02 != 0 {
		fmt.Fprintf(w, "  Temperature is above or below threshold.\n")
	}
	if cw&0x04 != 0 {
		fmt.Fprintf(w, "  NVM subsystem reliability has been degraded.\n")
	}
	if cw&0x08 != 0 {
		fmt.Fprintf(w, "  Media is in read-only mode.\n")
	}
	if cw&0x10 != 0 {
		fmt.Fprintf(w, "  Volatile memory backup device has failed.\n")
	}
}

func printNVMeErrorLog(w io.Writer, entries []nvme.ErrorLogEntry) {
	fmt.Fprintf(w, "Error Information (NVMe Log 0x01, 16 of the most recent entries)\n")

	hasErrors := false
	for _, e := range entries {
		if e.ErrorCount > 0 {
			hasErrors = true
			break
		}
	}
	if !hasErrors {
		fmt.Fprintf(w, "No Errors Logged\n")
		return
	}

	fmt.Fprintf(w, "Num   ErrCount  SqId   CmdId  StatusField  PELocation          LBA  NSID    VS\n")
	for i, e := range entries {
		if e.ErrorCount == 0 {
			continue
		}
		fmt.Fprintf(w, "  %d %10d  0x%04x 0x%04x       0x%04x      0x%04x  0x%016x 0x%08x 0x%02x\n",
			i, e.ErrorCount, e.SQID, e.CMDID, e.StatusField,
			e.ParamErrorLocation, e.LBA, e.NSID, e.VendorSpecific)
	}
}
