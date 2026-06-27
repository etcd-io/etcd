//go:build !windows

package fsutils

// NormalizePathInRegex it's a noop function on Unix.
func NormalizePathInRegex(path string) string {
	return path
}
