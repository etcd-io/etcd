//go:build !windows

package fsutils

import "path/filepath"

func evalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
