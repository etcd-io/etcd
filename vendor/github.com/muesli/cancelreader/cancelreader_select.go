//go:build solaris || darwin || freebsd || netbsd || openbsd || dragonfly
// +build solaris darwin freebsd netbsd openbsd dragonfly

package cancelreader

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// newSelectCancelReader returns a reader and a cancel function. If the input
// reader is a File, the cancel function can be used to interrupt a
// blocking call read call. In this case, the cancel function returns true if
// the call was canceled successfully. If the input reader is not a File or
// the file descriptor is 1024 or larger, the cancel function does nothing and
// always returns false. The generic unix implementation is based on the posix
// select syscall.
func newSelectCancelReader(reader io.Reader) (CancelReader, error) {
	file, ok := reader.(File)
	if !ok || file.Fd() >= unix.FD_SETSIZE {
		return newFallbackCancelReader(reader)
	}
	r := &selectCancelReader{file: file}

	var err error

	r.cancelSignalReader, r.cancelSignalWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}

	return r, nil
}

type selectCancelReader struct {
	file               File
	cancelSignalReader File
	cancelSignalWriter File
	cancelMixin
}

func (r *selectCancelReader) Read(data []byte) (int, error) {
	if r.isCanceled() {
		return 0, ErrCanceled
	}

	for {
		err := waitForRead(r.file, r.cancelSignalReader)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue // try again if the syscall was interrupted
			}

			if errors.Is(err, ErrCanceled) {
				// remove signal from pipe
				var b [1]byte
				_, readErr := r.cancelSignalReader.Read(b[:])
				if readErr != nil {
					return 0, fmt.Errorf("reading cancel signal: %w", readErr)
				}
			}

			return 0, err
		}

		return r.file.Read(data)
	}
}

func (r *selectCancelReader) Cancel() bool {
	r.setCanceled()

	// send cancel signal
	_, err := r.cancelSignalWriter.Write([]byte{'c'})
	return err == nil
}

func (r *selectCancelReader) Close() error {
	var errMsgs []string

	// close pipe
	err := r.cancelSignalWriter.Close()
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

func waitForRead(reader, abort File) error {
	readerFd := int(reader.Fd())
	abortFd := int(abort.Fd())

	maxFd := readerFd
	if abortFd > maxFd {
		maxFd = abortFd
	}

	// this is a limitation of the select syscall
	if maxFd >= unix.FD_SETSIZE {
		return fmt.Errorf("cannot select on file descriptor %d which is larger than 1024", maxFd)
	}

	fdSet := &unix.FdSet{}
	fdSet.Set(int(reader.Fd()))
	fdSet.Set(int(abort.Fd()))

	_, err := unix.Select(maxFd+1, fdSet, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}

	if fdSet.IsSet(abortFd) {
		return ErrCanceled
	}

	if fdSet.IsSet(readerFd) {
		return nil
	}

	return fmt.Errorf("select returned without setting a file descriptor")
}
