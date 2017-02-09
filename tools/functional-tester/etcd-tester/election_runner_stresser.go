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

type electionRunnerStresser struct {
	cf     *runner.EtcdRunnerConfig
	runWg  sync.WaitGroup
	errch  chan error
	ctx    context.Context
	cancel func()
}

func (ers *electionRunnerStresser) Stress() error {
	ers.ctx, ers.cancel = context.WithTimeout(context.Background(), runnerTimeout)
	ers.errch = make(chan error, 2)
	ers.runWg.Add(1)
	go func() {
		defer ers.cancel()
		defer ers.runWg.Done()
		ers.errch <- runner.RunElection(ers.ctx, ers.cf)
	}()
	return nil
}

func (ers *electionRunnerStresser) Cancel() {
	plog.Info("electionRunnerStresser stresser is canceling...")
	select {
	case <-ers.ctx.Done():
		if ers.ctx.Err() != nil {
			ers.errch <- ers.ctx.Err()
		}
	}
	ers.runWg.Wait()
	plog.Info("electionRunnerStresser stresser is canceled")
}

func (ers *electionRunnerStresser) ModifiedKeys() int64 {
	// only 1 key is created for election
	return 1
}

func (ers *electionRunnerStresser) Checker() Checker {
	return &electionRunnerChecker{check: func() <-chan error {
		return ers.errch
	}}
}
