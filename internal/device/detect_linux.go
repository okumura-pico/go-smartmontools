//go:build linux

package device

// IsNVMe returns true when path looks like a NVMe block device (/dev/nvme*).
func IsNVMe(path string) (bool, error) {
	return len(path) >= 9 && path[:9] == "/dev/nvme", nil
}
