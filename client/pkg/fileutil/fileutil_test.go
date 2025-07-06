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
	"io/ioutil"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestIsDirWriteable(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected ioutil.TempDir error: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	if err = IsDirWriteable(tmpdir); err != nil {
		t.Fatalf("unexpected IsDirWriteable error: %v", err)
	}
	if err = os.Chmod(tmpdir, 0444); err != nil {
		t.Fatalf("unexpected os.Chmod error: %v", err)
	}
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
	if err := IsDirWriteable(tmpdir); err == nil {
		t.Fatalf("expected IsDirWriteable to error")
	}
}

func TestCreateDirAll(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	tmpdir2 := filepath.Join(tmpdir, "testdir")
	if err = CreateDirAll(tmpdir2); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(tmpdir2, "text.txt"), []byte("test text"), PrivateFileMode); err != nil {
		t.Fatal(err)
	}

	if err = CreateDirAll(tmpdir2); err == nil || !strings.Contains(err.Error(), "to be empty, got") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestExist(t *testing.T) {
	fdir := filepath.Join(os.TempDir(), fmt.Sprint(time.Now().UnixNano()+rand.Int63n(1000)))
	os.RemoveAll(fdir)
	if err := os.Mkdir(fdir, 0666); err != nil {
		t.Skip(err)
	}
	defer os.RemoveAll(fdir)
	if !Exist(fdir) {
		t.Fatalf("expected Exist true, got %v", Exist(fdir))
	}

	f, err := ioutil.TempFile(os.TempDir(), "fileutil")
	if err != nil {
		t.Fatal(err)
	}
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
	dir, err := ioutil.TempDir(os.TempDir(), "empty_dir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if !DirEmpty(dir) {
		t.Fatalf("expected DirEmpty true, got %v", DirEmpty(dir))
	}

	file, err := ioutil.TempFile(dir, "new_file")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	if DirEmpty(dir) {
		t.Fatalf("expected DirEmpty false, got %v", DirEmpty(dir))
	}
	if DirEmpty(file.Name()) {
		t.Fatalf("expected DirEmpty false, got %v", DirEmpty(file.Name()))
	}
}

func TestZeroToEnd(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "fileutil")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	// Ensure 0 size is a nop so zero-to-end on an empty file won't give EINVAL.
	if err = ZeroToEnd(f); err != nil {
		t.Fatal(err)
	}

	b := make([]byte, 1024)
	for i := range b {
		b[i] = 12
	}
	if _, err = f.Write(b); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Seek(512, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err = ZeroToEnd(f); err != nil {
		t.Fatal(err)
	}
	off, serr := f.Seek(0, io.SeekCurrent)
	if serr != nil {
		t.Fatal(serr)
	}
	if off != 512 {
		t.Fatalf("expected offset 512, got %d", off)
	}

	b = make([]byte, 512)
	if _, err = f.Read(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		if b[i] != 0 {
			t.Errorf("expected b[%d] = 0, got %d", i, b[i])
		}
	}
}

func TestDirPermission(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	tmpdir2 := filepath.Join(tmpdir, "testpermission")
	// create a new dir with 0700
	if err = CreateDirAll(tmpdir2); err != nil {
		t.Fatal(err)
	}
	// check dir permission with mode different than created dir
	if err = CheckDirPermission(tmpdir2, 0600); err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestRemoveMatchFile(t *testing.T) {
	tmpdir := t.TempDir()
	defer os.RemoveAll(tmpdir)
	f, err := ioutil.TempFile(tmpdir, "tmp")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	f, err = ioutil.TempFile(tmpdir, "foo.tmp")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	err = RemoveMatchFile(zaptest.NewLogger(t), tmpdir, func(fileName string) bool {
		return strings.HasPrefix(fileName, "tmp")
	})
	if err != nil {
		t.Errorf("expected nil, got error")
	}
	fnames, err := ReadDir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fnames) != 1 {
		t.Errorf("expected exist 1 files, got %d", len(fnames))
	}

	f, err = ioutil.TempFile(tmpdir, "tmp")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	err = RemoveMatchFile(zaptest.NewLogger(t), tmpdir, func(fileName string) bool {
		os.Remove(filepath.Join(tmpdir, fileName))
		return strings.HasPrefix(fileName, "tmp")
	})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
