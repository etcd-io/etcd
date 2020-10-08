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

	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
)

type caseNoFailWithStress caseByFunc

func (c *caseNoFailWithStress) Inject(clus *Cluster) error {
	return nil
}

func (c *caseNoFailWithStress) Recover(clus *Cluster) error {
	return nil
}

func (c *caseNoFailWithStress) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseNoFailWithStress) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

func new_Case_NO_FAIL_WITH_STRESS(clus *Cluster) Case {
	c := &caseNoFailWithStress{
		rpcpbCase: rpcpb.Case_NO_FAIL_WITH_STRESS,
	}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

type caseNoFailWithNoStressForLiveness caseByFunc

func (c *caseNoFailWithNoStressForLiveness) Inject(clus *Cluster) error {
	clus.lg.Info(
		"extra delay for liveness mode with no stresser",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.String("desc", c.Desc()),
	)
	time.Sleep(clus.GetCaseDelayDuration())

	clus.lg.Info(
		"wait health in liveness mode",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.String("desc", c.Desc()),
	)
	return clus.WaitHealth()
}

func (c *caseNoFailWithNoStressForLiveness) Recover(clus *Cluster) error {
	return nil
}

func (c *caseNoFailWithNoStressForLiveness) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *caseNoFailWithNoStressForLiveness) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

func new_Case_NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS(clus *Cluster) Case {
	c := &caseNoFailWithNoStressForLiveness{
		rpcpbCase: rpcpb.Case_NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS,
	}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}
