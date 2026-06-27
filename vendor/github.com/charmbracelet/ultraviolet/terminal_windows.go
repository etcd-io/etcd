//go:build windows
// +build windows

package uv

import (
	"fmt"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/windows"
)

func (t *Terminal) makeRaw() (err error) {
	if t.inTty == nil || t.outTty == nil || !term.IsTerminal(t.inTty.Fd()) || !term.IsTerminal(t.outTty.Fd()) {
		return ErrNotTerminal
	}

	// Save stdin state and enable VT input.
	// We also need to enable VT input here.
	t.inTtyState, err = term.MakeRaw(t.inTty.Fd())
	if err != nil {
		return fmt.Errorf("error making terminal raw: %w", err)
	}

	// Enable VT input
	var imode uint32
	if err := windows.GetConsoleMode(windows.Handle(t.inTty.Fd()), &imode); err != nil {
		return fmt.Errorf("error getting console mode: %w", err)
	}

	if err := windows.SetConsoleMode(windows.Handle(t.inTty.Fd()), imode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err != nil {
		return fmt.Errorf("error setting console mode: %w", err)
	}

	// Save output screen buffer state and enable VT processing.
	t.outTtyState, err = term.GetState(t.outTty.Fd())
	if err != nil {
		return fmt.Errorf("error getting terminal state: %w", err)
	}

	var omode uint32
	if err := windows.GetConsoleMode(windows.Handle(t.outTty.Fd()), &omode); err != nil {
		return fmt.Errorf("error getting console mode: %w", err)
	}

	if err := windows.SetConsoleMode(windows.Handle(t.outTty.Fd()),
		omode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|
			windows.DISABLE_NEWLINE_AUTO_RETURN); err != nil {
		return fmt.Errorf("error setting console mode: %w", err)
	}

	return //nolint:nakedret
}

func (t *Terminal) getSize() (w, h int, err error) {
	if t.outTty != nil {
		return term.GetSize(t.outTty.Fd()) //nolint:wrapcheck
	}
	return 0, 0, ErrNotTerminal
}

func (t *Terminal) optimizeMovements() {
	t.useBspace = supportsBackspace(0)
	t.useTabs = supportsHardTabs(0)
}

func supportsBackspace(uint64) bool {
	return true
}

func supportsHardTabs(uint64) bool {
	return true
}
