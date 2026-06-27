//go:build windows
// +build windows

package cancelreader

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

var fileShareValidFlags uint32 = 0x00000007

// NewReader returns a reader and a cancel function. If the input reader is a
// File with the same file descriptor as os.Stdin, the cancel function can
// be used to interrupt a blocking read call. In this case, the cancel function
// returns true if the call was canceled successfully. If the input reader is
// not a File with the same file descriptor as os.Stdin, the cancel
// function does nothing and always returns false. The Windows implementation
// is based on WaitForMultipleObject with overlapping reads from CONIN$.
func NewReader(reader io.Reader) (CancelReader, error) {
	if f, ok := reader.(File); !ok || f.Fd() != os.Stdin.Fd() {
		return newFallbackCancelReader(reader)
	}

	// it is necessary to open CONIN$ (NOT windows.STD_INPUT_HANDLE) in
	// overlapped mode to be able to use it with WaitForMultipleObjects.
	conin, err := windows.CreateFile(
		&(utf16.Encode([]rune("CONIN$\x00"))[0]), windows.GENERIC_READ|windows.GENERIC_WRITE,
		fileShareValidFlags, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("open CONIN$ in overlapping mode: %w", err)
	}

	resetConsole, err := prepareConsole(conin)
	if err != nil {
		return nil, fmt.Errorf("prepare console: %w", err)
	}

	// flush input, otherwise it can contain events which trigger
	// WaitForMultipleObjects but which ReadFile cannot read, resulting in an
	// un-cancelable read
	err = flushConsoleInputBuffer(conin)
	if err != nil {
		return nil, fmt.Errorf("flush console input buffer: %w", err)
	}

	cancelEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("create stop event: %w", err)
	}

	return &winCancelReader{
		conin:              conin,
		cancelEvent:        cancelEvent,
		resetConsole:       resetConsole,
		blockingReadSignal: make(chan struct{}, 1),
	}, nil
}

type winCancelReader struct {
	conin       windows.Handle
	cancelEvent windows.Handle
	cancelMixin

	resetConsole       func() error
	blockingReadSignal chan struct{}
}

func (r *winCancelReader) Read(data []byte) (int, error) {
	if r.isCanceled() {
		return 0, ErrCanceled
	}

	err := r.wait()
	if err != nil {
		return 0, err
	}

	if r.isCanceled() {
		return 0, ErrCanceled
	}

	// windows.Read does not work on overlapping windows.Handles
	return r.readAsync(data)
}

// Cancel cancels ongoing and future Read() calls and returns true if the
// cancelation of the ongoing Read() was successful. On Windows Terminal,
// WaitForMultipleObjects sometimes immediately returns without input being
// available. In this case, graceful cancelation is not possible and Cancel()
// returns false.
func (r *winCancelReader) Cancel() bool {
	r.setCanceled()

	select {
	case r.blockingReadSignal <- struct{}{}:
		err := windows.SetEvent(r.cancelEvent)
		if err != nil {
			return false
		}
		<-r.blockingReadSignal
	case <-time.After(100 * time.Millisecond):
		// Read() hangs in a GetOverlappedResult which is likely due to
		// WaitForMultipleObjects returning without input being available
		// so we cannot cancel this ongoing read.
		return false
	}

	return true
}

func (r *winCancelReader) Close() error {
	err := windows.CloseHandle(r.cancelEvent)
	if err != nil {
		return fmt.Errorf("closing cancel event handle: %w", err)
	}

	err = r.resetConsole()
	if err != nil {
		return err
	}

	err = windows.Close(r.conin)
	if err != nil {
		return fmt.Errorf("closing CONIN$")
	}

	return nil
}

func (r *winCancelReader) wait() error {
	event, err := windows.WaitForMultipleObjects([]windows.Handle{r.conin, r.cancelEvent}, false, windows.INFINITE)
	switch {
	case windows.WAIT_OBJECT_0 <= event && event < windows.WAIT_OBJECT_0+2:
		if event == windows.WAIT_OBJECT_0+1 {
			return ErrCanceled
		}

		if event == windows.WAIT_OBJECT_0 {
			return nil
		}

		return fmt.Errorf("unexpected wait object is ready: %d", event-windows.WAIT_OBJECT_0)
	case windows.WAIT_ABANDONED <= event && event < windows.WAIT_ABANDONED+2:
		return fmt.Errorf("abandoned")
	case event == uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("timeout")
	case event == windows.WAIT_FAILED:
		return fmt.Errorf("failed")
	default:
		return fmt.Errorf("unexpected error: %w", error(err))
	}
}

// readAsync is necessary to read from a windows.Handle in overlapping mode.
func (r *winCancelReader) readAsync(data []byte) (int, error) {
	hevent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("create event: %w", err)
	}

	overlapped := windows.Overlapped{
		HEvent: hevent,
	}

	var n uint32

	err = windows.ReadFile(r.conin, data, &n, &overlapped)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return int(n), err
	}

	r.blockingReadSignal <- struct{}{}
	err = windows.GetOverlappedResult(r.conin, &overlapped, &n, true)
	if err != nil {
		return int(n), nil
	}
	<-r.blockingReadSignal

	return int(n), nil
}

func prepareConsole(input windows.Handle) (reset func() error, err error) {
	var originalMode uint32

	err = windows.GetConsoleMode(input, &originalMode)
	if err != nil {
		return nil, fmt.Errorf("get console mode: %w", err)
	}

	var newMode uint32
	newMode &^= windows.ENABLE_ECHO_INPUT
	newMode &^= windows.ENABLE_LINE_INPUT
	newMode &^= windows.ENABLE_MOUSE_INPUT
	newMode &^= windows.ENABLE_WINDOW_INPUT
	newMode &^= windows.ENABLE_PROCESSED_INPUT

	newMode |= windows.ENABLE_EXTENDED_FLAGS
	newMode |= windows.ENABLE_INSERT_MODE
	newMode |= windows.ENABLE_QUICK_EDIT_MODE

	// Enabling virtual terminal input is necessary for processing certain
	// types of input like X10 mouse events and arrows keys with the current
	// bytes-based input reader. It does, however, prevent cancelReader from
	// being able to cancel input. The planned solution for this is to read
	// Windows events in a more native fashion, rather than the current simple
	// bytes-based input reader which works well on unix systems.
	newMode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	err = windows.SetConsoleMode(input, newMode)
	if err != nil {
		return nil, fmt.Errorf("set console mode: %w", err)
	}

	return func() error {
		err := windows.SetConsoleMode(input, originalMode)
		if err != nil {
			return fmt.Errorf("reset console mode: %w", err)
		}

		return nil
	}, nil
}

var (
	modkernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procFlushConsoleInputBuffer = modkernel32.NewProc("FlushConsoleInputBuffer")
)

func flushConsoleInputBuffer(consoleInput windows.Handle) error {
	r, _, e := syscall.Syscall(procFlushConsoleInputBuffer.Addr(), 1,
		uintptr(consoleInput), 0, 0)
	if r == 0 {
		return error(e)
	}

	return nil
}
