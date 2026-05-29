//go:build windows

package device

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/windows"
)

// storageDeviceProperty = PropertyId 0 → returns STORAGE_DEVICE_DESCRIPTOR
const (
	storageDeviceProperty = 0
	propertyStandardQuery = 0
	busTypeNvme           = 17 // STORAGE_BUS_TYPE: BusTypeNvme
)

// IsNVMe opens the device and queries the storage bus type via
// IOCTL_STORAGE_QUERY_PROPERTY to determine whether it is NVMe.
func IsNVMe(path string) (bool, error) {
	h, err := openDevice(path)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)

	// Build STORAGE_PROPERTY_QUERY (8 bytes): PropertyId + QueryType
	in := make([]byte, 8)
	binary.LittleEndian.PutUint32(in[0:4], storageDeviceProperty)
	binary.LittleEndian.PutUint32(in[4:8], propertyStandardQuery)

	// Output is STORAGE_DEVICE_DESCRIPTOR; allocate 512 bytes (more than enough for header).
	out := make([]byte, 512)
	var returned uint32
	err = windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		&in[0], uint32(len(in)),
		&out[0], uint32(len(out)),
		&returned, nil)
	if err != nil {
		return false, fmt.Errorf("STORAGE_QUERY_PROPERTY: %w", err)
	}
	if returned < 32 {
		return false, fmt.Errorf("STORAGE_DEVICE_DESCRIPTOR too short: %d bytes", returned)
	}

	// BusType is at offset 28 in STORAGE_DEVICE_DESCRIPTOR.
	busType := binary.LittleEndian.Uint32(out[28:32])
	return busType == busTypeNvme, nil
}
