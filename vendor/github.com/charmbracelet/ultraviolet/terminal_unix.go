//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix
// +build darwin dragonfly freebsd linux netbsd openbsd solaris aix

package uv

import (
	"github.com/charmbracelet/x/term"
)

func (t *Terminal) makeRaw() error {
	var err error

	if t.inTty == nil && t.outTty == nil {
		return ErrNotTerminal
	}

	// Check if we have a terminal.
	for _, f := range []term.File{t.inTty, t.outTty} {
		if f == nil {
			continue
		}
		t.inTtyState, err = term.MakeRaw(f.Fd())
		if err == nil {
			break
		}
	}

	if err != nil {
		return err //nolint:wrapcheck
	}

	return nil
}

func (t *Terminal) getSize() (w, h int, err error) {
	// Try both inTty and outTty to get the size.
	err = ErrNotTerminal
	for _, f := range []term.File{t.inTty, t.outTty} {
		if f == nil {
			continue
		}
		w, h, err = term.GetSize(f.Fd())
		if err == nil {
			return w, h, nil
		}
	}
	return
}

func (t *Terminal) optimizeMovements() {
	// Try both inTty and outTty to get the size.
	var state *term.State
	var err error
	for _, s := range []*term.State{t.inTtyState, t.outTtyState} {
		if s == nil {
			continue
		}
		state = s
		break
	}
	if state == nil {
		for _, f := range []term.File{t.inTty, t.outTty} {
			if f == nil {
				continue
			}
			state, err = term.GetState(f.Fd())
			if err == nil {
				break
			}
		}
	}
	if state == nil {
		return
	}
	t.useTabs = supportsHardTabs(uint64(state.Oflag))    //nolint:unconvert,nolintlint
	t.useBspace = supportsBackspace(uint64(state.Lflag)) //nolint:unconvert,nolintlint
}
