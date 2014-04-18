// +build !linux !amd64

package btrfs

import (
	"fmt"
)

// IsBtrfs checks whether the file is in btrfs
func IsBtrfs(path string) bool {
	return false
}

// SetNOCOWFile sets NOCOW flag for file
func SetNOCOWFile(path string) error {
	return fmt.Errorf("unsupported for the platform")
}
