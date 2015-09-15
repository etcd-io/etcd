// +build linux darwin freebsd netbsd openbsd solaris

package pb

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	TIOCGWINSZ     = 0x5413
	TIOCGWINSZ_OSX = 1074295912
)

var tty *os.File

func init() {
	var err error
	tty, err = os.Open("/dev/tty")
	if err != nil {
		tty = os.Stdin
	}
}

func bold(str string) string {
	return "\033[1m" + str + "\033[0m"
}

func terminalWidth() (int, error) {
	w := new(window)
	tio := syscall.TIOCGWINSZ
	if runtime.GOOS == "darwin" {
		tio = TIOCGWINSZ_OSX
	}
	res, _, err := syscall.Syscall(sys_ioctl,
		tty.Fd(),
		uintptr(tio),
		uintptr(unsafe.Pointer(w)),
	)
	if int(res) == -1 {
		return 0, err
	}
	return int(w.Col), nil
}
