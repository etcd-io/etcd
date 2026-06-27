package termutil

import (
	"errors"
	"os"
	"os/signal"
	"sync"
)

var echoLocked bool
var echoLockMutex sync.Mutex
var errLocked = errors.New("terminal locked")
var autoTerminate = true

// AutoTerminate enables or disables automatic terminate signal catching.
// It's needed to restore the terminal state after the pool was used.
// By default, it's enabled.
func AutoTerminate(enable bool) {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()
	autoTerminate = enable
}

// RawModeOn switches terminal to raw mode
func RawModeOn() (quit chan struct{}, err error) {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()
	if echoLocked {
		err = errLocked
		return
	}
	if err = lockEcho(); err != nil {
		return
	}
	echoLocked = true
	quit = make(chan struct{}, 1)
	go catchTerminate(quit)
	return
}

// RawModeOff restore previous terminal state
func RawModeOff() (err error) {
	echoLockMutex.Lock()
	defer echoLockMutex.Unlock()
	if !echoLocked {
		return
	}
	if err = unlockEcho(); err != nil {
		return
	}
	echoLocked = false
	return
}

// listen exit signals and restore terminal state
func catchTerminate(quit chan struct{}) {
	sig := make(chan os.Signal, 1)
	if autoTerminate {
		signal.Notify(sig, unlockSignals...)
		defer signal.Stop(sig)
	}
	select {
	case <-quit:
		RawModeOff()
	case <-sig:
		RawModeOff()
	}
}
