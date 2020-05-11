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

package atomic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorByValue(t *testing.T) {
	err := &Error{}
	require.Nil(t, err.Load(), "Initial value shall be nil")
}

func TestNewErrorWithNilArgument(t *testing.T) {
	err := NewError(nil)
	require.Nil(t, err.Load(), "Initial value shall be nil")
}

func TestErrorCanStoreNil(t *testing.T) {
	err := NewError(errors.New("hello"))
	err.Store(nil)
	require.Nil(t, err.Load(), "Stored value shall be nil")
}

func TestNewErrorWithError(t *testing.T) {
	err1 := errors.New("hello1")
	err2 := errors.New("hello2")

	atom := NewError(err1)
	require.Equal(t, err1, atom.Load(), "Expected Load to return initialized value")

	atom.Store(err2)
	require.Equal(t, err2, atom.Load(), "Expected Load to return overridden value")
}
