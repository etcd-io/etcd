//go:build !windows

package lipgloss

import "os"

// EnableLegacyWindowsANSI is only needed on Windows.
func EnableLegacyWindowsANSI(*os.File) {}
