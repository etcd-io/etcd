// Copyright 2023 The etcd Authors
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

package traffic

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

func TestLimiter(t *testing.T) {
	limiter := NewConcurrencyLimiter(3)
	counter := &atomic.Int64{}
	g := errgroup.Group{}
	for i := 0; i < 10; i++ {
		g.Go(func() error {
			if limiter.Take() {
				counter.Add(1)
			}
			return nil
		})
	}
	g.Wait()
	assert.Equal(t, 3, int(counter.Load()))
	assert.False(t, limiter.Take())

	limiter.Return()
	counter.Store(0)
	for i := 0; i < 10; i++ {
		g.Go(func() error {
			if limiter.Take() {
				counter.Add(1)
			}
			return nil
		})
	}
	g.Wait()
	assert.Equal(t, 1, int(counter.Load()))
	assert.False(t, limiter.Take())

	limiter.Return()
	limiter.Return()
	limiter.Return()
	assert.Panics(t, func() {
		limiter.Return()
	})
}
