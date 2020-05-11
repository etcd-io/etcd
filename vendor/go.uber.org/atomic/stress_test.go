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
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	_parallelism = 4
	_iterations  = 1000
)

var _stressTests = map[string]func() func(){
	"i32/std":  stressStdInt32,
	"i32":      stressInt32,
	"i64/std":  stressStdInt32,
	"i64":      stressInt64,
	"u32/std":  stressStdUint32,
	"u32":      stressUint32,
	"u64/std":  stressStdUint64,
	"u64":      stressUint64,
	"f64":      stressFloat64,
	"bool":     stressBool,
	"string":   stressString,
	"duration": stressDuration,
	"error":    stressError,
}

func TestStress(t *testing.T) {
	for name, ff := range _stressTests {
		t.Run(name, func(t *testing.T) {
			defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(_parallelism))

			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(_parallelism)
			f := ff()
			for i := 0; i < _parallelism; i++ {
				go func() {
					defer wg.Done()
					<-start
					for j := 0; j < _iterations; j++ {
						f()
					}
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

func BenchmarkStress(b *testing.B) {
	for name, ff := range _stressTests {
		b.Run(name, func(b *testing.B) {
			f := ff()

			b.Run("serial", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					f()
				}
			})

			b.Run("parallel", func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						f()
					}
				})
			})
		})
	}
}

func stressStdInt32() func() {
	var atom int32
	return func() {
		atomic.LoadInt32(&atom)
		atomic.AddInt32(&atom, 1)
		atomic.AddInt32(&atom, -2)
		atomic.AddInt32(&atom, 1)
		atomic.AddInt32(&atom, -1)
		atomic.CompareAndSwapInt32(&atom, 1, 0)
		atomic.SwapInt32(&atom, 5)
		atomic.StoreInt32(&atom, 1)
	}
}

func stressInt32() func() {
	var atom Int32
	return func() {
		atom.Load()
		atom.Add(1)
		atom.Sub(2)
		atom.Inc()
		atom.Dec()
		atom.CAS(1, 0)
		atom.Swap(5)
		atom.Store(1)
	}
}

func stressStdInt64() func() {
	var atom int64
	return func() {
		atomic.LoadInt64(&atom)
		atomic.AddInt64(&atom, 1)
		atomic.AddInt64(&atom, -2)
		atomic.AddInt64(&atom, 1)
		atomic.AddInt64(&atom, -1)
		atomic.CompareAndSwapInt64(&atom, 1, 0)
		atomic.SwapInt64(&atom, 5)
		atomic.StoreInt64(&atom, 1)
	}
}

func stressInt64() func() {
	var atom Int64
	return func() {
		atom.Load()
		atom.Add(1)
		atom.Sub(2)
		atom.Inc()
		atom.Dec()
		atom.CAS(1, 0)
		atom.Swap(5)
		atom.Store(1)
	}
}

func stressStdUint32() func() {
	var atom uint32
	return func() {
		atomic.LoadUint32(&atom)
		atomic.AddUint32(&atom, 1)
		// Adding `MaxUint32` is the same as subtracting 1
		atomic.AddUint32(&atom, math.MaxUint32-1)
		atomic.AddUint32(&atom, 1)
		atomic.AddUint32(&atom, math.MaxUint32)
		atomic.CompareAndSwapUint32(&atom, 1, 0)
		atomic.SwapUint32(&atom, 5)
		atomic.StoreUint32(&atom, 1)
	}
}

func stressUint32() func() {
	var atom Uint32
	return func() {
		atom.Load()
		atom.Add(1)
		atom.Sub(2)
		atom.Inc()
		atom.Dec()
		atom.CAS(1, 0)
		atom.Swap(5)
		atom.Store(1)
	}
}

func stressStdUint64() func() {
	var atom uint64
	return func() {
		atomic.LoadUint64(&atom)
		atomic.AddUint64(&atom, 1)
		// Adding `MaxUint64` is the same as subtracting 1
		atomic.AddUint64(&atom, math.MaxUint64-1)
		atomic.AddUint64(&atom, 1)
		atomic.AddUint64(&atom, math.MaxUint64)
		atomic.CompareAndSwapUint64(&atom, 1, 0)
		atomic.SwapUint64(&atom, 5)
		atomic.StoreUint64(&atom, 1)
	}
}

func stressUint64() func() {
	var atom Uint64
	return func() {
		atom.Load()
		atom.Add(1)
		atom.Sub(2)
		atom.Inc()
		atom.Dec()
		atom.CAS(1, 0)
		atom.Swap(5)
		atom.Store(1)
	}
}

func stressFloat64() func() {
	var atom Float64
	return func() {
		atom.Load()
		atom.CAS(1.0, 0.1)
		atom.Add(1.1)
		atom.Sub(0.2)
		atom.Store(1.0)
	}
}

func stressBool() func() {
	var atom Bool
	return func() {
		atom.Load()
		atom.Store(false)
		atom.Swap(true)
		atom.CAS(true, false)
		atom.CAS(true, false)
		atom.Load()
		atom.Toggle()
		atom.Toggle()
	}
}

func stressString() func() {
	var atom String
	return func() {
		atom.Load()
		atom.Store("abc")
		atom.Load()
		atom.Store("def")
		atom.Load()
		atom.Store("")
	}
}

func stressDuration() func() {
	var atom = NewDuration(0)
	return func() {
		atom.Load()
		atom.Add(1)
		atom.Sub(2)
		atom.CAS(1, 0)
		atom.Swap(5)
		atom.Store(1)
	}
}

func stressError() func() {
	var atom = NewError(nil)
	var err1 = errors.New("err1")
	var err2 = errors.New("err2")
	return func() {
		atom.Load()
		atom.Store(err1)
		atom.Load()
		atom.Store(err2)
		atom.Load()
		atom.Store(nil)
	}
}
