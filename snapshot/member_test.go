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

package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/embed"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/pkg/testutil"
)

// TestSnapshotV3RestoreMultiMemberAdd ensures that multiple members
// can boot into the same cluster after being restored from a same
// snapshot file, and also be able to add another member to the cluster.
func TestSnapshotV3RestoreMultiMemberAdd(t *testing.T) {
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)

	clusterN := 3
	cURLs, pURLs, srvs := restoreCluster(t, clusterN, dbPath)
	defer func() {
		for i := 0; i < clusterN; i++ {
			os.RemoveAll(srvs[i].Config().Dir)
			srvs[i].Close()
		}
	}()

	// wait for health interval + leader election
	time.Sleep(etcdserver.HealthInterval + 2*time.Second)

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{cURLs[0].String()}})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	urls := newEmbedURLs(2)
	newCURLs, newPURLs := urls[:1], urls[1:]
	if _, err = cli.MemberAdd(context.Background(), []string{newPURLs[0].String()}); err != nil {
		t.Fatal(err)
	}

	// wait for membership reconfiguration apply
	time.Sleep(testutil.ApplyTimeout)

	cfg := embed.NewConfig()
	cfg.Name = "3"
	cfg.InitialClusterToken = testClusterTkn
	cfg.ClusterState = "existing"
	cfg.LCUrls, cfg.ACUrls = newCURLs, newCURLs
	cfg.LPUrls, cfg.APUrls = newPURLs, newPURLs
	cfg.InitialCluster = ""
	for i := 0; i < clusterN; i++ {
		cfg.InitialCluster += fmt.Sprintf(",%d=%s", i, pURLs[i].String())
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
	cfg.InitialCluster += fmt.Sprintf(",%s=%s", cfg.Name, newPURLs[0].String())
	cfg.Dir = filepath.Join(os.TempDir(), fmt.Sprint(time.Now().Nanosecond()))

	srv, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(cfg.Dir)
		srv.Close()
	}()
	select {
	case <-srv.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		t.Fatalf("failed to start the newly added etcd member")
	}

	cli2, err := clientv3.New(clientv3.Config{Endpoints: []string{newCURLs[0].String()}})
	if err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), testutil.RequestTimeout)
	mresp, err := cli2.MemberList(ctx)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if len(mresp.Members) != 4 {
		t.Fatalf("expected 4 members, got %+v", mresp)
	}

	// make sure restored cluster has kept all data on recovery
	var gresp *clientv3.GetResponse
	ctx, cancel = context.WithTimeout(context.Background(), testutil.RequestTimeout)
	gresp, err = cli2.Get(ctx, "foo", clientv3.WithPrefix())
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	for i := range gresp.Kvs {
		if string(gresp.Kvs[i].Key) != kvs[i].k {
			t.Fatalf("#%d: key expected %s, got %s", i, kvs[i].k, string(gresp.Kvs[i].Key))
		}
		if string(gresp.Kvs[i].Value) != kvs[i].v {
			t.Fatalf("#%d: value expected %s, got %s", i, kvs[i].v, string(gresp.Kvs[i].Value))
		}
	}
}
