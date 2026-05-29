# Development Guide

## Environment setup

This project uses [mise](https://mise.jdx.dev/) to manage the Go toolchain.

```sh
# Install mise (if not already installed)
curl https://mise.run | sh

# Install Go (version defined in mise.toml)
mise install
```

Verify:

```sh
mise exec go -- go version
```

## Build

```sh
# Linux binary (current platform)
mise exec go -- go build -o gosmart ./cmd/gosmart

# Cross-compile for Windows (from Linux)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  mise exec go -- go build -o gosmart.exe ./cmd/gosmart

# With version string embedded
mise exec go -- go build -ldflags "-X main.version=v1.2.3" -o gosmart ./cmd/gosmart
```

## Test

All tests use synthetic binary fixtures. No real hardware or root privileges needed.

```sh
mise exec go -- go test ./...

# Verbose
mise exec go -- go test -v ./...

# Single package
mise exec go -- go test -v ./internal/ata/
```

32 tests across 3 packages:

| Package              | Tests |
|----------------------|-------|
| `internal/ata`       | 19    |
| `internal/nvme`      | 7     |
| `internal/printer`   | 9     |

## Architecture

```
cmd/gosmart/
  main.go               Entry point, flag parsing, timeout handling

internal/
  device/
    device.go           ATADevice and NVMeDevice interfaces
    linux.go            Linux ioctl implementation (HDIO_DRIVE_CMD, NVME_IOCTL_ADMIN_CMD)
    windows.go          Windows IOCTL implementation (IOCTL_SMART_*, IOCTL_STORAGE_QUERY_PROPERTY)
    detect_linux.go     IsNVMe: path heuristic (/dev/nvme*)
    detect_windows.go   IsNVMe: STORAGE_DEVICE_DESCRIPTOR bus type query

  ata/
    ata.go              Log address constants
    identify.go         ParseIdentify: IDENTIFY DEVICE (512 bytes → IdentifyInfo)
    smart.go            ParseSmartValues, ParseSmartThresholds, ParseSmartErrorLog,
                        ParseSmartSelfTestLog, ParseSmartSelectiveSelfTestLog
    attr.go             AttrName: SMART attribute ID → human-readable name

  nvme/
    nvme.go             ParseIdentifyController, ParseSmartLog, ParseErrorLog

  printer/
    ata.go              PrintATAAll: smartctl --all compatible text output for ATA
    nvme.go             PrintNVMeAll: smartctl --all compatible text output for NVMe
```

### Data flow

```
main
 └─ device.IsNVMe(path)
     ├─ [ATA]  device.OpenATA → ATADevice
     │           ├─ Identify()              → ata.ParseIdentify      → IdentifyInfo
     │           ├─ SmartReadData()         → ata.ParseSmartValues   → SmartValues
     │           ├─ SmartReadThresholds()   → ata.ParseSmartThresholds
     │           ├─ SmartStatus()                                    → bool
     │           ├─ SmartReadLog(0x01)      → ata.ParseSmartErrorLog
     │           ├─ SmartReadLog(0x06)      → ata.ParseSmartSelfTestLog
     │           └─ SmartReadLog(0x09)      → ata.ParseSmartSelectiveSelfTestLog
     │                                                 ↓
     │                                       printer.PrintATAAll
     │
     └─ [NVMe] device.OpenNVMe → NVMeDevice
                 ├─ IdentifyController()    → nvme.ParseIdentifyController → IdentifyController
                 ├─ GetLogPage(0x02, 512)   → nvme.ParseSmartLog
                 └─ GetLogPage(0x01, 1024)  → nvme.ParseErrorLog
                                                       ↓
                                             printer.PrintNVMeAll
```

### Timeout handling

`ioctl` calls can block indefinitely on unresponsive devices and cannot be
cancelled. `runWithTimeout` runs the device query in a goroutine and races it
against `time.After`. On timeout, the process exits immediately (exit code 2)
and the OS reclaims the blocked goroutine.

### Platform-specific code

Build tags (`//go:build linux` / `//go:build windows`) keep platform code
isolated. Both platforms implement the same `ATADevice` and `NVMeDevice`
interfaces, so the rest of the codebase is platform-agnostic.

| OS      | ATA                        | NVMe                              |
|---------|----------------------------|-----------------------------------|
| Linux   | `HDIO_DRIVE_CMD` ioctl     | `NVME_IOCTL_ADMIN_CMD` ioctl      |
| Windows | `IOCTL_SMART_RCV_DRIVE_DATA` / `IOCTL_SMART_SEND_DRIVE_COMMAND` | `IOCTL_STORAGE_QUERY_PROPERTY` with `StorageDeviceProtocolSpecificProperty` |

## ATA binary format notes

All SMART commands return a 512-byte sector. Byte 511 is a checksum: the sum
of all 512 bytes must equal 0x00.

ATA strings (model, serial, firmware) have their byte pairs swapped within
each 16-bit word — see `ataString()` in `internal/ata/identify.go`.

## Adding a new platform

1. Create `internal/device/<os>.go` with `//go:build <os>` and implement
   `OpenATA`, `OpenNVMe`, `ATADevice`, and `NVMeDevice`.
2. Create `internal/device/detect_<os>.go` with `IsNVMe(path string) (bool, error)`.
3. Add a matrix entry in `.github/workflows/release.yml`.

## Releasing

Tag a commit with a semantic version and push:

```sh
git tag v1.2.3
git push origin main --tags
```

GitHub Actions builds Linux and Windows amd64 binaries and publishes a
GitHub Release automatically. The version string is embedded via
`-ldflags "-X main.version=v1.2.3"`.
