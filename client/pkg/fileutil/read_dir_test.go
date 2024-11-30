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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadDir(t *testing.T) {
	tmpdir := t.TempDir()

	files := []string{"def", "abc", "xyz", "ghi"}
	for _, f := range files {
		writeFunc(t, filepath.Join(tmpdir, f))
	}
	fs, err := ReadDir(tmpdir)
	require.NoErrorf(t, err, "error calling ReadDir")
	wfs := []string{"abc", "def", "ghi", "xyz"}
	require.Truef(t, reflect.DeepEqual(fs, wfs), "ReadDir: got %v, want %v", fs, wfs)

	files = []string{"def.wal", "abc.wal", "xyz.wal", "ghi.wal"}
	for _, f := range files {
		writeFunc(t, filepath.Join(tmpdir, f))
	}
	fs, err = ReadDir(tmpdir, WithExt(".wal"))
	require.NoErrorf(t, err, "error calling ReadDir")
	wfs = []string{"abc.wal", "def.wal", "ghi.wal", "xyz.wal"}
	require.Truef(t, reflect.DeepEqual(fs, wfs), "ReadDir: got %v, want %v", fs, wfs)
}

func writeFunc(t *testing.T, path string) {
	t.Helper()
	fh, err := os.Create(path)
	require.NoErrorf(t, err, "error creating file")
	assert.NoErrorf(t, fh.Close(), "error closing file")
}
