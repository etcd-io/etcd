// +build linux,amd64

package btrfs

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/coreos/etcd/log"
)

const (
	// from Linux/include/uapi/linux/magic.h
	BTRFS_SUPER_MAGIC = 0x9123683E

	// from Linux/include/uapi/linux/fs.h
	FS_NOCOW_FL = 0x00800000
	FS_IOC_GETFLAGS = 0x80086601
	FS_IOC_SETFLAGS = 0x40086602
)

// IsBtrfs checks whether the file is in btrfs
func IsBtrfs(path string) bool {
	// btrfs is linux-only filesystem
	// exit on other platforms
	if runtime.GOOS != "linux" {
		return false
	}
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		log.Warnf("Failed to statfs: %v", err)
		return false
	}
	log.Debugf("The type of path %v is %v", path, buf.Type)
	if buf.Type != BTRFS_SUPER_MAGIC {
		return false
	}
	log.Infof("The path %v is in btrfs", path)
	return true
}

// SetNOCOWFile sets NOCOW flag for file
func SetNOCOWFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fileinfo, err := file.Stat()
	if err != nil {
		return err
	}
	if fileinfo.IsDir() {
		return fmt.Errorf("skip directory")
	}
	if fileinfo.Size() != 0 {
		return fmt.Errorf("skip nonempty file")
	}

	var attr int
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), FS_IOC_GETFLAGS, uintptr(unsafe.Pointer(&attr))); errno != 0 {
		return errno
	}
	attr |= FS_NOCOW_FL
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), FS_IOC_SETFLAGS, uintptr(unsafe.Pointer(&attr))); errno != 0 {
		return errno
	}
	log.Infof("Set NOCOW to path %v succeeded", path)
	return nil
}
