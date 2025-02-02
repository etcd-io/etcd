// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
