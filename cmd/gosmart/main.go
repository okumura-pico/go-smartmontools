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

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

// exitTimeout is the exit code used when the command times out.
const exitTimeout = 2

func main() {
	printer.Version = "gosmart " + version

	list := flag.Bool("l", false, "List available SMART field names")
	timeoutSec := flag.Int("t", 5, "Timeout in seconds (0 = no limit)")
	flag.IntVar(timeoutSec, "timeout", 5, "Timeout in seconds (0 = no limit)")

	flag.Usage = usage
	flag.Parse()

	var fn func() error
	switch {
	case *list && flag.NArg() == 1:
		path := flag.Arg(0)
		fn = func() error { return runList(path) }
	case !*list && flag.NArg() == 2:
		path, field := flag.Arg(0), flag.Arg(1)
		fn = func() error { return runGet(path, field) }
	case !*list && flag.NArg() == 1:
		path := flag.Arg(0)
		fn = func() error { return run(path) }
	default:
		usage()
		os.Exit(1)
	}

	os.Exit(runWithTimeout(fn, *timeoutSec))
}

// runWithTimeout runs fn in a goroutine and enforces timeoutSec.
// Returns the appropriate exit code (0 = OK, 1 = error, exitTimeout = timed out).
//
// ioctl calls cannot be interrupted mid-flight, so on timeout we exit the
// process immediately; the OS will reclaim the blocked goroutine.
func runWithTimeout(fn func() error, timeoutSec int) int {
	errCh := make(chan error, 1)
	go func() { errCh <- fn() }()

	if timeoutSec == 0 {
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
	fmt.Fprintf(os.Stderr, "Usage: gosmart [options] <device>\n")
	fmt.Fprintf(os.Stderr, "       gosmart [options] -l <device>\n")
	fmt.Fprintf(os.Stderr, "       gosmart [options] <device> <field>\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -l                     List available SMART field names\n")
	fmt.Fprintf(os.Stderr, "  -t, --timeout <secs>   Timeout in seconds (default 5, 0 = no limit)\n")
}

func isNVMePath(path string) (bool, error) {
	ok, err := device.IsNVMe(path)
	if err != nil {
		return false, fmt.Errorf("detect device type: %w", err)
	}
	return ok, nil
}

// --- full output ---

func run(path string) error {
	nvmeDev, err := isNVMePath(path)
	if err != nil {
		return err
	}
	if nvmeDev {
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

	idData, err := dev.Identify()
	if err != nil {
		return fmt.Errorf("IDENTIFY DEVICE: %w", err)
	}
	id, err := ata.ParseIdentify(idData)
	if err != nil {
		return err
	}

	svData, err := dev.SmartReadData()
	if err != nil {
		return fmt.Errorf("SMART READ DATA: %w", err)
	}
	sv, err := ata.ParseSmartValues(svData)
	if err != nil {
		return err
	}

	stData, err := dev.SmartReadThresholds()
	if err != nil {
		return fmt.Errorf("SMART READ THRESHOLDS: %w", err)
	}
	st, err := ata.ParseSmartThresholds(stData)
	if err != nil {
		return err
	}

	smartFailing, err := dev.SmartStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: SMART STATUS: %v\n", err)
	}

	elData, err := dev.SmartReadLog(ata.SmartReadLogAddrError)
	var el *ata.SmartErrorLog
	if err == nil {
		el, _ = ata.ParseSmartErrorLog(elData)
	}

	slData, err := dev.SmartReadLog(ata.SmartReadLogAddrSelfTest)
	var sl *ata.SmartSelfTestLog
	if err == nil {
		sl, _ = ata.ParseSmartSelfTestLog(slData)
	}

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

	icData, err := dev.IdentifyController()
	if err != nil {
		return fmt.Errorf("NVMe Identify Controller: %w", err)
	}
	ic, err := nvme.ParseIdentifyController(icData)
	if err != nil {
		return err
	}

	slData, err := dev.GetLogPage(0x02, 512)
	var sl *nvme.SmartLog
	if err == nil {
		sl, _ = nvme.ParseSmartLog(slData)
	}

	elData, err := dev.GetLogPage(0x01, 16*64)
	var errEntries []nvme.ErrorLogEntry
	if err == nil {
		errEntries, _ = nvme.ParseErrorLog(elData)
	}

	printer.PrintNVMeAll(os.Stdout, path, ic, sl, errEntries)
	return nil
}

// --- list mode ---

func runList(path string) error {
	nvmeDev, err := isNVMePath(path)
	if err != nil {
		return err
	}
	if nvmeDev {
		return listNVMe(path)
	}
	return listATA(path)
}

func listATA(path string) error {
	dev, err := device.OpenATA(path)
	if err != nil {
		return fmt.Errorf("open ATA device: %w", err)
	}
	defer dev.Close()

	idData, err := dev.Identify()
	if err != nil {
		return fmt.Errorf("IDENTIFY DEVICE: %w", err)
	}
	id, err := ata.ParseIdentify(idData)
	if err != nil {
		return err
	}

	svData, err := dev.SmartReadData()
	if err != nil {
		return fmt.Errorf("SMART READ DATA: %w", err)
	}
	sv, err := ata.ParseSmartValues(svData)
	if err != nil {
		return err
	}

	for _, name := range sv.AttrNames(id.RPM) {
		fmt.Println(name)
	}
	return nil
}

func listNVMe(path string) error {
	dev, err := device.OpenNVMe(path)
	if err != nil {
		return fmt.Errorf("open NVMe device: %w", err)
	}
	defer dev.Close()

	slData, err := dev.GetLogPage(0x02, 512)
	if err != nil {
		return fmt.Errorf("NVMe SMART log: %w", err)
	}
	sl, err := nvme.ParseSmartLog(slData)
	if err != nil {
		return err
	}

	for _, name := range sl.FieldNames() {
		fmt.Println(name)
	}
	return nil
}

// --- get mode ---

func runGet(path, field string) error {
	nvmeDev, err := isNVMePath(path)
	if err != nil {
		return err
	}
	if nvmeDev {
		return getNVMe(path, field)
	}
	return getATA(path, field)
}

func getATA(path, field string) error {
	dev, err := device.OpenATA(path)
	if err != nil {
		return fmt.Errorf("open ATA device: %w", err)
	}
	defer dev.Close()

	idData, err := dev.Identify()
	if err != nil {
		return fmt.Errorf("IDENTIFY DEVICE: %w", err)
	}
	id, err := ata.ParseIdentify(idData)
	if err != nil {
		return err
	}

	svData, err := dev.SmartReadData()
	if err != nil {
		return fmt.Errorf("SMART READ DATA: %w", err)
	}
	sv, err := ata.ParseSmartValues(svData)
	if err != nil {
		return err
	}

	a := sv.AttrByName(field, id.RPM)
	if a == nil {
		return fmt.Errorf("field not found: %s", field)
	}
	fmt.Println(a.RawValue48())
	return nil
}

func getNVMe(path, field string) error {
	dev, err := device.OpenNVMe(path)
	if err != nil {
		return fmt.Errorf("open NVMe device: %w", err)
	}
	defer dev.Close()

	slData, err := dev.GetLogPage(0x02, 512)
	if err != nil {
		return fmt.Errorf("NVMe SMART log: %w", err)
	}
	sl, err := nvme.ParseSmartLog(slData)
	if err != nil {
		return err
	}

	val, ok := sl.FieldValue(field)
	if !ok {
		return fmt.Errorf("field not found: %s", field)
	}
	fmt.Println(val)
	return nil
}
