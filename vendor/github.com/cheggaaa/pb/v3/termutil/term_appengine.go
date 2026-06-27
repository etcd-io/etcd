//go:build appengine
// +build appengine

package termutil

import "errors"

// terminalWidth returns width of the terminal, which is not supported
// and should always failed on appengine classic which is a sandboxed PaaS.
func TerminalWidth() (int, error) {
	return 0, errors.New("Not supported")
}
