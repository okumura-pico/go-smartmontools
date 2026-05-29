# go-smartmontools

A Go port of [smartmontools](https://www.smartmontools.org/) scoped to
reading and printing all SMART information from storage devices.
Supports ATA/SATA and NVMe drives on Linux and Windows.

## Features

- ATA/SATA drives: IDENTIFY DEVICE, SMART health status, attributes with
  thresholds, error log, self-test log, selective self-test log
- NVMe drives: Identify Controller, SMART/Health log, error information log
- Output compatible with `smartctl --all` format
- List available SMART field names (`-l`)
- Query a single field's raw value for scripting
- Single static binary, no runtime dependencies
- Linux and Windows support

## Installation

Download a pre-built binary from the [Releases](https://github.com/okumura-pico/go-smartmontools/releases) page.

## Usage

```
gosmart [options] <device>              Show all SMART information
gosmart [options] -l <device>          List available field names
gosmart [options] <device> <field>     Print raw value of a single field

Options:
  -l                     List available SMART field names
  -t, --timeout <secs>   Timeout in seconds (default 5, 0 = no limit)
```

### Linux

```sh
# Show all SMART information
sudo gosmart /dev/sda
sudo gosmart /dev/nvme0

# List available field names
sudo gosmart -l /dev/sda

# Get the raw value of a single field
sudo gosmart /dev/sda Power_On_Hours
sudo gosmart /dev/nvme0 Temperature

# Disable timeout (e.g. slow/old drive)
sudo gosmart -t 0 /dev/sda
```

### Windows (run as Administrator)

```powershell
# PhysicalDrive number from Disk Management
gosmart \\.\PhysicalDrive0
gosmart -l \\.\PhysicalDrive0
gosmart \\.\PhysicalDrive0 Power_On_Hours
```

## Example output

### ATA/SATA

```
gosmart v1.0.0
Copyright (C) 2002-26, smartmontools contributors

=== START OF INFORMATION SECTION ===
Device Model:     Samsung SSD 860 EVO 500GB
Serial Number:    S3YVNX0K123456
Firmware Version: RVT21B6Q
User Capacity:    500,107,862,016 bytes [500 GB]
Sector Size:      512 bytes logical/physical
Rotation Rate:    Solid State Device
SMART support is: Available - device has SMART capability.
SMART support is: Enabled

=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED

SMART Attributes Data Structure revision number: 1
Vendor Specific SMART Attributes with Thresholds:
ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
  5 Reallocated_Sector_Ct   0x0033   100   100   010    Pre-fail  Always       -       0
  9 Power_On_Hours           0x0032   098   098   000    Old_age   Always       -       1765
...

SMART Error Log Version: 1
No Errors Logged

SMART Self-test log structure revision number 1
No self-tests have been logged.

SMART Selective self-test log data structure revision number 1
 SPAN  MIN_LBA  MAX_LBA  CURRENT_TEST_STATUS
    1        0        0  Not_testing
```

### NVMe

```
gosmart v1.0.0
Copyright (C) 2002-26, smartmontools contributors

=== START OF INFORMATION SECTION ===
Model Number:     Samsung SSD 980 PRO 1TB
Serial Number:    S5GXNX0W123456
Firmware Version: 5B2QGXA7
Total NVM Capacity: 1,000,204,886,016 bytes [1.0 TB]
NVMe Version:     1.4

=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED

SMART/Health Information (NVMe Log 0x02)
Critical Warning:                   0x00
Temperature:                        38 Celsius
Available Spare:                    100%
Available Spare Threshold:          10%
Percentage Used:                    0%
Power On Hours:                     100
...

Error Information (NVMe Log 0x01, 16 of the most recent entries)
No Errors Logged
```

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for build instructions, architecture overview, and release process.

## License

GNU GPL Version 2 — see [COPYING](COPYING).
