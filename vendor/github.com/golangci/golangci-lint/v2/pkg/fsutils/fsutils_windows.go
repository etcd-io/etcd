//go:build windows

package fsutils

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// This is a workaround for the behavior of [filepath.EvalSymlinks],
// which fails with [syscall.ENOTDIR] if the specified path contains a junction on Windows.
// Junctions can occur, for example, when a volume is mounted as a subdirectory inside another drive.
// This can usually happen when using the Dev Drives feature and replacing existing directories.
// See: https://github.com/golang/go/issues/40180
//
// Since [syscall.ENOTDIR] is only returned when calling [filepath.EvalSymlinks] on Windows
// if part of the presented path is a junction and nothing before was a symlink,
// we simply treat this as NOT symlink,
// because a symlink over the junction makes no sense at all.
func evalSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, syscall.ENOTDIR) {
		return "", err
	}

	_, err = os.Stat(path)
	if err != nil {
		return "", err
	}

	// If exists, we make the path absolute, to be sure...
	return filepath.Abs(path)
}
