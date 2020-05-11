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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInt32(t *testing.T) {
	atom := NewInt32(42)

	require.Equal(t, int32(42), atom.Load(), "Load didn't work.")
	require.Equal(t, int32(46), atom.Add(4), "Add didn't work.")
	require.Equal(t, int32(44), atom.Sub(2), "Sub didn't work.")
	require.Equal(t, int32(45), atom.Inc(), "Inc didn't work.")
	require.Equal(t, int32(44), atom.Dec(), "Dec didn't work.")

	require.True(t, atom.CAS(44, 0), "CAS didn't report a swap.")
	require.Equal(t, int32(0), atom.Load(), "CAS didn't set the correct value.")

	require.Equal(t, int32(0), atom.Swap(1), "Swap didn't return the old value.")
	require.Equal(t, int32(1), atom.Load(), "Swap didn't set the correct value.")

	atom.Store(42)
	require.Equal(t, int32(42), atom.Load(), "Store didn't set the correct value.")
}

func TestInt64(t *testing.T) {
	atom := NewInt64(42)

	require.Equal(t, int64(42), atom.Load(), "Load didn't work.")
	require.Equal(t, int64(46), atom.Add(4), "Add didn't work.")
	require.Equal(t, int64(44), atom.Sub(2), "Sub didn't work.")
	require.Equal(t, int64(45), atom.Inc(), "Inc didn't work.")
	require.Equal(t, int64(44), atom.Dec(), "Dec didn't work.")

	require.True(t, atom.CAS(44, 0), "CAS didn't report a swap.")
	require.Equal(t, int64(0), atom.Load(), "CAS didn't set the correct value.")

	require.Equal(t, int64(0), atom.Swap(1), "Swap didn't return the old value.")
	require.Equal(t, int64(1), atom.Load(), "Swap didn't set the correct value.")

	atom.Store(42)
	require.Equal(t, int64(42), atom.Load(), "Store didn't set the correct value.")
}

func TestUint32(t *testing.T) {
	atom := NewUint32(42)

	require.Equal(t, uint32(42), atom.Load(), "Load didn't work.")
	require.Equal(t, uint32(46), atom.Add(4), "Add didn't work.")
	require.Equal(t, uint32(44), atom.Sub(2), "Sub didn't work.")
	require.Equal(t, uint32(45), atom.Inc(), "Inc didn't work.")
	require.Equal(t, uint32(44), atom.Dec(), "Dec didn't work.")

	require.True(t, atom.CAS(44, 0), "CAS didn't report a swap.")
	require.Equal(t, uint32(0), atom.Load(), "CAS didn't set the correct value.")

	require.Equal(t, uint32(0), atom.Swap(1), "Swap didn't return the old value.")
	require.Equal(t, uint32(1), atom.Load(), "Swap didn't set the correct value.")

	atom.Store(42)
	require.Equal(t, uint32(42), atom.Load(), "Store didn't set the correct value.")
}

func TestUint64(t *testing.T) {
	atom := NewUint64(42)

	require.Equal(t, uint64(42), atom.Load(), "Load didn't work.")
	require.Equal(t, uint64(46), atom.Add(4), "Add didn't work.")
	require.Equal(t, uint64(44), atom.Sub(2), "Sub didn't work.")
	require.Equal(t, uint64(45), atom.Inc(), "Inc didn't work.")
	require.Equal(t, uint64(44), atom.Dec(), "Dec didn't work.")

	require.True(t, atom.CAS(44, 0), "CAS didn't report a swap.")
	require.Equal(t, uint64(0), atom.Load(), "CAS didn't set the correct value.")

	require.Equal(t, uint64(0), atom.Swap(1), "Swap didn't return the old value.")
	require.Equal(t, uint64(1), atom.Load(), "Swap didn't set the correct value.")

	atom.Store(42)
	require.Equal(t, uint64(42), atom.Load(), "Store didn't set the correct value.")
}

func TestBool(t *testing.T) {
	atom := NewBool(false)
	require.False(t, atom.Toggle(), "Expected Toggle to return previous value.")
	require.True(t, atom.Toggle(), "Expected Toggle to return previous value.")
	require.False(t, atom.Toggle(), "Expected Toggle to return previous value.")
	require.True(t, atom.Load(), "Unexpected state after swap.")

	require.True(t, atom.CAS(true, true), "CAS should swap when old matches")
	require.True(t, atom.Load(), "CAS should have no effect")
	require.True(t, atom.CAS(true, false), "CAS should swap when old matches")
	require.False(t, atom.Load(), "CAS should have modified the value")
	require.False(t, atom.CAS(true, false), "CAS should fail on old mismatch")
	require.False(t, atom.Load(), "CAS should not have modified the value")

	atom.Store(false)
	require.False(t, atom.Load(), "Unexpected state after store.")

	prev := atom.Swap(false)
	require.False(t, prev, "Expected Swap to return previous value.")

	prev = atom.Swap(true)
	require.False(t, prev, "Expected Swap to return previous value.")
}

func TestFloat64(t *testing.T) {
	atom := NewFloat64(4.2)

	require.Equal(t, float64(4.2), atom.Load(), "Load didn't work.")

	require.True(t, atom.CAS(4.2, 0.5), "CAS didn't report a swap.")
	require.Equal(t, float64(0.5), atom.Load(), "CAS didn't set the correct value.")
	require.False(t, atom.CAS(0.0, 1.5), "CAS reported a swap.")

	atom.Store(42.0)
	require.Equal(t, float64(42.0), atom.Load(), "Store didn't set the correct value.")
	require.Equal(t, float64(42.5), atom.Add(0.5), "Add didn't work.")
	require.Equal(t, float64(42.0), atom.Sub(0.5), "Sub didn't work.")
}

func TestDuration(t *testing.T) {
	atom := NewDuration(5 * time.Minute)

	require.Equal(t, 5*time.Minute, atom.Load(), "Load didn't work.")
	require.Equal(t, 6*time.Minute, atom.Add(time.Minute), "Add didn't work.")
	require.Equal(t, 4*time.Minute, atom.Sub(2*time.Minute), "Sub didn't work.")

	require.True(t, atom.CAS(4*time.Minute, time.Minute), "CAS didn't report a swap.")
	require.Equal(t, time.Minute, atom.Load(), "CAS didn't set the correct value.")

	require.Equal(t, time.Minute, atom.Swap(2*time.Minute), "Swap didn't return the old value.")
	require.Equal(t, 2*time.Minute, atom.Load(), "Swap didn't set the correct value.")

	atom.Store(10 * time.Minute)
	require.Equal(t, 10*time.Minute, atom.Load(), "Store didn't set the correct value.")
}

func TestValue(t *testing.T) {
	var v Value
	assert.Nil(t, v.Load(), "initial Value is not nil")

	v.Store(42)
	assert.Equal(t, 42, v.Load())

	v.Store(84)
	assert.Equal(t, 84, v.Load())

	assert.Panics(t, func() { v.Store("foo") })
}
