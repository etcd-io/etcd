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

package schedule

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFIFOSchedule(t *testing.T) {
	s := NewFIFOScheduler(zaptest.NewLogger(t))
	defer s.Stop()

	next := 0
	jobCreator := func(i int) Job {
		return NewJob(fmt.Sprintf("i_%d_increse", i), func(ctx context.Context) {
			defer func() {
				if err := recover(); err != nil {
					fmt.Println("err: ", err)
				}
			}()
			require.Equalf(t, next, i, "job#%d: got %d, want %d", i, next, i)
			next = i + 1
			if next%3 == 0 {
				panic("fifo panic")
			}
		})
	}

	var jobs []Job
	for i := 0; i < 100; i++ {
		jobs = append(jobs, jobCreator(i))
	}

	for _, j := range jobs {
		s.Schedule(j)
	}

	s.WaitFinish(100)
	assert.Equalf(t, 100, s.Finished(), "finished = %d, want %d", s.Finished(), 100)
}
