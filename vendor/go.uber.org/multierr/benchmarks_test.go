// Copyright (c) 2017 Uber Technologies, Inc.
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

package multierr

import (
	"errors"
	"fmt"
	"testing"
)

func BenchmarkAppend(b *testing.B) {
	errorTypes := []struct {
		name string
		err  error
	}{
		{
			name: "nil",
			err:  nil,
		},
		{
			name: "single error",
			err:  errors.New("test"),
		},
		{
			name: "multiple errors",
			err:  appendN(nil, errors.New("err"), 10),
		},
	}

	for _, initial := range errorTypes {
		for _, v := range errorTypes {
			msg := fmt.Sprintf("append %v to %v", v.name, initial.name)
			b.Run(msg, func(b *testing.B) {
				for _, appends := range []int{1, 2, 10} {
					b.Run(fmt.Sprint(appends), func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							appendN(initial.err, v.err, appends)
						}
					})
				}
			})
		}
	}
}
