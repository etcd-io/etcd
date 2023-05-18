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
	"net"
	"net/http"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestCompactionHash tests the compaction hash
// TODO: Change this to fuzz test
func TestCompactionHash(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient(t)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", clus.Members[0].PeerURLs[0].Host)
			},
		},
	}

	testutil.TestCompactionHash(context.Background(), t, hashTestCase{cc, clus.Members[0].GRPCURL(), client, clus.Members[0].Server}, 1000)
}

type hashTestCase struct {
	*clientv3.Client
	url    string
	http   *http.Client
	server *etcdserver.EtcdServer
}

func (tc hashTestCase) Put(ctx context.Context, key, value string) error {
	_, err := tc.Client.Put(ctx, key, value)
	return err
}

func (tc hashTestCase) Delete(ctx context.Context, key string) error {
	_, err := tc.Client.Delete(ctx, key)
	return err
}

func (tc hashTestCase) HashByRev(ctx context.Context, rev int64) (testutil.KeyValueHash, error) {
	resp, err := etcdserver.HashByRev(ctx, tc.server.Cluster().ID(), tc.http, "http://unix", rev)
	return testutil.KeyValueHash{Hash: resp.Hash, CompactRevision: resp.CompactRevision, Revision: resp.Header.Revision}, err
}

func (tc hashTestCase) Defrag(ctx context.Context) error {
	_, err := tc.Client.Defragment(ctx, tc.url)
	return err
}

func (tc hashTestCase) Compact(ctx context.Context, rev int64) error {
	_, err := tc.Client.Compact(ctx, rev)
	// Wait for compaction to be compacted
	time.Sleep(50 * time.Millisecond)
	return err
}
