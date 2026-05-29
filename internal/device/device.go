// Package device defines the interface for communicating with storage devices.
package device

import "errors"

// ErrUnsupported is returned when a command is not supported by the device.
var ErrUnsupported = errors.New("command not supported")

// ATADevice is the interface for issuing ATA commands to a storage device.
type ATADevice interface {
	// Identify issues the ATA IDENTIFY DEVICE command and returns 512 bytes.
	Identify() ([]byte, error)

	// SmartReadData issues SMART READ DATA and returns 512 bytes.
	SmartReadData() ([]byte, error)

	// SmartReadThresholds issues SMART READ THRESHOLDS and returns 512 bytes.
	SmartReadThresholds() ([]byte, error)

	// SmartStatus issues SMART RETURN STATUS.
	// Returns true if SMART status indicates a failure.
	SmartStatus() (failing bool, err error)

	// SmartReadLog issues SMART READ LOG SECTOR for the given log address.
	// Returns 512 bytes.
	SmartReadLog(logAddr uint8) ([]byte, error)

	// Path returns the device path string (e.g. "/dev/sda").
	Path() string

	Close() error
}

// NVMeDevice is the interface for issuing NVMe Admin Commands to a device.
type NVMeDevice interface {
	// IdentifyController issues NVMe Identify Controller and returns 4096 bytes.
	IdentifyController() ([]byte, error)

	// GetLogPage issues NVMe Get Log Page with the specified log ID and returns data.
	GetLogPage(logID uint8, size uint32) ([]byte, error)

	// Path returns the device path string (e.g. "/dev/nvme0").
	Path() string

	Close() error
}
