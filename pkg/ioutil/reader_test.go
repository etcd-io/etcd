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

package ioutil

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitedBufferReaderRead(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 10))
	ln := 1
	lr := NewLimitedBufferReader(buf, ln)
	n, err := lr.Read(make([]byte, 10))
	require.NoErrorf(t, err, "unexpected read error: %v", err)
	assert.Equalf(t, n, ln, "len(data read) = %d, want %d", n, ln)
}

// NewLimitedBufferReader promises an upper bound on the total amount of data
// returned, not just on a single Read. io.ReadAll grows its buffer, so a
// per-call clamp alone lets a caller read far past the limit (the rafthttp
// handler relies on this bound to cap a request body).
func TestLimitedBufferReaderTotalLimit(t *testing.T) {
	const limit = 64
	src := bytes.NewReader(make([]byte, 4*limit))

	got, err := io.ReadAll(NewLimitedBufferReader(src, limit))
	require.NoError(t, err)
	assert.Len(t, got, limit, "total bytes read must not exceed the limit")
}

func TestLimitedBufferReaderLimitZero(t *testing.T) {
	lr := NewLimitedBufferReader(bytes.NewReader([]byte("data")), 0)
	_, err := lr.Read(make([]byte, 4))
	assert.ErrorIs(t, err, io.EOF)
}
