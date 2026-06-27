package termutil

import (
	"errors"
	"os"
	"syscall"
)

var (
	consctl *os.File

	// Plan 9 doesn't have syscall.SIGQUIT
	unlockSignals = []os.Signal{
		os.Interrupt, syscall.SIGTERM, syscall.SIGKILL,
	}
)

// TerminalWidth returns width of the terminal.
func TerminalWidth() (int, error) {
	return 0, errors.New("Not supported")
}

func lockEcho() error {
	if consctl != nil {
		return errors.New("consctl already open")
	}
	var err error
	consctl, err = os.OpenFile("/dev/consctl", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = consctl.WriteString("rawon")
	if err != nil {
		consctl.Close()
		consctl = nil
		return err
	}
	return nil
}

func unlockEcho() error {
	if consctl == nil {
		return nil
	}
	if err := consctl.Close(); err != nil {
		return err
	}
	consctl = nil
	return nil
}
