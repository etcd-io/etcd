//go:build darwin || freebsd || netbsd || openbsd || dragonfly
// +build darwin freebsd netbsd openbsd dragonfly

package cancelreader

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// NewReader returns a reader and a cancel function. If the input reader is a
// File, the cancel function can be used to interrupt a blocking read call.
// In this case, the cancel function returns true if the call was canceled
// successfully. If the input reader is not a File, the cancel function
// does nothing and always returns false. The BSD and macOS implementation is
// based on the kqueue mechanism.
func NewReader(reader io.Reader) (CancelReader, error) {
	file, ok := reader.(File)
	if !ok {
		return newFallbackCancelReader(reader)
	}

	// kqueue returns instantly when polling /dev/tty so fallback to select
	if file.Name() == "/dev/tty" {
		return newSelectCancelReader(reader)
	}

	kQueue, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("create kqueue: %w", err)
	}

	r := &kqueueCancelReader{
		file:   file,
		kQueue: kQueue,
	}

	r.cancelSignalReader, r.cancelSignalWriter, err = os.Pipe()
	if err != nil {
		_ = unix.Close(kQueue)
		return nil, err
	}

	unix.SetKevent(&r.kQueueEvents[0], int(file.Fd()), unix.EVFILT_READ, unix.EV_ADD)
	unix.SetKevent(&r.kQueueEvents[1], int(r.cancelSignalReader.Fd()), unix.EVFILT_READ, unix.EV_ADD)

	return r, nil
}

type kqueueCancelReader struct {
	file               File
	cancelSignalReader File
	cancelSignalWriter File
	cancelMixin
	kQueue       int
	kQueueEvents [2]unix.Kevent_t
}

func (r *kqueueCancelReader) Read(data []byte) (int, error) {
	if r.isCanceled() {
		return 0, ErrCanceled
	}

	err := r.wait()
	if err != nil {
		if errors.Is(err, ErrCanceled) {
			// remove signal from pipe
			var b [1]byte
			_, errRead := r.cancelSignalReader.Read(b[:])
			if errRead != nil {
				return 0, fmt.Errorf("reading cancel signal: %w", errRead)
			}
		}

		return 0, err
	}

	return r.file.Read(data)
}

func (r *kqueueCancelReader) Cancel() bool {
	r.setCanceled()

	// send cancel signal
	_, err := r.cancelSignalWriter.Write([]byte{'c'})
	return err == nil
}

func (r *kqueueCancelReader) Close() error {
	var errMsgs []string

	// close kqueue
	err := unix.Close(r.kQueue)
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("closing kqueue: %v", err))
	}

	// close pipe
	err = r.cancelSignalWriter.Close()
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("closing cancel signal writer: %v", err))
	}

	err = r.cancelSignalReader.Close()
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("closing cancel signal reader: %v", err))
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf(strings.Join(errMsgs, ", "))
	}

	return nil
}

func (r *kqueueCancelReader) wait() error {
	events := make([]unix.Kevent_t, 1)

	for {
		_, err := unix.Kevent(r.kQueue, r.kQueueEvents[:], events, nil)
		if errors.Is(err, unix.EINTR) {
			continue // try again if the syscall was interrupted
		}

		if err != nil {
			return fmt.Errorf("kevent: %w", err)
		}

		break
	}

	ident := uint64(events[0].Ident)
	switch ident {
	case uint64(r.file.Fd()):
		return nil
	case uint64(r.cancelSignalReader.Fd()):
		return ErrCanceled
	}

	return fmt.Errorf("unknown error")
}
