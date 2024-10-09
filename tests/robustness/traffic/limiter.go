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

func NewConcurrencyLimiter(size int) ConcurrencyLimiter {
	return &concurrencyLimiter{
		ch: make(chan struct{}, size),
	}
}

type ConcurrencyLimiter interface {
	Take() bool
	Return()
}

type concurrencyLimiter struct {
	ch chan struct{}
}

func (c *concurrencyLimiter) Take() bool {
	select {
	case c.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (c *concurrencyLimiter) Return() {
	select {
	case _ = <-c.ch:
	default:
		panic("Call to Return() without a successful Take")
	}
}
