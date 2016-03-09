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
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	ErrLocked = errors.New("The process cannot access the file because another process has locked a portion of the file.")
)

type Lock interface {
	Name() string
	TryLock() error
	Lock() error
	Unlock() error
	Destroy() error
}

type lock struct {
	filePath string
	lockPath string
}

func (l *lock) Name() string {
	return l.filePath
}

// TryLock acquires exclusivity on the lock without blocking.
func (l *lock) TryLock() error {
	if Exist(l.lockPath) {
		return ErrLocked
	}
	return ioutil.WriteFile(l.lockPath, []byte(""), privateFileMode)
}

// Lock acquires exclusivity on the lock.
func (l *lock) Lock() error {
	for Exist(l.lockPath) {
	}
	return ioutil.WriteFile(l.lockPath, []byte(""), privateFileMode)
}

// Unlock unlocks the lock
func (l *lock) Unlock() error {
	return os.Remove(l.lockPath)
}

func (l *lock) Destroy() error {
	if Exist(l.lockPath) {
		return ErrLocked
	}
	return nil
}

func NewLock(file string) (Lock, error) {
	fbase := filepath.Base(file)
	return &lock{filePath: file, lockPath: filepath.Join(filepath.Dir(file), "."+fbase+".etcd_fileutil_windows_flock")}, nil
}
