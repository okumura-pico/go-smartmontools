// Command gosmart reads and prints SMART information from storage devices.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/okumura-pico/go-smartmontools/internal/ata"
	"github.com/okumura-pico/go-smartmontools/internal/device"
	"github.com/okumura-pico/go-smartmontools/internal/nvme"
	"github.com/okumura-pico/go-smartmontools/internal/printer"
)

// exitTimeout is the exit code used when the command times out.
const exitTimeout = 2

func main() {
	timeoutSec := flag.Int("t", 5, "Timeout in seconds (0 = no limit)")
	flag.IntVar(timeoutSec, "timeout", 5, "Timeout in seconds (0 = no limit)")

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

	path := flag.Arg(0)
	os.Exit(runWithTimeout(path, *timeoutSec))
}

// runWithTimeout runs run() in a goroutine and enforces timeoutSec.
// Returns the appropriate exit code (0 = OK, 1 = error, exitTimeout = timed out).
//
// ioctl calls cannot be interrupted mid-flight, so on timeout we exit the
// process immediately; the OS will reclaim the blocked goroutine.
func runWithTimeout(path string, timeoutSec int) int {
	errCh := make(chan error, 1)
	go func() { errCh <- run(path) }()

	if timeoutSec == 0 {
		// No timeout: wait indefinitely.
		if err := <-errCh; err != nil {
			fmt.Fprintf(os.Stderr, "gosmart: %v\n", err)
			return 1
		}
		return 0
	}

	select {
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "gosmart: %v\n", err)
			return 1
		}
		return 0
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		fmt.Fprintf(os.Stderr, "gosmart: timeout after %ds — device not responding\n", timeoutSec)
		return exitTimeout
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: gosmart [options] <device>\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -t, --timeout <secs>   Timeout in seconds (default 5, 0 = no limit)\n")
}

func run(path string) error {
	nvme, err := device.IsNVMe(path)
	if err != nil {
		return fmt.Errorf("detect device type: %w", err)
	}
	if nvme {
		return runNVMe(path)
	}
	return runATA(path)
}

func runATA(path string) error {
	dev, err := device.OpenATA(path)
	if err != nil {
		return fmt.Errorf("open ATA device: %w", err)
	}
	defer dev.Close()

	// IDENTIFY DEVICE
	idData, err := dev.Identify()
	if err != nil {
		return fmt.Errorf("IDENTIFY DEVICE: %w", err)
	}
	id, err := ata.ParseIdentify(idData)
	if err != nil {
		return err
	}

	// SMART READ DATA
	svData, err := dev.SmartReadData()
	if err != nil {
		return fmt.Errorf("SMART READ DATA: %w", err)
	}
	sv, err := ata.ParseSmartValues(svData)
	if err != nil {
		return err
	}

	// SMART READ THRESHOLDS
	stData, err := dev.SmartReadThresholds()
	if err != nil {
		return fmt.Errorf("SMART READ THRESHOLDS: %w", err)
	}
	st, err := ata.ParseSmartThresholds(stData)
	if err != nil {
		return err
	}

	// SMART RETURN STATUS
	smartFailing, err := dev.SmartStatus()
	if err != nil {
		// Non-fatal: just report unknown
		fmt.Fprintf(os.Stderr, "Warning: SMART STATUS: %v\n", err)
	}

	// Error Log (log 0x01)
	elData, err := dev.SmartReadLog(ata.SmartReadLogAddrError)
	var el *ata.SmartErrorLog
	if err == nil {
		el, _ = ata.ParseSmartErrorLog(elData)
	}

	// Self-Test Log (log 0x06)
	slData, err := dev.SmartReadLog(ata.SmartReadLogAddrSelfTest)
	var sl *ata.SmartSelfTestLog
	if err == nil {
		sl, _ = ata.ParseSmartSelfTestLog(slData)
	}

	// Selective Self-Test Log (log 0x09)
	selData, err := dev.SmartReadLog(ata.SmartReadLogAddrSelective)
	var selSl *ata.SmartSelectiveSelfTestLog
	if err == nil {
		selSl, _ = ata.ParseSmartSelectiveSelfTestLog(selData)
	}

	printer.PrintATAAll(os.Stdout, path, id, sv, st, smartFailing, el, sl, selSl)
	return nil
}

func runNVMe(path string) error {
	dev, err := device.OpenNVMe(path)
	if err != nil {
		return fmt.Errorf("open NVMe device: %w", err)
	}
	defer dev.Close()

	// Identify Controller
	icData, err := dev.IdentifyController()
	if err != nil {
		return fmt.Errorf("NVMe Identify Controller: %w", err)
	}
	ic, err := nvme.ParseIdentifyController(icData)
	if err != nil {
		return err
	}

	// SMART/Health Log (log 0x02, 512 bytes)
	slData, err := dev.GetLogPage(0x02, 512)
	var sl *nvme.SmartLog
	if err == nil {
		sl, _ = nvme.ParseSmartLog(slData)
	}

	// Error Information Log (log 0x01, 16 entries × 64 bytes)
	elData, err := dev.GetLogPage(0x01, 16*64)
	var errEntries []nvme.ErrorLogEntry
	if err == nil {
		errEntries, _ = nvme.ParseErrorLog(elData)
	}

	printer.PrintNVMeAll(os.Stdout, path, ic, sl, errEntries)
	return nil
}
