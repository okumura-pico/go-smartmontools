//go:build windows

package device

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/windows"
)

// Windows IOCTL codes (from winioctl.h).
// CTL_CODE(DeviceType, Function, Method, Access)
//   = (DeviceType<<16)|(Access<<14)|(Function<<2)|Method
const (
	// IOCTL_DISK_BASE=0x07, FILE_READ_ACCESS|FILE_WRITE_ACCESS=3, METHOD_BUFFERED=0
	ioctlSmartRcvDriveData = 0x0007c088 // IOCTL_SMART_RCV_DRIVE_DATA
	ioctlSmartSendDriveCmd = 0x0007c084 // IOCTL_SMART_SEND_DRIVE_COMMAND
	// IOCTL_STORAGE_BASE=0x2d, FILE_ANY_ACCESS=0
	ioctlStorageQueryProperty = 0x002d1400 // IOCTL_STORAGE_QUERY_PROPERTY

	// ATA command / subcommand codes
	ataSmartCmdW            = 0xb0
	ataIdentifyDeviceW      = 0xec
	ataSmartReadValuesW     = 0xd0
	ataSmartReadThresholdsW = 0xd1
	ataSmartStatusW         = 0xda
	ataSmartReadLogW        = 0xd5

	// Cylinder register values for SMART STATUS
	smartCylLow  = 0x4f
	smartCylHigh = 0xc2
	failCylLow   = 0xf4
	failCylHigh  = 0x2c

	// SENDCMDINPARAMS:  cBufferSize(4)+IDEREGS(8)+bDriveNumber(1)+reserved(3)+dwReserved(16) = 32
	// SENDCMDOUTPARAMS: cBufferSize(4)+DRIVERSTATUS(12) = 16, then bBuffer
	sendCmdInSize  = 32
	sendCmdOutHdr  = 16

	// NVMe protocol constants for IOCTL_STORAGE_QUERY_PROPERTY
	storageDeviceProtocolSpecificProperty = 49
	protocolTypeNvme                      = 3
	nvmeDataTypeIdentify                  = 1
	nvmeDataTypeLogPage                   = 2

	// STORAGE_PROPERTY_QUERY header (PropertyId+QueryType) = 8 bytes
	// STORAGE_PROTOCOL_SPECIFIC_DATA = 8 ULONGs = 32 bytes
	sizeQueryHdr     = 8
	sizeProtocolData = 32
)

// openDevice opens a Windows device handle with read/write access.
func openDevice(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	h, err := windows.CreateFile(p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("open %s: %w", path, err)
	}
	return h, nil
}

// ---- ATA ----

type windowsATADevice struct {
	h    windows.Handle
	path string
}

// OpenATA opens \\.\PhysicalDriveN for ATA SMART access on Windows.
func OpenATA(path string) (ATADevice, error) {
	h, err := openDevice(path)
	if err != nil {
		return nil, err
	}
	return &windowsATADevice{h: h, path: path}, nil
}

func (d *windowsATADevice) Path() string { return d.path }
func (d *windowsATADevice) Close() error { return windows.CloseHandle(d.h) }

// rcvDriveData sends IOCTL_SMART_RCV_DRIVE_DATA and returns the 512-byte sector.
// SENDCMDINPARAMS layout:
//
//	[0..3]  cBufferSize
//	[4]     IDEREGS.bFeaturesReg
//	[5]     IDEREGS.bSectorCountReg
//	[6]     IDEREGS.bSectorNumberReg
//	[7]     IDEREGS.bCylLowReg
//	[8]     IDEREGS.bCylHighReg
//	[9]     IDEREGS.bDriveHeadReg
//	[10]    IDEREGS.bCommandReg
//	[11..31] reserved
func (d *windowsATADevice) rcvDriveData(cmd, feature, sectorNum, cylLow, cylHigh, sectorCount uint8) ([]byte, error) {
	in := make([]byte, sendCmdInSize)
	binary.LittleEndian.PutUint32(in[0:4], 512) // cBufferSize
	in[4] = feature
	in[5] = sectorCount
	in[6] = sectorNum
	in[7] = cylLow
	in[8] = cylHigh
	in[9] = 0xa0 // bDriveHeadReg: drive 0, LBA mode
	in[10] = cmd

	out := make([]byte, sendCmdOutHdr+512)
	var returned uint32
	if err := windows.DeviceIoControl(d.h, ioctlSmartRcvDriveData,
		&in[0], uint32(len(in)), &out[0], uint32(len(out)), &returned, nil); err != nil {
		return nil, err
	}
	if returned < uint32(sendCmdOutHdr+512) {
		return nil, fmt.Errorf("IOCTL_SMART_RCV_DRIVE_DATA: short reply %d B", returned)
	}
	if out[4] != 0 { // DRIVERSTATUS.bDriverError at offset 4
		return nil, fmt.Errorf("IOCTL_SMART_RCV_DRIVE_DATA: driver error=0x%02x ide error=0x%02x", out[4], out[5])
	}
	return out[sendCmdOutHdr:], nil
}

func (d *windowsATADevice) Identify() ([]byte, error) {
	return d.rcvDriveData(ataIdentifyDeviceW, 0, 0, 0, 0, 1)
}

func (d *windowsATADevice) SmartReadData() ([]byte, error) {
	return d.rcvDriveData(ataSmartCmdW, ataSmartReadValuesW, 0, smartCylLow, smartCylHigh, 1)
}

func (d *windowsATADevice) SmartReadThresholds() ([]byte, error) {
	return d.rcvDriveData(ataSmartCmdW, ataSmartReadThresholdsW, 1, smartCylLow, smartCylHigh, 1)
}

