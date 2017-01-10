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

package main

import (
	"sync"

	"golang.org/x/time/rate"

	"context"

	"time"

	"github.com/coreos/etcd/tools/functional-tester/etcd-runner/runner"
)

var (
	clientTimeout = 5 // seconds
	runnerTimeout = 120 * time.Second
)

type watchRunnerStresser struct {
	cf      *runner.WatchRunnerConfig
	limiter *rate.Limiter
	runWg   sync.WaitGroup
	errch   chan error
	ctx     context.Context
	cancel  func()
}

func (wrs *watchRunnerStresser) Stress() error {
	wrs.ctx, wrs.cancel = context.WithTimeout(context.Background(), runnerTimeout)
	wrs.errch = make(chan error, 2)
	wrs.runWg.Add(1)
	go func() {
		defer wrs.cancel()
		defer wrs.runWg.Done()
		wrs.errch <- runner.RunWatchOnce(wrs.ctx, wrs.cf, wrs.limiter, 1)
	}()

	return nil
}

func (wrs *watchRunnerStresser) Cancel() {
	plog.Info("watchRunnerStresser stresser is canceling...")
	select {
	case <-wrs.ctx.Done():
		if wrs.ctx.Err() != nil {
			wrs.errch <- wrs.ctx.Err()
		}
	}
	wrs.runWg.Wait()
	plog.Info("watchRunnerStresser stresser is canceled")
}

func (wrs *watchRunnerStresser) ModifiedKeys() int64 {
	// return a upper bound of the keys
	return int64(wrs.cf.TotalKeys)
}

func (wrs *watchRunnerStresser) Checker() Checker {
	return &watchRunnerChecker{check: func() <-chan error {
		return wrs.errch
	}}
}
