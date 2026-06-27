//go:build aix

package termutil

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

var (
	tty *os.File

	unlockSignals = []os.Signal{
		os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGKILL,
	}
)

func init() {
	var err error
	tty, err = os.Open("/dev/tty")
	if err != nil {
		tty = os.Stdin
	}
}

// TerminalWidth returns width of the terminal.
func TerminalWidth() (int, error) {
	_, width, err := TerminalSize()
	return width, err
}

// TerminalSize returns size of the terminal.
func TerminalSize() (int, int, error) {
	w, err := unix.IoctlGetWinsize(int(tty.Fd()), syscall.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(w.Row), int(w.Col), nil
}

var oldState unix.Termios

func lockEcho() error {
	fd := int(tty.Fd())
	currentState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	oldState = *currentState
	newState := oldState
	newState.Lflag &^= syscall.ECHO
	newState.Lflag |= syscall.ICANON | syscall.ISIG
	newState.Iflag |= syscall.ICRNL
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return err
	}
	return nil
}

func unlockEcho() (err error) {
	fd := int(tty.Fd())
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &oldState); err != nil {
		return err
	}
	return
}
