// Copyright 2021 The etcd Authors
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

//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris zos

package etcdserver

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// MlockAll prevents current and future mmaped memory areas from being swapped out.
func MlockAll() error {
	err := unix.Mlockall(unix.MCL_FUTURE | unix.MCL_CURRENT)
	if err != nil {
		return fmt.Errorf("cannot mlockAll: %v", err)
	}
	return nil
}
