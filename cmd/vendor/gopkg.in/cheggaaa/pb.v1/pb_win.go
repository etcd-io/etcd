// +build windows

package pb

import (
	"os"
	"syscall"
	"unsafe"
)

var tty = os.Stdin

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	// GetConsoleScreenBufferInfo retrieves information about the
	// specified console screen buffer.
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms683171(v=vs.85).aspx
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type (
	// Defines the coordinates of the upper left and lower right corners
	// of a rectangle.
	// See
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms686311(v=vs.85).aspx
	smallRect struct {
		Left, Top, Right, Bottom int16
	}

	// Defines the coordinates of a character cell in a console screen
	// buffer. The origin of the coordinate system (0,0) is at the top, left cell
	// of the buffer.
	// See
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms682119(v=vs.85).aspx
	coordinates struct {
		X, Y int16
	}

	word int16

	// Contains information about a console screen buffer.
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms682093(v=vs.85).aspx
	consoleScreenBufferInfo struct {
		dwSize              coordinates
		dwCursorPosition    coordinates
		wAttributes         word
		srWindow            smallRect
		dwMaximumWindowSize coordinates
	}
)

// terminalWidth returns width of the terminal.
func terminalWidth() (width int, err error) {
	var info consoleScreenBufferInfo
	_, _, e := syscall.Syscall(procGetConsoleScreenBufferInfo.Addr(), 2, uintptr(syscall.Stdout), uintptr(unsafe.Pointer(&info)), 0)
	if e != 0 {
		return 0, error(e)
	}
	return int(info.dwSize.X), nil
}
