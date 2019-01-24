// +build linux darwin freebsd netbsd openbsd solaris dragonfly
// +build !appengine !js

package pb

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

var ErrPoolWasStarted = errors.New("Bar pool was started")

var (
	echoLockMutex    sync.Mutex
	origTermStatePtr *unix.Termios
	tty              *os.File
	istty            bool
)

func init() {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()

	var err error
	tty, err = os.Open("/dev/tty")
	istty = true
	if err != nil {
		tty = os.Stdin
		istty = false
	}
}

// terminalWidth returns width of the terminal.
func terminalWidth() (int, error) {
	if !istty {
		return 0, errors.New("Not Supported")
	}
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()

	fd := int(tty.Fd())

	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}

	return int(ws.Col), nil
}

func lockEcho() (shutdownCh chan struct{}, err error) {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()
	if istty {
		if origTermStatePtr != nil {
			return shutdownCh, ErrPoolWasStarted
		}

		fd := int(tty.Fd())

		origTermStatePtr, err = unix.IoctlGetTermios(fd, ioctlReadTermios)
		if err != nil {
			return nil, fmt.Errorf("Can't get terminal settings: %v", err)
		}

		oldTermios := *origTermStatePtr
		newTermios := oldTermios
		newTermios.Lflag &^= syscall.ECHO
		newTermios.Lflag |= syscall.ICANON | syscall.ISIG
		newTermios.Iflag |= syscall.ICRNL
		if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &newTermios); err != nil {
			return nil, fmt.Errorf("Can't set terminal settings: %v", err)
		}

	}
	shutdownCh = make(chan struct{})
	go catchTerminate(shutdownCh)
	return
}

func unlockEcho() error {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()
	if istty {
		if origTermStatePtr == nil {
			return nil
		}

		fd := int(tty.Fd())

		if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, origTermStatePtr); err != nil {
			return fmt.Errorf("Can't set terminal settings: %v", err)
		}

	}
	origTermStatePtr = nil

	return nil
}

// listen exit signals and restore terminal state
func catchTerminate(shutdownCh chan struct{}) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGKILL)
	defer signal.Stop(sig)
	select {
	case <-shutdownCh:
		unlockEcho()
	case <-sig:
		unlockEcho()
	}
}
