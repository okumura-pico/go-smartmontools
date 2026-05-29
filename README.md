# go-smartmontools

A Go port of [smartmontools](https://www.smartmontools.org/) scoped to
`smartctl --all` — reading and printing all SMART information from storage
devices. Supports ATA/SATA and NVMe drives on Linux.

## Features

- ATA/SATA drives: IDENTIFY DEVICE, SMART health status, attributes with
  thresholds, error log, self-test log, selective self-test log
- NVMe drives: Identify Controller, SMART/Health log, error information log
- Output compatible with the original `smartctl --all` format
- Single static binary, no runtime dependencies

## Requirements

- Linux (uses `HDIO_DRIVE_CMD` for ATA, `NVME_IOCTL_ADMIN_CMD` for NVMe)
- Root privileges to access `/dev/sd*` or `/dev/nvme*`
- Go 1.22 or later to build

## Installation

```sh
go install github.com/okumura-pico/go-smartmontools/cmd/smartctl@latest
```

Or build from source:

```sh
git clone https://github.com/okumura-pico/go-smartmontools.git
cd go-smartmontools
go build -o smartctl ./cmd/smartctl
```

## Usage

```
smartctl -a <device>
smartctl --all <device>
```

### ATA/SATA drive

```sh
sudo smartctl -a /dev/sda
```

Example output:

```
go-smartmontools 0.1
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

General SMART Values:
...

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
...
```

### NVMe drive

```sh
sudo smartctl -a /dev/nvme0
```

Example output:

```
go-smartmontools 0.1
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
Data Units Read:                    ...
Data Units Written:                 ...
Power On Hours:                     100
...

Error Information (NVMe Log 0x01, 16 of the most recent entries)
No Errors Logged
```

## Project structure

```
cmd/smartctl/        CLI entry point
internal/
  ata/               ATA data structures, SMART parsing, attribute names
  nvme/              NVMe data structures and log parsing
  device/            Device interface + Linux ioctl implementation
  printer/           smartctl-compatible text output
```

## License

GNU GPL Version 2 — see [COPYING](COPYING).
