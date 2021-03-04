// Copyright 2017 The etcd Authors
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

package ordering

import (
	"errors"
	"sync/atomic"

	"go.etcd.io/etcd/client/v3"
)

type OrderViolationFunc func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error

var ErrNoGreaterRev = errors.New("etcdclient: no cluster members have a revision higher than the previously received revision")

func NewOrderViolationSwitchEndpointClosure(c *clientv3.Client) OrderViolationFunc {
	violationCount := int32(0)
	return func(_ clientv3.Op, _ clientv3.OpResponse, _ int64) error {
		// Each request is assigned by round-robin load-balancer's picker to a different
		// endpoints. If we cycled them 5 times (even with some level of concurrency),
		// with high probability no endpoint points on a member with fresh data.
		// TODO: Ideally we should track members (resp.opp.Header) that returned
		// stale result and explicitly temporarily disable them in 'picker'.
		if atomic.LoadInt32(&violationCount) > int32(5*len(c.Endpoints())) {
			return ErrNoGreaterRev
		}
		atomic.AddInt32(&violationCount, 1)
		return nil
	}
}
