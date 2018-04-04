// Copyright 2018 The etcd Authors
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

package tester

import (
	"time"

	"github.com/coreos/etcd/tools/functional-tester/rpcpb"

	"go.uber.org/zap"
)

type failureNoFailWithStress failureByFunc

func (f *failureNoFailWithStress) Inject(clus *Cluster) error {
	return nil
}

func (f *failureNoFailWithStress) Recover(clus *Cluster) error {
	return nil
}

func (f *failureNoFailWithStress) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

func newFailureNoFailWithStress(clus *Cluster) Failure {
	f := &failureNoFailWithStress{
		failureCase: rpcpb.FailureCase_NO_FAIL_WITH_STRESS,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}

type failureNoFailWithNoStressForLiveness failureByFunc

func (f *failureNoFailWithNoStressForLiveness) Inject(clus *Cluster) error {
	clus.lg.Info(
		"extra delay for liveness mode with no stresser",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.String("desc", f.Desc()),
	)
	time.Sleep(clus.GetFailureDelayDuration())

	clus.lg.Info(
		"wait health in liveness mode",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.String("desc", f.Desc()),
	)
	return clus.WaitHealth()
}

func (f *failureNoFailWithNoStressForLiveness) Recover(clus *Cluster) error {
	return nil
}

func (f *failureNoFailWithNoStressForLiveness) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

func newFailureNoFailWithNoStressForLiveness(clus *Cluster) Failure {
	f := &failureNoFailWithNoStressForLiveness{
		failureCase: rpcpb.FailureCase_NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}
