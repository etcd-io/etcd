package uv

import "os"

// OpenTTY opens the terminal's input and output file descriptors.
// It returns the input and output files, or an error if the terminal is not
// available.
//
// This is useful for applications that need to interact with the terminal
// directly while piping or redirecting input/output.
func OpenTTY() (inTty, outTty *os.File, err error) {
	return openTTY()
}

// Suspend suspends the current process group.
func Suspend() error {
	return suspend()
}
