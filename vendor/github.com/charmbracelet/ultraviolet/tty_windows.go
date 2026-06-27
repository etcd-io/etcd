//go:build windows
// +build windows

package uv

import "os"

func openTTY() (inTty, outTty *os.File, err error) {
	// On Windows, when the input/output is redirected or piped, we need to
	// open the console explicitly.
	// See https://learn.microsoft.com/en-us/windows/console/getstdhandle#remarks
	inTty, err = os.OpenFile("CONIN$", os.O_RDWR, 0o644) //nolint:gosec
	if err != nil {
		return nil, nil, err //nolint:wrapcheck
	}
	outTty, err = os.OpenFile("CONOUT$", os.O_RDWR, 0o644) //nolint:gosec
	if err != nil {
		return nil, nil, err //nolint:wrapcheck
	}
	return inTty, outTty, nil
}

func suspend() (err error) {
	// On Windows, suspending the process group is not supported in the same
	// way as Unix-like systems.
	return nil
}
