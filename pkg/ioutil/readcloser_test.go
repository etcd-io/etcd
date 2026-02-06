// Copyright 2016 The etcd Authors
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

	"github.com/stretchr/testify/require"
)

type readerNilCloser struct{ io.Reader }

func (rc *readerNilCloser) Close() error { return nil }

// TestExactReadCloserExpectEOF expects an eof when reading too much.
func TestExactReadCloserExpectEOF(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 10))
	rc := NewExactReadCloser(&readerNilCloser{buf}, 1)
	_, err := rc.Read(make([]byte, 10))
	require.ErrorIsf(t, err, ErrExpectEOF, "expected %v, got %v", ErrExpectEOF, err)
}

// TestExactReadCloserShort expects an eof when reading too little
func TestExactReadCloserShort(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 5))
	rc := NewExactReadCloser(&readerNilCloser{buf}, 10)
	_, err := rc.Read(make([]byte, 10))
	require.NoErrorf(t, err, "Read expected nil err, got %v", err)
	require.ErrorIs(t, rc.Close(), ErrShortRead)
}
