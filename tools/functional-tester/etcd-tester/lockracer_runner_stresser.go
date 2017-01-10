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
	"context"
	"sync"

	"github.com/coreos/etcd/tools/functional-tester/etcd-runner/runner"
)

type lockracerRunnerStresser struct {
	cf     *runner.EtcdRunnerConfig
	runWg  sync.WaitGroup
	errch  chan error
	ctx    context.Context
	cancel func()
}

func (lrs *lockracerRunnerStresser) Stress() error {
	lrs.ctx, lrs.cancel = context.WithTimeout(context.Background(), runnerTimeout)
	lrs.errch = make(chan error, 2)
	lrs.runWg.Add(1)
	go func() {
		defer lrs.cancel()
		defer lrs.runWg.Done()
		lrs.errch <- runner.RunRacer(lrs.ctx, lrs.cf)
	}()
	return nil
}

func (lrs *lockracerRunnerStresser) Cancel() {
	plog.Info("lockracerRunnerStresser stresser is canceling...")
	select {
	case <-lrs.ctx.Done():
		if lrs.ctx.Err() != nil {
			lrs.errch <- lrs.ctx.Err()
		}
	}
	lrs.runWg.Wait()
	plog.Info("lockracerRunnerStresser stresser is canceled")
}

func (lrs *lockracerRunnerStresser) ModifiedKeys() int64 {
	// only 1 key is created to run lock racer
	return 1
}

func (lrs *lockracerRunnerStresser) Checker() Checker {
	return &lockRacerRunnerChecker{check: func() <-chan error {
		return lrs.errch
	}}
}
