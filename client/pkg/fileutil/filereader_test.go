// Copyright 2022 The etcd Authors
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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileBufReader(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wal")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	fbr := NewFileBufReader(NewFileReader(f))

	if !strings.HasPrefix(fbr.FileInfo().Name(), "wal") {
		t.Errorf("Unexpected file name: %s", fbr.FileInfo().Name())
	}
	assert.Equal(t, fi.Size(), fbr.FileInfo().Size())
	assert.Equal(t, fi.IsDir(), fbr.FileInfo().IsDir())
	assert.Equal(t, fi.Mode(), fbr.FileInfo().Mode())
	assert.Equal(t, fi.ModTime(), fbr.FileInfo().ModTime())
}
