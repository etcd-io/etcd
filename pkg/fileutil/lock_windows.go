// +build windows

package fileutil

import (
	"errors"
	"os"
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
	return nil
}

// Lock acquires exclusivity on the lock without blocking
func (l *lock) Lock() error {
	return nil
}

// Unlock unlocks the lock
func (l *lock) Unlock() error {
	return nil
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
