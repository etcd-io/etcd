// +build !windows,!plan9

package fileutil

import (
	"errors"
	"os"
	"syscall"
)

var (
	ErrLocked = errors.New("file already locked")
)

type Lock interface {
	Name() string
	TryLock() error
	Lock() error
	Unlock() error
	Destroy() error
}

type lock struct {
	fd   int
	file *os.File
}

func (l *lock) Name() string {
	return l.file.Name()
}

// TryLock acquires exclusivity on the lock without blocking
func (l *lock) TryLock() error {
	err := syscall.Flock(l.fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil && err == syscall.EWOULDBLOCK {
		return ErrLocked
	}
	return err
}

// Lock acquires exclusivity on the lock without blocking
func (l *lock) Lock() error {
	return syscall.Flock(l.fd, syscall.LOCK_EX)
}

// Unlock unlocks the lock
func (l *lock) Unlock() error {
	return syscall.Flock(l.fd, syscall.LOCK_UN)
}

func (l *lock) Destroy() error {
	return l.file.Close()
}

func NewLock(file string) (Lock, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	l := &lock{int(f.Fd()), f}
	return l, nil
}
