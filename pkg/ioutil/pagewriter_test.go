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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageWriterRandom(t *testing.T) {
	// smaller buffer for stress testing
	defaultBufferBytes = 8 * 1024
	pageBytes := 128
	buf := make([]byte, 4*defaultBufferBytes)
	cw := &checkPageWriter{pageBytes: pageBytes, t: t}
	w := NewPageWriter(cw, pageBytes, 0)
	n := 0
	for i := 0; i < 4096; i++ {
		c, err := w.Write(buf[:rand.Intn(len(buf))])
		if err != nil {
			t.Fatal(err)
		}
		n += c
	}
	if cw.writeBytes > n {
		t.Fatalf("wrote %d bytes to io.Writer, but only wrote %d bytes", cw.writeBytes, n)
	}
	if maxPendingBytes := pageBytes + defaultBufferBytes; n-cw.writeBytes > maxPendingBytes {
		t.Fatalf("got %d bytes pending, expected less than %d bytes", n-cw.writeBytes, maxPendingBytes)
	}
	t.Logf("total writes: %d", cw.writes)
	t.Logf("total write bytes: %d (of %d)", cw.writeBytes, n)
}

// TestPageWriterPartialSlack tests the case where a write overflows the buffer
// but there is not enough data to complete the slack write.
func TestPageWriterPartialSlack(t *testing.T) {
	defaultBufferBytes = 1024
	pageBytes := 128
	buf := make([]byte, defaultBufferBytes)
	cw := &checkPageWriter{pageBytes: 64, t: t}
	w := NewPageWriter(cw, pageBytes, 0)
	// put writer in non-zero page offset
	if _, err := w.Write(buf[:64]); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if cw.writes != 1 {
		t.Fatalf("got %d writes, expected 1", cw.writes)
	}
	// nearly fill buffer
	if _, err := w.Write(buf[:1022]); err != nil {
		t.Fatal(err)
	}
	// overflow buffer, but without enough to write as aligned
	if _, err := w.Write(buf[:8]); err != nil {
		t.Fatal(err)
	}
	if cw.writes != 1 {
		t.Fatalf("got %d writes, expected 1", cw.writes)
	}
	// finish writing slack space
	if _, err := w.Write(buf[:128]); err != nil {
		t.Fatal(err)
	}
	if cw.writes != 2 {
		t.Fatalf("got %d writes, expected 2", cw.writes)
	}
}

// TestPageWriterOffset tests if page writer correctly repositions when offset is given.
func TestPageWriterOffset(t *testing.T) {
	defaultBufferBytes = 1024
	pageBytes := 128
	buf := make([]byte, defaultBufferBytes)
	cw := &checkPageWriter{pageBytes: 64, t: t}
	w := NewPageWriter(cw, pageBytes, 0)
	if _, err := w.Write(buf[:64]); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if w.pageOffset != 64 {
		t.Fatalf("w.pageOffset expected 64, got %d", w.pageOffset)
	}

	w = NewPageWriter(cw, w.pageOffset, pageBytes)
	if _, err := w.Write(buf[:64]); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if w.pageOffset != 0 {
		t.Fatalf("w.pageOffset expected 0, got %d", w.pageOffset)
	}
}

func TestPageWriterPageBytes(t *testing.T) {
	cases := []struct {
		name        string
		pageBytes   int
		expectPanic bool
	}{
		{
			name:        "normal page bytes",
			pageBytes:   4096,
			expectPanic: false,
		},
		{
			name:        "negative page bytes",
			pageBytes:   -1,
			expectPanic: true,
		},
		{
			name:        "zero page bytes",
			pageBytes:   0,
			expectPanic: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defaultBufferBytes = 1024
			cw := &checkPageWriter{pageBytes: tc.pageBytes, t: t}
			if tc.expectPanic {
				assert.Panicsf(t, func() {
					NewPageWriter(cw, tc.pageBytes, 0)
				}, "expected panic when pageBytes is %d", tc.pageBytes)
			} else {
				pw := NewPageWriter(cw, tc.pageBytes, 0)
				assert.NotEqual(t, pw, nil)
			}
		})
	}
}

// checkPageWriter implements an io.Writer that fails a test on unaligned writes.
type checkPageWriter struct {
	pageBytes  int
	writes     int
	writeBytes int
	t          *testing.T
}

func (cw *checkPageWriter) Write(p []byte) (int, error) {
	if len(p)%cw.pageBytes != 0 {
		cw.t.Fatalf("got write len(p) = %d, expected len(p) == k*cw.pageBytes", len(p))
	}
	cw.writes++
	cw.writeBytes += len(p)
	return len(p), nil
}
