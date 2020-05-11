// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zap

import (
	"encoding/hex"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestOpenNoPaths(t *testing.T) {
	ws, cleanup, err := Open()
	defer cleanup()

	assert.NoError(t, err, "Expected opening no paths to succeed.")
	assert.Equal(
		t,
		zapcore.AddSync(ioutil.Discard),
		ws,
		"Expected opening no paths to return a no-op WriteSyncer.",
	)
}

func TestOpen(t *testing.T) {
	tempName := tempFileName("", "zap-open-test")
	assert.False(t, fileExists(tempName))
	require.True(t, strings.HasPrefix(tempName, "/"), "Expected absolute temp file path.")

	tests := []struct {
		paths []string
		errs  []string
	}{
		{[]string{"stdout"}, nil},
		{[]string{"stderr"}, nil},
		{[]string{tempName}, nil},
		{[]string{"file://" + tempName}, nil},
		{[]string{"file://localhost" + tempName}, nil},
		{[]string{"/foo/bar/baz"}, []string{"open /foo/bar/baz: no such file or directory"}},
		{[]string{"file://localhost/foo/bar/baz"}, []string{"open /foo/bar/baz: no such file or directory"}},
		{
			paths: []string{"stdout", "/foo/bar/baz", tempName, "file:///baz/quux"},
			errs: []string{
				"open /foo/bar/baz: no such file or directory",
				"open /baz/quux: no such file or directory",
			},
		},
		{[]string{"file:///stderr"}, []string{"open /stderr: permission denied"}},
		{[]string{"file:///stdout"}, []string{"open /stdout: permission denied"}},
		{[]string{"file://host01.test.com" + tempName}, []string{"empty or use localhost"}},
		{[]string{"file://rms@localhost" + tempName}, []string{"user and password not allowed"}},
		{[]string{"file://localhost" + tempName + "#foo"}, []string{"fragments not allowed"}},
		{[]string{"file://localhost" + tempName + "?foo=bar"}, []string{"query parameters not allowed"}},
		{[]string{"file://localhost:8080" + tempName}, []string{"ports not allowed"}},
	}

	for _, tt := range tests {
		_, cleanup, err := Open(tt.paths...)
		if err == nil {
			defer cleanup()
		}

		if len(tt.errs) == 0 {
			assert.NoError(t, err, "Unexpected error opening paths %v.", tt.paths)
		} else {
			msg := err.Error()
			for _, expect := range tt.errs {
				assert.Contains(t, msg, expect, "Unexpected error opening paths %v.", tt.paths)
			}
		}
	}

	assert.True(t, fileExists(tempName))
	os.Remove(tempName)
}

func TestOpenRelativePath(t *testing.T) {
	const name = "test-relative-path.txt"

	require.False(t, fileExists(name), "Test file already exists.")
	s, cleanup, err := Open(name)
	require.NoError(t, err, "Open failed.")
	defer func() {
		err := os.Remove(name)
		if !t.Failed() {
			// If the test has already failed, we probably didn't create this file.
			require.NoError(t, err, "Deleting test file failed.")
		}
	}()
	defer cleanup()

	_, err = s.Write([]byte("test"))
	assert.NoError(t, err, "Write failed.")
	assert.True(t, fileExists(name), "Didn't create file for relative path.")
}

func TestOpenFails(t *testing.T) {
	tests := []struct {
		paths []string
	}{
		{paths: []string{"./non-existent-dir/file"}},           // directory doesn't exist
		{paths: []string{"stdout", "./non-existent-dir/file"}}, // directory doesn't exist
		{paths: []string{"://foo.log"}},                        // invalid URL, scheme can't begin with colon
		{paths: []string{"mem://somewhere"}},                   // scheme not registered
	}

	for _, tt := range tests {
		_, cleanup, err := Open(tt.paths...)
		require.Nil(t, cleanup, "Cleanup function should never be nil")
		assert.Error(t, err, "Open with invalid URL should fail.")
	}
}

type testWriter struct {
	expected string
	t        testing.TB
}

func (w *testWriter) Write(actual []byte) (int, error) {
	assert.Equal(w.t, []byte(w.expected), actual, "Unexpected write error.")
	return len(actual), nil
}

func (w *testWriter) Sync() error {
	return nil
}

func TestOpenWithErroringSinkFactory(t *testing.T) {
	defer resetSinkRegistry()

	msg := "expected factory error"
	factory := func(_ *url.URL) (Sink, error) {
		return nil, errors.New(msg)
	}

	assert.NoError(t, RegisterSink("test", factory), "Failed to register sink factory.")
	_, _, err := Open("test://some/path")
	assert.Contains(t, err.Error(), msg, "Unexpected error.")
}

func TestCombineWriteSyncers(t *testing.T) {
	tw := &testWriter{"test", t}
	w := CombineWriteSyncers(tw)
	w.Write([]byte("test"))
}

func tempFileName(prefix, suffix string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return filepath.Join(os.TempDir(), prefix+hex.EncodeToString(randBytes)+suffix)
}

func fileExists(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}
