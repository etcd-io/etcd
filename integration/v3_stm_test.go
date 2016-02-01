// Copyright 2016 CoreOS, Inc.
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
// limitations under the License.package recipe
package integration

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/coreos/etcd/contrib/recipes"
)

// TestSTMConflict tests that conflicts are retried.
func TestSTMConflict(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	etcdc := clus.RandClient()
	keys := make([]*recipe.RemoteKV, 5)
	for i := 0; i < len(keys); i++ {
		rk, err := recipe.NewKV(etcdc, fmt.Sprintf("foo-%d", i), "100", 0)
		if err != nil {
			t.Fatalf("could not make key (%v)", err)
		}
		keys[i] = rk
	}

	errc := make([]<-chan error, len(keys))
	for i, rk := range keys {
		curEtcdc := clus.RandClient()
		srcKey := rk.Key()
		applyf := func(stm *recipe.STM) error {
			src, err := stm.Get(srcKey)
			if err != nil {
				return err
			}
			// must be different key to avoid double-adding
			dstKey := srcKey
			for dstKey == srcKey {
				dstKey = keys[rand.Intn(len(keys))].Key()
			}
			dst, err := stm.Get(dstKey)
			if err != nil {
				return err
			}
			srcV, _ := strconv.ParseInt(src, 10, 64)
			dstV, _ := strconv.ParseInt(dst, 10, 64)
			xfer := int64(rand.Intn(int(srcV)) / 2)
			stm.Put(srcKey, fmt.Sprintf("%d", srcV-xfer))
			stm.Put(dstKey, fmt.Sprintf("%d", dstV+xfer))
			return nil
		}
		errc[i] = recipe.NewSTM(curEtcdc, applyf)
	}

	// wait for txns
	for _, ch := range errc {
		if err := <-ch; err != nil {
			t.Fatalf("apply failed (%v)", err)
		}
	}

	// ensure sum matches initial sum
	sum := 0
	for _, oldRK := range keys {
		rk, err := recipe.GetRemoteKV(etcdc, oldRK.Key())
		if err != nil {
			t.Fatalf("couldn't fetch key %s (%v)", oldRK.Key(), err)
		}
		v, _ := strconv.ParseInt(rk.Value(), 10, 64)
		sum += int(v)
	}
	if sum != len(keys)*100 {
		t.Fatalf("bad sum. got %d, expected %d", sum, len(keys)*100)
	}
}

// TestSTMPutNewKey confirms a STM put on a new key is visible after commit.
func TestSTMPutNewKey(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	etcdc := clus.RandClient()
	applyf := func(stm *recipe.STM) error {
		stm.Put("foo", "bar")
		return nil
	}
	errc := recipe.NewSTM(etcdc, applyf)
	if err := <-errc; err != nil {
		t.Fatalf("error on stm txn (%v)", err)
	}

	rk, err := recipe.GetRemoteKV(etcdc, "foo")
	if err != nil {
		t.Fatalf("error fetching key (%v)", err)
	}
	if rk.Value() != "bar" {
		t.Fatalf("bad value. got %v, expected bar", rk.Value())
	}
}

// TestSTMAbort tests that an aborted txn does not modify any keys.
func TestSTMAbort(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	etcdc := clus.RandClient()
	applyf := func(stm *recipe.STM) error {
		stm.Put("foo", "baz")
		stm.Abort()
		stm.Put("foo", "baz")
		return nil
	}
	errc := recipe.NewSTM(etcdc, applyf)
	if err := <-errc; err != nil {
		t.Fatalf("error on stm txn (%v)", err)
	}

	rk, err := recipe.GetRemoteKV(etcdc, "foo")
	if err != nil {
		t.Fatalf("error fetching key (%v)", err)
	}
	if rk.Value() != "" {
		t.Fatalf("bad value. got %v, expected empty string", rk.Value())
	}
}
