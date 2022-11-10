// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

var (
	PutGetTraffic Traffic = putGetTraffic{}
)

type Traffic interface {
	Run(ctx context.Context, c *recordingClient, limiter *rate.Limiter)
}

type putGetTraffic struct{}

func (t putGetTraffic) Run(ctx context.Context, c *recordingClient, limiter *rate.Limiter) {
	maxOperationsPerClient := 10
	id := maxOperationsPerClient * c.id
	key := "key"

	for i := 0; i < maxOperationsPerClient; {
		select {
		case <-ctx.Done():
			return
		default:
		}
		getCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		err := c.Get(getCtx, key)
		cancel()
		if err != nil {
			continue
		}
		limiter.Wait(ctx)
		putData := fmt.Sprintf("%d", id+i)
		putCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		err = c.Put(putCtx, key, putData)
		cancel()
		if err != nil {
			continue
		}
		limiter.Wait(ctx)
		i++
	}
	return
}