func (d *windowsATADevice) SmartReadLog(logAddr uint8) ([]byte, error) {
	return d.rcvDriveData(ataSmartCmdW, ataSmartReadLogW, logAddr, smartCylLow, smartCylHigh, 1)
}

// SmartStatus uses IOCTL_SMART_SEND_DRIVE_COMMAND to check SMART health.
// The returned IDEREGS cylinder registers indicate pass/fail.
func (d *windowsATADevice) SmartStatus() (bool, error) {
	in := make([]byte, sendCmdInSize)
	// cBufferSize = 0 (no data transfer)
	in[4] = ataSmartStatusW // bFeaturesReg
	in[7] = smartCylLow    // bCylLowReg
	in[8] = smartCylHigh   // bCylHighReg
	in[9] = 0xa0            // bDriveHeadReg
	in[10] = ataSmartCmdW  // bCommandReg

	// Output: 16-byte header + 8-byte returned IDEREGS
	out := make([]byte, sendCmdOutHdr+8)
	var returned uint32
	if err := windows.DeviceIoControl(d.h, ioctlSmartSendDriveCmd,
		&in[0], uint32(len(in)), &out[0], uint32(len(out)), &returned, nil); err != nil {
		return false, err
	}
	if out[4] != 0 {
		return false, fmt.Errorf("SMART STATUS: driver error=0x%02x", out[4])
	}
	// Returned IDEREGS starts at sendCmdOutHdr; CylLow=[3], CylHigh=[4].
	cylLow := out[sendCmdOutHdr+3]
	cylHigh := out[sendCmdOutHdr+4]
	switch {
	case cylLow == smartCylLow && cylHigh == smartCylHigh:
		return false, nil // PASSED
	case cylLow == failCylLow && cylHigh == failCylHigh:
		return true, nil // FAILING
	default:
		return false, fmt.Errorf("ambiguous SMART STATUS: cylLow=0x%02x cylHigh=0x%02x", cylLow, cylHigh)
	}
}

// ---- NVMe ----

type windowsNVMeDevice struct {
	h    windows.Handle
	path string
}

// OpenNVMe opens \\.\PhysicalDriveN for NVMe Admin Command access on Windows.
func OpenNVMe(path string) (NVMeDevice, error) {
	h, err := openDevice(path)
	if err != nil {
		return nil, err
	}
	return &windowsNVMeDevice{h: h, path: path}, nil
}

func (d *windowsNVMeDevice) Path() string { return d.path }
func (d *windowsNVMeDevice) Close() error { return windows.CloseHandle(d.h) }

// nvmeQuery sends IOCTL_STORAGE_QUERY_PROPERTY with a NVMe protocol-specific
// query and returns the response data.
//
// Buffer layout (same pointer used for both input and output):
//
//	[0..7]           STORAGE_PROPERTY_QUERY header (PropertyId + QueryType)
//	[8..39]          STORAGE_PROTOCOL_SPECIFIC_DATA (32 bytes, in AdditionalParameters)
//	[40..40+dataLen] output data from the device
func (d *windowsNVMeDevice) nvmeQuery(dataType, requestValue, dataLen uint32) ([]byte, error) {
	bufSize := sizeQueryHdr + sizeProtocolData + int(dataLen)
	buf := make([]byte, bufSize)

	// STORAGE_PROPERTY_QUERY header at [0..7]
	binary.LittleEndian.PutUint32(buf[0:4], storageDeviceProtocolSpecificProperty)
	binary.LittleEndian.PutUint32(buf[4:8], 0) // PropertyStandardQuery

	// STORAGE_PROTOCOL_SPECIFIC_DATA at [8..39]
	o := sizeQueryHdr
	binary.LittleEndian.PutUint32(buf[o+0:o+4], protocolTypeNvme)
	binary.LittleEndian.PutUint32(buf[o+4:o+8], dataType)
	binary.LittleEndian.PutUint32(buf[o+8:o+12], requestValue)
	binary.LittleEndian.PutUint32(buf[o+12:o+16], 0) // RequestSubValue
	// ProtocolDataOffset: relative to start of STORAGE_PROTOCOL_SPECIFIC_DATA
	binary.LittleEndian.PutUint32(buf[o+16:o+20], sizeProtocolData)
	binary.LittleEndian.PutUint32(buf[o+20:o+24], dataLen)
	// FixedProtocolReturnData and Reserved remain 0

	var returned uint32
	if err := windows.DeviceIoControl(d.h, ioctlStorageQueryProperty,
		&buf[0], uint32(bufSize), &buf[0], uint32(bufSize), &returned, nil); err != nil {
		return nil, err
	}
	dataStart := sizeQueryHdr + sizeProtocolData
	if int(returned) < dataStart+int(dataLen) {
		return nil, fmt.Errorf("NVMe query: short reply %d B", returned)
	}
	out := make([]byte, dataLen)
	copy(out, buf[dataStart:])
	return out, nil
}

func (d *windowsNVMeDevice) IdentifyController() ([]byte, error) {
	// DataType=Identify(1), RequestValue=CNS_CONTROLLER(1), 4096 bytes
	data, err := d.nvmeQuery(nvmeDataTypeIdentify, 1, 4096)
	if err != nil {
		return nil, fmt.Errorf("NVMe Identify Controller: %w", err)
	}
	return data, nil
}

func (d *windowsNVMeDevice) GetLogPage(logID uint8, size uint32) ([]byte, error) {
	data, err := d.nvmeQuery(nvmeDataTypeLogPage, uint32(logID), size)
	if err != nil {
		return nil, fmt.Errorf("NVMe Get Log Page 0x%02x: %w", logID, err)
	}
	return data, nil
}
