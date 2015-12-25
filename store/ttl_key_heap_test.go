// Copyright 2015 CoreOS, Inc.
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

package store

import (
	"fmt"
	"testing"
	"time"
)

func TestTtlKeyHeap(t *testing.T) {
	s := newStore()
	heapN := 100
	exp := time.Minute

	nodesExpSoonestLast := make([]*node, heapN)
	for i := 0; i < heapN; i++ {
		key := fmt.Sprintf("%d_Foo", i)
		val := fmt.Sprintf("%d_Val", i)
		expireTime := time.Now().Add(time.Duration(i) * exp)
		nodesExpSoonestLast[heapN-i-1] = newKV(s, key, val, uint64(i), nil, expireTime)
	}

	th := newTtlKeyHeap()
	for i := range nodesExpSoonestLast {
		th.push(nodesExpSoonestLast[i])
	}

	// first element must be the one expiring soonest
	if th.top().Path != nodesExpSoonestLast[heapN-1].Path {
		t.Errorf("th.top().Path = %s, want = %s", th.top().Path, nodesExpSoonestLast[heapN-1].Path)
	}

	if nd := th.pop(); nd.Path != nodesExpSoonestLast[heapN-1].Path {
		t.Errorf("th.top().Path = %s, want = %s", nd.Path, nodesExpSoonestLast[heapN-1].Path)
	} else {
		// put it back
		th.push(nd)
		if th.top().Path != nodesExpSoonestLast[heapN-1].Path {
			t.Errorf("th.top().Path = %s, want = %s", th.top().Path, nodesExpSoonestLast[heapN-1].Path)
		}
	}

	// update the first element to the last element
	top := th.top()
	top.ExpireTime = time.Now().Add(time.Duration(heapN*10) * exp)
	th.update(top)

	// first element must be the one expiring soonest.
	// Here, it must be second element in test array
	// since we updated the first(top) node.
	if th.top().Path != nodesExpSoonestLast[heapN-2].Path {
		t.Errorf("th.top().Path = %s, want = %s", th.top().Path, nodesExpSoonestLast[heapN-2].Path)
	}

	// update 5th element to second last
	nodesExpSoonestLast[4].ExpireTime = time.Now().Add(time.Duration(heapN*9) * exp)
	th.update(nodesExpSoonestLast[4])

	// update 7th element to third last
	nodesExpSoonestLast[6].ExpireTime = time.Now().Add(time.Duration(heapN*8) * exp)
	th.update(nodesExpSoonestLast[6])

	// compare all the expiratonTime
	var expT time.Time
	for i := 0; th.Len() > 0; i++ {
		n := th.pop()

		if expT.IsZero() {
			expT = n.ExpireTime
			continue
		}

		if n.ExpireTime.Before(expT) {
			t.Errorf("n.ExpireTime.Before(expT) = %v, want = false", n.ExpireTime.Before(expT))
		}

		// check second last
		if i == heapN-3 {
			if n.Path != nodesExpSoonestLast[4].Path {
				t.Errorf("n.Path = %s, want = %s", n.Path, nodesExpSoonestLast[4].Path)
			}
		}

		// check third last
		if i == heapN-4 {
			if n.Path != nodesExpSoonestLast[6].Path {
				t.Errorf("n.Path = %s, want = %s", n.Path, nodesExpSoonestLast[6].Path)
			}
		}
	}
}
