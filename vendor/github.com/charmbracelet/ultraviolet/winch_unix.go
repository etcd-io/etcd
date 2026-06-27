//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos
// +build darwin dragonfly freebsd linux netbsd openbsd solaris zos

package uv

import (
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"
	"github.com/charmbracelet/x/termios"
)

func (n *SizeNotifier) start() error {
	n.m.Lock()
	defer n.m.Unlock()
	if n.f == nil || !term.IsTerminal(n.f.Fd()) {
		return ErrNotTerminal
	}

	signal.Notify(n.sig, syscall.SIGWINCH)
	return nil
}

func (n *SizeNotifier) stop() error {
	n.m.Lock()
	signal.Stop(n.sig)
	n.m.Unlock()
	return nil
}

func (n *SizeNotifier) getWindowSize() (cells Size, pixels Size, err error) {
	n.m.Lock()
	defer n.m.Unlock()

	winsize, err := termios.GetWinsize(int(n.f.Fd()))
	if err != nil {
		return Size{}, Size{}, err //nolint:wrapcheck
	}

	cells = Size{
		Width:  int(winsize.Col),
		Height: int(winsize.Row),
	}
	pixels = Size{
		Width:  int(winsize.Xpixel),
		Height: int(winsize.Ypixel),
	}
	return cells, pixels, nil
}
