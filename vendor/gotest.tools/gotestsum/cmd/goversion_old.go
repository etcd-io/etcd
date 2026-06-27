//go:build !go1.22
// +build !go1.22

package cmd

func isGoVersionAtLeast(_ string) bool {
	return false
}
