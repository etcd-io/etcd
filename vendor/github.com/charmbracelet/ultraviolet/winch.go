package uv

import (
	"os"
	"sync"

	"github.com/charmbracelet/x/term"
)

// SizeNotifier represents a notifier that listens for window size
// changes using the SIGWINCH signal and notifies the given channel.
type SizeNotifier struct {
	// Channel that receives terminal size change notifications.
	C <-chan os.Signal

	f   term.File
	sig chan os.Signal
	m   sync.Mutex
}

// NewSizeNotifier creates a new [SizeNotifier] that listens for window size
// changes on the given TTY file through SIGWINCH signals.
func NewSizeNotifier(f term.File) *SizeNotifier {
	if f == nil {
		panic("no file set")
	}
	sig := make(chan os.Signal)
	return &SizeNotifier{
		f:   f,
		sig: sig,
		C:   sig,
	}
}

// Start starts listening for window size changes and notifies [SizeNotifier.C]
// about any changes.
func (n *SizeNotifier) Start() error {
	return n.start()
}

// Stop stops the notifier and cleans up resources.
func (n *SizeNotifier) Stop() error {
	return n.stop()
}

// GetWindowSize returns the current size of the terminal window.
func (n *SizeNotifier) GetWindowSize() (cells Size, pixels Size, err error) {
	return n.getWindowSize()
}

// GetSize returns the current cell size of the terminal window.
func (n *SizeNotifier) GetSize() (width, height int, err error) {
	n.m.Lock()
	defer n.m.Unlock()

	width, height, err = term.GetSize(n.f.Fd())
	if err != nil {
		return 0, 0, err //nolint:wrapcheck
	}

	return width, height, nil
}
