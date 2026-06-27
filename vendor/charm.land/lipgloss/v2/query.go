package lipgloss

import (
	"fmt"
	"image/color"
	"os"
	"runtime"

	"github.com/charmbracelet/x/term"
)

func backgroundColor(in term.File, out term.File) (color.Color, error) {
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return nil, fmt.Errorf("error setting raw state to detect background color: %w", err)
	}

	defer term.Restore(in.Fd(), state) //nolint:errcheck

	bg, err := queryBackgroundColor(in, out)
	if err != nil {
		return nil, err
	}

	return bg, nil
}

// BackgroundColor queries the terminal's background color. Typically, you'll
// want to query against stdin and either stdout or stderr, depending on what
// you're writing to.
//
// This function is intended for standalone Lip Gloss use only. If you're using
// Bubble Tea, listen for tea.BackgroundColorMsg in your update function.
func BackgroundColor(in term.File, out term.File) (bg color.Color, err error) {
	if runtime.GOOS == "windows" { //nolint:nestif
		// On Windows, when the input/output is redirected or piped, we need to
		// open the console explicitly.
		// See https://learn.microsoft.com/en-us/windows/console/getstdhandle#remarks
		if !term.IsTerminal(in.Fd()) {
			f, err := os.OpenFile("CONIN$", os.O_RDWR, 0o644) //nolint:gosec
			if err != nil {
				return nil, fmt.Errorf("error opening CONIN$: %w", err)
			}
			in = f
		}
		if !term.IsTerminal(out.Fd()) {
			f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0o644) //nolint:gosec
			if err != nil {
				return nil, fmt.Errorf("error opening CONOUT$: %w", err)
			}
			out = f
		}
		return backgroundColor(in, out)
	}

	// NOTE: On Unix, one of the given files must be a tty.
	if !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return nil, fmt.Errorf("input/output is not a terminal")
	}
	for _, f := range []term.File{in, out} {
		if bg, err = backgroundColor(f, f); err == nil {
			return bg, nil
		}
	}

	return bg, err
}

// HasDarkBackground detects whether the terminal has a light or dark
// background.
//
// Typically, you'll want to query against stdin and either stdout or stderr
// depending on what you're writing to.
//
//	hasDarkBG := HasDarkBackground(os.Stdin, os.Stdout)
//	lightDark := LightDark(hasDarkBG)
//	myHotColor := lightDark("#ff0000", "#0000ff")
//
// This is intended for use in standalone Lip Gloss only. In Bubble Tea, listen
// for tea.BackgroundColorMsg in your Update function.
//
//	case tea.BackgroundColorMsg:
//	    hasDarkBackground = msg.IsDark()
//
// By default, this function will return true if it encounters an error.
func HasDarkBackground(in term.File, out term.File) bool {
	bg, err := BackgroundColor(in, out)
	if err != nil || bg == nil {
		return true
	}
	return isDarkColor(bg)
}
