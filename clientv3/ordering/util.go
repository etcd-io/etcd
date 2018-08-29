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
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
)

type OrderViolationFunc func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error

var ErrNoGreaterRev = errors.New("etcdclient: no cluster members have a revision higher than the previously received revision")

func NewOrderViolationSwitchEndpointClosure(c clientv3.Client) OrderViolationFunc {
	var mu sync.Mutex
	violationCount := 0
	return func(op clientv3.Op, resp clientv3.OpResponse, prevRev int64) error {
		if violationCount > len(c.Endpoints()) {
			return ErrNoGreaterRev
		}
		mu.Lock()
		defer mu.Unlock()
		eps := c.Endpoints()
		// force client to connect to given endpoint by limiting to a single endpoint
		c.SetEndpoints(eps[violationCount%len(eps)])
		// give enough time for operation
		time.Sleep(1 * time.Second)
		// set available endpoints back to all endpoints in to ensure
		// the client has access to all the endpoints.
		c.SetEndpoints(eps...)
		// give enough time for operation
		time.Sleep(1 * time.Second)
		violationCount++
		return nil
	}
}
