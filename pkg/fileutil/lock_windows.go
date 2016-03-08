// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build windows

package fileutil

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")

	ErrTimeout = errors.New("timeout")
	ErrLocked  = errors.New("The process cannot access the file because another process has locked a portion of the file.")
)

const (
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365203(v=vs.85).aspx
	LOCKFILE_EXCLUSIVE_LOCK   = 2
	LOCKFILE_FAIL_IMMEDIATELY = 1

	// see https://msdn.microsoft.com/en-us/library/windows/desktop/ms681382(v=vs.85).aspx
	errLockViolation syscall.Errno = 0x21
)

type Lock interface {
	Name() string
	TryLock() error
	Lock() error
	Unlock() error
	Destroy() error
}

// fileMutex is similar to sync.RWMutex, but also synchronizes across processes.
// This implementation is based on flock syscall.
// https://github.com/golang/build/blob/master/cmd/builder/filemutex_windows.go
type fileMutex struct {
	mu       sync.RWMutex
	fd       syscall.Handle
	filename string
}

func lockFileEx(h syscall.Handle, flags, reserved, locklow, lockhigh uint32, ol *syscall.Overlapped) (err error) {
	r1, _, e1 := syscall.Syscall6(procLockFileEx.Addr(), 6, uintptr(h), uintptr(flags), uintptr(reserved), uintptr(locklow), uintptr(lockhigh), uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func unlockFileEx(h syscall.Handle, locklow, lockhigh uint32, ol *syscall.Overlapped) (err error) {
	var reserved uint32 = 0
	r1, _, e1 := syscall.Syscall6(procUnlockFileEx.Addr(), 5, uintptr(h), uintptr(reserved), uintptr(locklow), uintptr(lockhigh), uintptr(unsafe.Pointer(ol)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// NewLock creates a NewLock.
func NewLock(file string) (Lock, error) {
	if file == "" {
		return &fileMutex{fd: syscall.InvalidHandle, filename: file}, fmt.Errorf("cannot open empty filename")
	}
	fd, err := syscall.CreateFile(&(syscall.StringToUTF16(file)[0]),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		return nil, err
	}
	return &fileMutex{fd: fd, filename: file}, nil
}

// TryLock acquires exclusivity on the lock without blocking.
func (fm *fileMutex) TryLock() error {
	err := fm.lockFile(LOCKFILE_FAIL_IMMEDIATELY)
	if err != nil && err != ErrLocked {
		fm.mu.Unlock()
	}
	return err
}

// Lock acquires exclusivity on the lock.
// It blocks until the lock is acquired.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365202(v=vs.85).aspx
func (fm *fileMutex) Lock() error {
	err := fm.lockFile(0)
	if err != nil && err != ErrLocked {
		fm.mu.Unlock()
	}
	return err
}

// Unlock unlocks the lock.
func (fm *fileMutex) Unlock() error {
	if fm.fd != syscall.InvalidHandle {
		if err := unlockFileEx(fm.fd, 1, 0, &syscall.Overlapped{}); err != nil {
			return err
		}
	}
	fm.mu.Unlock()
	return nil
}

func (fm *fileMutex) lockFile(flags uint32) error {
	fm.mu.Lock()
	var flag uint32 = LOCKFILE_EXCLUSIVE_LOCK
	if flags != 0 {
		flag |= flags
	}
	if fm.fd == syscall.InvalidHandle {
		return nil
	}
	for {
		err := lockFileEx(fm.fd, flag, 0, 1, 0, &syscall.Overlapped{})
		if err == nil {
			return nil
		} else if err.Error() == ErrLocked.Error() {
			return ErrLocked
		} else if err != errLockViolation {
			return err
		}

		// Wait for a bit and try again.
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Name returns the filename.
func (fm *fileMutex) Name() string {
	return fm.filename
}

// Destroy closes the file handle.
func (fm *fileMutex) Destroy() error {
	return syscall.CloseHandle(fm.fd)
}
