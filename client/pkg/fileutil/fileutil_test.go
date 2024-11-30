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
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestIsDirWriteable(t *testing.T) {
	tmpdir := t.TempDir()
	require.NoErrorf(t, IsDirWriteable(tmpdir), "unexpected IsDirWriteable error")
	require.NoErrorf(t, os.Chmod(tmpdir, 0o444), "unexpected os.Chmod error")
	me, err := user.Current()
	if err != nil {
		// err can be non-nil when cross compiled
		// http://stackoverflow.com/questions/20609415/cross-compiling-user-current-not-implemented-on-linux-amd64
		t.Skipf("failed to get current user: %v", err)
	}
	if me.Name == "root" || runtime.GOOS == "windows" {
		// ideally we should check CAP_DAC_OVERRIDE.
		// but it does not matter for tests.
		// Chmod is not supported under windows.
		t.Skipf("running as a superuser or in windows")
	}
	require.Errorf(t, IsDirWriteable(tmpdir), "expected IsDirWriteable to error")
}

func TestCreateDirAll(t *testing.T) {
	tmpdir := t.TempDir()

	tmpdir2 := filepath.Join(tmpdir, "testdir")
	require.NoError(t, CreateDirAll(zaptest.NewLogger(t), tmpdir2))

	require.NoError(t, os.WriteFile(filepath.Join(tmpdir2, "text.txt"), []byte("test text"), PrivateFileMode))

	if err := CreateDirAll(zaptest.NewLogger(t), tmpdir2); err == nil || !strings.Contains(err.Error(), "to be empty, got") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestExist(t *testing.T) {
	fdir := filepath.Join(os.TempDir(), fmt.Sprint(time.Now().UnixNano()+rand.Int63n(1000)))
	os.RemoveAll(fdir)
	if err := os.Mkdir(fdir, 0o666); err != nil {
		t.Skip(err)
	}
	defer os.RemoveAll(fdir)
	require.Truef(t, Exist(fdir), "expected Exist true, got %v", Exist(fdir))

	f, err := os.CreateTemp(os.TempDir(), "fileutil")
	require.NoError(t, err)
	f.Close()

	if g := Exist(f.Name()); !g {
		t.Errorf("exist = %v, want true", g)
	}

	os.Remove(f.Name())
	if g := Exist(f.Name()); g {
		t.Errorf("exist = %v, want false", g)
	}
}

func TestDirEmpty(t *testing.T) {
	dir := t.TempDir()

	require.Truef(t, DirEmpty(dir), "expected DirEmpty true, got %v", DirEmpty(dir))

	file, err := os.CreateTemp(dir, "new_file")
	require.NoError(t, err)
	file.Close()

	require.Falsef(t, DirEmpty(dir), "expected DirEmpty false, got %v", DirEmpty(dir))
	require.Falsef(t, DirEmpty(file.Name()), "expected DirEmpty false, got %v", DirEmpty(file.Name()))
}

func TestZeroToEnd(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "fileutil")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	// Ensure 0 size is a nop so zero-to-end on an empty file won't give EINVAL.
	require.NoError(t, ZeroToEnd(f))

	b := make([]byte, 1024)
	for i := range b {
		b[i] = 12
	}
	_, err = f.Write(b)
	require.NoError(t, err)
	_, err = f.Seek(512, io.SeekStart)
	require.NoError(t, err)
	require.NoError(t, ZeroToEnd(f))
	off, serr := f.Seek(0, io.SeekCurrent)
	require.NoError(t, serr)
	require.Equalf(t, int64(512), off, "expected offset 512, got %d", off)

	b = make([]byte, 512)
	_, err = f.Read(b)
	require.NoError(t, err)
	for i := range b {
		if b[i] != 0 {
			t.Errorf("expected b[%d] = 0, got %d", i, b[i])
		}
	}
}

func TestDirPermission(t *testing.T) {
	tmpdir := t.TempDir()

	tmpdir2 := filepath.Join(tmpdir, "testpermission")
	// create a new dir with 0700
	require.NoError(t, CreateDirAll(zaptest.NewLogger(t), tmpdir2))
	// check dir permission with mode different than created dir
	if err := CheckDirPermission(tmpdir2, 0o600); err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestRemoveMatchFile(t *testing.T) {
	tmpdir := t.TempDir()
	f, err := os.CreateTemp(tmpdir, "tmp")
	require.NoError(t, err)
	f.Close()
	f, err = os.CreateTemp(tmpdir, "foo.tmp")
	require.NoError(t, err)
	f.Close()

	err = RemoveMatchFile(zaptest.NewLogger(t), tmpdir, func(fileName string) bool {
		return strings.HasPrefix(fileName, "tmp")
	})
	if err != nil {
		t.Errorf("expected nil, got error")
	}
	fnames, err := ReadDir(tmpdir)
	require.NoError(t, err)
	if len(fnames) != 1 {
		t.Errorf("expected exist 1 files, got %d", len(fnames))
	}

	f, err = os.CreateTemp(tmpdir, "tmp")
	require.NoError(t, err)
	f.Close()
	err = RemoveMatchFile(zaptest.NewLogger(t), tmpdir, func(fileName string) bool {
		os.Remove(filepath.Join(tmpdir, fileName))
		return strings.HasPrefix(fileName, "tmp")
	})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestTouchDirAll(t *testing.T) {
	tmpdir := t.TempDir()
	assert.Panicsf(t, func() {
		TouchDirAll(nil, tmpdir)
	}, "expected panic with nil log")

	assert.NoError(t, TouchDirAll(zaptest.NewLogger(t), tmpdir))
}
