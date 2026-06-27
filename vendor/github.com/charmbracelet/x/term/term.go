// Package term provides a platform-independent interfaces for interacting with
// Terminal and TTY devices.
package term

// State contains platform-specific state of a terminal.
type State struct {
	state
}

// IsTerminal returns whether the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return isTerminal(fd)
}

// MakeRaw puts the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd uintptr) (*State, error) {
	return makeRaw(fd)
}

// GetState returns the current state of a terminal which may be useful to
// restore the terminal after a signal.
func GetState(fd uintptr) (*State, error) {
	return getState(fd)
}

// SetState sets the given state of the terminal.
func SetState(fd uintptr, state *State) error {
	return setState(fd, state)
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func Restore(fd uintptr, oldState *State) error {
	return restore(fd, oldState)
}

// GetSize returns the visible dimensions of the given terminal.
//
// These dimensions don't include any scrollback buffer height.
func GetSize(fd uintptr) (width, height int, err error) {
	return getSize(fd)
}

// ReadPassword reads a line of input from a terminal without local echo.  This
// is commonly used for inputting passwords and other sensitive data. The slice
// returned does not include the \n.
func ReadPassword(fd uintptr) ([]byte, error) {
	return readPassword(fd)
}
