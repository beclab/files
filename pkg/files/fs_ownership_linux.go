//go:build linux

package files

import (
	"syscall"

	"k8s.io/klog/v2"
)

// Filesystem magic numbers from include/uapi/linux/magic.h for
// filesystems with no on-disk ownership; chown(2) on them always
// returns EPERM regardless of caller privilege.
const (
	magicMSDOS uint32 = 0x4d44     // FAT12/16/32
	magicEXFAT uint32 = 0x2011BAB0 // exFAT (kernel native, >= 5.7)
	magicNTFS  uint32 = 0x5346544e // NTFS (legacy)
	magicNTFS3 uint32 = 0x7366746e // NTFS3 (kernel native, >= 5.15)
	magicISO   uint32 = 0x9660     // ISO 9660
	magicUDF   uint32 = 0x15013346 // UDF
)

// SupportsOwnership reports whether the filesystem backing path can
// honour chown(2). Conservative: on statfs error or unknown magic it
// returns true so the caller still attempts the syscall.
func SupportsOwnership(path string) bool {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		klog.V(4).Infof("SupportsOwnership: statfs(%s) failed: %v; assuming supported", path, err)
		return true
	}
	switch uint32(stat.Type) {
	case magicMSDOS, magicEXFAT, magicNTFS, magicNTFS3, magicISO, magicUDF:
		return false
	}
	return true
}
