// Copyright 2018 The etcd Authors
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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadDir(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)
	if err != nil {
		t.Fatalf("unexpected ioutil.TempDir error: %v", err)
	}

	files := []string{"def", "abc", "xyz", "ghi"}
	for _, f := range files {
		writeFunc(t, filepath.Join(tmpdir, f))
	}
	fs, err := ReadDir(tmpdir)
	if err != nil {
		t.Fatalf("error calling ReadDir: %v", err)
	}
	wfs := []string{"abc", "def", "ghi", "xyz"}
	if !reflect.DeepEqual(fs, wfs) {
		t.Fatalf("ReadDir: got %v, want %v", fs, wfs)
	}

	files = []string{"def.wal", "abc.wal", "xyz.wal", "ghi.wal"}
	for _, f := range files {
		writeFunc(t, filepath.Join(tmpdir, f))
	}
	fs, err = ReadDir(tmpdir, WithExt(".wal"))
	if err != nil {
		t.Fatalf("error calling ReadDir: %v", err)
	}
	wfs = []string{"abc.wal", "def.wal", "ghi.wal", "xyz.wal"}
	if !reflect.DeepEqual(fs, wfs) {
		t.Fatalf("ReadDir: got %v, want %v", fs, wfs)
	}
}

func writeFunc(t *testing.T, path string) {
	fh, err := os.Create(path)
	if err != nil {
		t.Fatalf("error creating file: %v", err)
	}
	if err = fh.Close(); err != nil {
		t.Fatalf("error closing file: %v", err)
	}
}
