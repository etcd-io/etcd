// Copyright 2015 The etcd Authors
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

package fileutil

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockAndUnlock(t *testing.T) {
	f, err := os.CreateTemp("", "lock")
	require.NoError(t, err)
	f.Close()
	defer func() {
		require.NoError(t, os.Remove(f.Name()))
	}()

	// lock the file
	l, err := LockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
	require.NoError(t, err)

	// try lock a locked file
	_, err = TryLockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
	require.ErrorIs(t, err, ErrLocked)

	// unlock the file
	require.NoError(t, l.Close())

	// try lock the unlocked file
	dupl, err := TryLockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
	if err != nil {
		t.Errorf("err = %v, want %v", err, nil)
	}

	// blocking on locked file
	locked := make(chan struct{}, 1)
	go func() {
		bl, blerr := LockFile(f.Name(), os.O_WRONLY, PrivateFileMode)
		if blerr != nil {
			t.Error(blerr)
		}
		locked <- struct{}{}
		if blerr = bl.Close(); blerr != nil {
			t.Error(blerr)
		}
	}()

	select {
	case <-locked:
		t.Error("unexpected unblocking")
	case <-time.After(100 * time.Millisecond):
	}

	// unlock
	require.NoError(t, dupl.Close())

	// the previously blocked routine should be unblocked
	select {
	case <-locked:
	case <-time.After(1 * time.Second):
		t.Error("unexpected blocking")
	}
}
