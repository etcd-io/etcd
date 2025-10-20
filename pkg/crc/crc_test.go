// Copyright 2025 The etcd Authors
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

package crc

import (
	"hash/crc32"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHash32 tests that Hash32 provided by this package can take an initial
// crc and behaves exactly the same as the standard one in the following calls.
func TestHash32(t *testing.T) {
	stdhash := crc32.New(crc32.IEEETable)
	_, err := stdhash.Write([]byte("test data"))
	require.NoErrorf(t, err, "unexpected write error: %v", err)
	// create a new hash with stdhash.Sum32() as initial crc
	hash := New(stdhash.Sum32(), crc32.IEEETable)

	assert.Equalf(t, hash.Size(), stdhash.Size(), "size")
	assert.Equalf(t, hash.BlockSize(), stdhash.BlockSize(), "block size")
	assert.Equalf(t, hash.Sum32(), stdhash.Sum32(), "Sum32")
	wsum := stdhash.Sum(make([]byte, 32))
	g := hash.Sum(make([]byte, 32))
	assert.Truef(t, reflect.DeepEqual(g, wsum), "sum")

	// write something
	_, err = stdhash.Write([]byte("test data"))
	require.NoErrorf(t, err, "unexpected write error: %v", err)
	_, err = hash.Write([]byte("test data"))
	require.NoErrorf(t, err, "unexpected write error: %v", err)
	assert.Equalf(t, hash.Sum32(), stdhash.Sum32(), "Sum32 after write")

	// reset
	stdhash.Reset()
	hash.Reset()
	assert.Equalf(t, hash.Sum32(), stdhash.Sum32(), "Sum32 after reset")
}
