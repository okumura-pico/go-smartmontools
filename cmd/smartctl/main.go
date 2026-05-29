// Command smartctl reads and prints SMART information from storage devices.
// Only the --all (-a) mode is implemented.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/okumura-pico/go-smartmontools/internal/ata"
	"github.com/okumura-pico/go-smartmontools/internal/device"
	"github.com/okumura-pico/go-smartmontools/internal/nvme"
	"github.com/okumura-pico/go-smartmontools/internal/printer"
)

func main() {
	all := flag.Bool("a", false, "Show all SMART information")
	flag.BoolVar(all, "all", false, "Show all SMART information (long form)")
	flag.Usage = usage
	flag.Parse()

	if !*all || flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

	path := flag.Arg(0)
	if err := run(path); err != nil {
		fmt.Fprintf(os.Stderr, "smartctl: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: smartctl -a <device>\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -a, --all    Show all SMART information\n")
}

func run(path string) error {
	// Try ATA first, then NVMe based on device naming convention.
	if isNVMe(path) {
		return runNVMe(path)
	}
	return runATA(path)
}

func isNVMe(path string) bool {
	// Heuristic: /dev/nvme* devices are NVMe.
	if len(path) >= 9 && path[:9] == "/dev/nvme" {
		return true
	}
	return false
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
