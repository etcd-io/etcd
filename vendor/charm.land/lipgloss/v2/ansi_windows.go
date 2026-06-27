//go:build windows

package lipgloss

import (
	"os"

	"golang.org/x/sys/windows"
)

// EnableLegacyWindowsANSI enables support for ANSI color sequences in the
// Windows default console (cmd.exe and the PowerShell application). Note that
// this only works with Windows 10 and greater. Also note that Windows Terminal
// supports colors by default.
func EnableLegacyWindowsANSI(f *os.File) {
	var mode uint32
	handle := windows.Handle(f.Fd())
	err := windows.GetConsoleMode(handle, &mode)
	if err != nil {
		return
	}

	// See https://docs.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING {
		vtpmode := mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
		if err := windows.SetConsoleMode(handle, vtpmode); err != nil {
			return
		}
	}
}
