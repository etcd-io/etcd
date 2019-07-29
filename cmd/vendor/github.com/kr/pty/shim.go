// Package pty is a wrapper for github.com/creack/pty, which provides
// functions for working with Unix terminals.
//
// This package is deprecated. Existing clients will continue to work,
// but no further updates will happen here. New clients should use
// github.com/creack/pty directly.
package pty

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
	newpty "github.com/creack/pty"
)

// ErrUnsupported is returned if a function is not available on the
// current platform.
//
// Deprecated; please use github.com/creack/pty instead.
var ErrUnsupported = pty.ErrUnsupported

// Winsize describes the terminal size.
//
// Deprecated; please use github.com/creack/pty instead.
type Winsize = pty.Winsize

// Getsize returns the number of rows (lines) and cols (positions in
// each line) in terminal t.
//
// Deprecated; please use github.com/creack/pty instead.
func Getsize(t *os.File) (rows, cols int, err error) { return pty.Getsize(t) }

// GetsizeFull returns the full terminal size description.
//
// Deprecated; please use github.com/creack/pty instead.
func GetsizeFull(t *os.File) (size *Winsize, err error) {
	return pty.GetsizeFull(t)
}

// InheritSize applies the terminal size of pty to tty. This should be
// run in a signal handler for syscall.SIGWINCH to automatically
// resize the tty when the pty receives a window size change
// notification.
//
// Deprecated; please use github.com/creack/pty instead.
func InheritSize(pty, tty *os.File) error { return newpty.InheritSize(pty, tty) }

// Opens a pty and its corresponding tty.
//
// Deprecated; please use github.com/creack/pty instead.
func Open() (pty, tty *os.File, err error) { return newpty.Open() }

// Setsize resizes t to s.
//
// Deprecated; please use github.com/creack/pty instead.
func Setsize(t *os.File, ws *Winsize) error { return pty.Setsize(t, ws) }

// Start assigns a pseudo-terminal tty os.File to c.Stdin, c.Stdout,
// and c.Stderr, calls c.Start, and returns the File of the tty's
// corresponding pty.
//
// Deprecated; please use github.com/creack/pty instead.
func Start(c *exec.Cmd) (pty *os.File, err error) { return newpty.Start(c) }

// StartWithSize assigns a pseudo-terminal tty os.File to c.Stdin,
// c.Stdout, and c.Stderr, calls c.Start, and returns the File of the
// tty's corresponding pty.
//
// This will resize the pty to the specified size before starting the
// command.
//
// Deprecated; please use github.com/creack/pty instead.
func StartWithSize(c *exec.Cmd, sz *Winsize) (pty *os.File, err error) {
	return newpty.StartWithSize(c, sz)
}
