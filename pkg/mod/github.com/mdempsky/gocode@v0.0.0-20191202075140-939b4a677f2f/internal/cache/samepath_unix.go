// +build !windows

package cache

// SamePath checks two file paths for their equality based on the current filesystem
func SamePath(a, b string) bool {
	return a == b
}
