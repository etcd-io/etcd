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

	"github.com/stretchr/testify/require"
)

func TestPreallocateExtend(t *testing.T) {
	pf := func(f *os.File, sz int64) error { return Preallocate(f, sz, true) }
	tf := func(t *testing.T, f *os.File) {
		t.Helper()
		testPreallocateExtend(t, f, pf)
	}
	runPreallocTest(t, tf)
}

func TestPreallocateExtendTrunc(t *testing.T) {
	tf := func(t *testing.T, f *os.File) {
		t.Helper()
		testPreallocateExtend(t, f, preallocExtendTrunc)
	}
	runPreallocTest(t, tf)
}

func testPreallocateExtend(t *testing.T, f *os.File, pf func(*os.File, int64) error) {
	t.Helper()
	size := int64(64 * 1000)
	require.NoError(t, pf(f, size))

	stat, err := f.Stat()
	require.NoError(t, err)
	if stat.Size() != size {
		t.Errorf("size = %d, want %d", stat.Size(), size)
	}
}

func TestPreallocateFixed(t *testing.T) { runPreallocTest(t, testPreallocateFixed) }
func testPreallocateFixed(t *testing.T, f *os.File) {
	t.Helper()
	size := int64(64 * 1000)
	require.NoError(t, Preallocate(f, size, false))

	stat, err := f.Stat()
	require.NoError(t, err)
	if stat.Size() != 0 {
		t.Errorf("size = %d, want %d", stat.Size(), 0)
	}
}

func runPreallocTest(t *testing.T, test func(*testing.T, *os.File)) {
	t.Helper()
	p := t.TempDir()

	f, err := os.CreateTemp(p, "")
	require.NoError(t, err)
	test(t, f)
}
