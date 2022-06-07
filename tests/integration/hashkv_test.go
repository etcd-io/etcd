// Copyright 2022 The etcd Authors
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

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
)

const (
	// Use high prime
	compactionCycle = 71
)

func TestCompactionHashHTTP(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx := context.Background()
	cc, err := clus.ClusterClient()
	if err != nil {
		t.Fatal(err)
	}

	var totalRevisions int64 = 1210
	assert.Less(t, int64(1000), totalRevisions)
	assert.Less(t, int64(compactionCycle*10), totalRevisions)
	var rev int64
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", clus.Members[0].PeerURLs[0].Host)
			},
		},
	}
	for ; rev < totalRevisions; rev += compactionCycle {
		testCompactionHash(ctx, t, cc, client, rev, rev+compactionCycle)
	}
	testCompactionHash(ctx, t, cc, client, rev, rev+totalRevisions)
}

func testCompactionHash(ctx context.Context, t *testing.T, cc *clientv3.Client, client *http.Client, start, stop int64) {
	for i := start; i <= stop; i++ {
		cc.Put(ctx, pickKey(i), fmt.Sprint(i))
	}
	hash1, err := etcdserver.HashByRev(ctx, client, "http://unix", stop)
	assert.NoError(t, err, "error on rev %v", stop)

	_, err = cc.Compact(ctx, stop)
	assert.NoError(t, err, "error on compact rev %v", stop)

	// Wait for compaction to be compacted
	time.Sleep(50 * time.Millisecond)

	hash2, err := etcdserver.HashByRev(ctx, client, "http://unix", stop)
	assert.NoError(t, err, "error on rev %v", stop)
	assert.Equal(t, hash1, hash2, "hashes do not match on rev %v", stop)
}

func pickKey(i int64) string {
	if i%(compactionCycle*2) == 30 {
		return "zenek"
	}
	if i%compactionCycle == 30 {
		return "xavery"
	}
	// Use low prime number to ensure repeats without alignment
	switch i % 7 {
	case 0:
		return "alice"
	case 1:
		return "bob"
	case 2:
		return "celine"
	case 3:
		return "dominik"
	case 4:
		return "eve"
	case 5:
		return "frederica"
	case 6:
		return "gorge"
	default:
		panic("Can't count")
	}
}
