//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris zos

package term

import (
	"golang.org/x/sys/unix"
)

type state struct {
	unix.Termios
}

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	return err == nil
}

func makeRaw(fd uintptr) (*State, error) {
	termios, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	oldState := State{state{Termios: *termios}}

	// This attempts to replicate the behaviour documented for cfmakeraw in
	// the termios(3) manpage.
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), ioctlWriteTermios, termios); err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &oldState, nil
}

func setState(fd uintptr, state *State) error {
	var termios *unix.Termios
	if state != nil {
		termios = &state.Termios
	}
	return unix.IoctlSetTermios(int(fd), ioctlWriteTermios, termios) //nolint:wrapcheck
}

func getState(fd uintptr) (*State, error) {
	termios, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &State{state{Termios: *termios}}, nil
}

func restore(fd uintptr, state *State) error {
	return unix.IoctlSetTermios(int(fd), ioctlWriteTermios, &state.Termios) //nolint:wrapcheck
}

func getSize(fd uintptr) (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err //nolint:wrapcheck
	}
	return int(ws.Col), int(ws.Row), nil
}

// passwordReader is an io.Reader that reads from a specific file descriptor.
type passwordReader int

func (r passwordReader) Read(buf []byte) (int, error) {
	return unix.Read(int(r), buf) //nolint:wrapcheck
}

func readPassword(fd uintptr) ([]byte, error) {
	termios, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	newState := *termios
	newState.Lflag &^= unix.ECHO
	newState.Lflag |= unix.ICANON | unix.ISIG
	newState.Iflag |= unix.ICRNL
	if err := unix.IoctlSetTermios(int(fd), ioctlWriteTermios, &newState); err != nil {
		return nil, err //nolint:wrapcheck
	}

	defer unix.IoctlSetTermios(int(fd), ioctlWriteTermios, termios) //nolint:errcheck

	return readPasswordLine(passwordReader(fd))
}
