//go:build linux

package device

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux ioctl numbers for ATA and NVMe.
// These match <linux/hdreg.h> and <linux/nvme_ioctl.h>.
const (
	hdioDriveCmd = 0x031f // HDIO_DRIVE_CMD
	hdioDriveTask = 0x031e // HDIO_DRIVE_TASK

	// ATA command codes
	ataSmartCmd         = 0xb0
	ataIdentifyDevice   = 0xec
	ataCheckPowerMode   = 0xe5

	// ATA SMART subcommands (feature register)
	ataSmartReadValues     = 0xd0
	ataSmartReadThresholds = 0xd1
	ataSmartEnable         = 0xd8
	ataSmartDisable        = 0xd9
	ataSmartStatus         = 0xda
	ataSmartReadLog        = 0xd5

	// ATA cylinder register values for SMART STATUS check
	smartStatusCylinderLow  = 0x4f
	smartStatusCylinderHigh = 0xc2
	smartFailCylinderLow    = 0xf4
	smartFailCylinderHigh   = 0x2c

	// HDIO_DRIVE_CMD buffer layout: [cmd, lbaLow, feature, sectorCount, data...]
	hdioOffset = 4

	// NVMe ioctl: _IOWR('N', 0x41, struct nvme_admin_cmd)
	nvmeIoctlAdminCmd = 0xc0484e41

	nvmeAdminIdentify   = 0x06
	nvmeAdminGetLogPage = 0x02
)

// linuxATADevice implements ATADevice for Linux using HDIO_DRIVE_CMD.
type linuxATADevice struct {
	f    *os.File
}

// OpenATA opens a block device for ATA access.
func OpenATA(path string) (ATADevice, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &linuxATADevice{f: f}, nil
}

func (d *linuxATADevice) Path() string { return d.f.Name() }
func (d *linuxATADevice) Close() error { return d.f.Close() }

// hdioCmdRead issues HDIO_DRIVE_CMD that reads 512 bytes of data.
// buff layout: [command, lbaLow, feature, sectorCount, 512 bytes data...]
func (d *linuxATADevice) hdioCmdRead(command, lbaLow, feature, sectorCount uint8) ([]byte, error) {
	buf := make([]byte, hdioOffset+512)
	buf[0] = command
	buf[1] = lbaLow
	buf[2] = feature
	buf[3] = sectorCount

	if err := ioctl(d.f.Fd(), hdioDriveCmd, buf); err != nil {
		return nil, err
	}
	data := make([]byte, 512)
	copy(data, buf[hdioOffset:])
	return data, nil
}

func (d *linuxATADevice) Identify() ([]byte, error) {
	return d.hdioCmdRead(ataIdentifyDevice, 0, 0, 1)
}

func (d *linuxATADevice) SmartReadData() ([]byte, error) {
	return d.hdioCmdRead(ataSmartCmd, 0, ataSmartReadValues, 1)
}

func (d *linuxATADevice) SmartReadThresholds() ([]byte, error) {
	return d.hdioCmdRead(ataSmartCmd, 1, ataSmartReadThresholds, 1)
}

// SmartStatus uses HDIO_DRIVE_TASK to issue SMART RETURN STATUS.
// The drive reports health via the cylinder registers in the task file.
func (d *linuxATADevice) SmartStatus() (bool, error) {
	// HDIO_DRIVE_TASK buffer: [cmd, feat, sectorCount, sectorNum, cylLow, cylHigh, driveHead]
	buf := [7]byte{
		ataSmartCmd,
		ataSmartStatus,
		0, 0, 0, 0, 0,
	}
	if err := ioctl(d.f.Fd(), hdioDriveTask, buf[:]); err != nil {
		return false, err
	}
	cylLow := buf[4]
	cylHigh := buf[5]
	if cylLow == smartStatusCylinderLow && cylHigh == smartStatusCylinderHigh {
		return false, nil // PASSED
	}
	if cylLow == smartFailCylinderLow && cylHigh == smartFailCylinderHigh {
		return true, nil // FAILING
	}
	// Ambiguous response: assume OK but return a warning-style error
	return false, fmt.Errorf("SMART STATUS returned ambiguous cylinder registers: low=0x%02x high=0x%02x", cylLow, cylHigh)
}

func (d *linuxATADevice) SmartReadLog(logAddr uint8) ([]byte, error) {
	return d.hdioCmdRead(ataSmartCmd, logAddr, ataSmartReadLog, 1)
}

// ioctl is a thin wrapper around unix.Syscall for ioctl calls that pass a byte slice.
func ioctl(fd uintptr, req uintptr, arg []byte) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(&arg[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

// nvmeAdminCmd mirrors the kernel's nvme_passthru_cmd struct.
type nvmeAdminCmdStruct struct {
	Opcode      uint8
	Flags       uint8
	Rsvd1       uint16
	Nsid        uint32
	Cdw2        uint32
	Cdw3        uint32
	Metadata    uint64
	Addr        uint64
	MetadataLen uint32
	DataLen     uint32
	Cdw10       uint32
	Cdw11       uint32
	Cdw12       uint32
	Cdw13       uint32
	Cdw14       uint32
	Cdw15       uint32
	TimeoutMs   uint32
	Result      uint32
}

// linuxNVMeDevice implements NVMeDevice for Linux.
type linuxNVMeDevice struct {
	f *os.File
}

// OpenNVMe opens a NVMe device for admin command access.
func OpenNVMe(path string) (NVMeDevice, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &linuxNVMeDevice{f: f}, nil
}

func (d *linuxNVMeDevice) Path() string { return d.f.Name() }
func (d *linuxNVMeDevice) Close() error { return d.f.Close() }

func (d *linuxNVMeDevice) nvmeAdmin(opcode uint8, nsid, cdw10, cdw11 uint32, data []byte) error {
	cmd := nvmeAdminCmdStruct{
		Opcode:  opcode,
		Nsid:    nsid,
		Addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		DataLen: uint32(len(data)),
		Cdw10:   cdw10,
		Cdw11:   cdw11,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		d.f.Fd(),
		nvmeIoctlAdminCmd,
		uintptr(unsafe.Pointer(&cmd)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func (d *linuxNVMeDevice) IdentifyController() ([]byte, error) {
	data := make([]byte, 4096)
	// CDW10: CNS=0x01 (Identify Controller)
	if err := d.nvmeAdmin(nvmeAdminIdentify, 0, 0x01, 0, data); err != nil {
		return nil, fmt.Errorf("NVMe Identify Controller: %w", err)
	}
	return data, nil
}

func (d *linuxNVMeDevice) GetLogPage(logID uint8, size uint32) ([]byte, error) {
	data := make([]byte, size)
	// CDW10: LID | (numd << 16) where numd = (size/4 - 1)
	numd := size/4 - 1
	cdw10 := uint32(logID) | (numd << 16)
	if err := d.nvmeAdmin(nvmeAdminGetLogPage, 0xffffffff, cdw10, 0, data); err != nil {
		return nil, fmt.Errorf("NVMe Get Log Page 0x%02x: %w", logID, err)
	}
	return data, nil
}
