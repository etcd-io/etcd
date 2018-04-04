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
)

type failureNoOp failureByFunc

func (f *failureNoOp) Inject(clus *Cluster) error     { return nil }
func (f *failureNoOp) Recover(clus *Cluster) error    { return nil }
func (f *failureNoOp) FailureCase() rpcpb.FailureCase { return f.failureCase }

func newFailureNoOp() Failure {
	f := &failureNoOp{
		failureCase: rpcpb.FailureCase_NO_FAIL_WITH_STRESS,
	}
	return &failureDelay{
		Failure:       f,
		delayDuration: 5 * time.Second,
	}
}
