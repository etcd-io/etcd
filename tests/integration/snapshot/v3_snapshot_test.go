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

package snapshot_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/cpuutil"
	"go.etcd.io/etcd/server/v3/embed"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestSnapshotV3RestoreSingle tests single node cluster restoring
// from a snapshot file.
func TestSnapshotV3RestoreSingle(t *testing.T) {
	integration2.BeforeTest(t)
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)

	clusterN := 1
	urls := newEmbedURLs(t, clusterN*2)
	cURLs, pURLs := urls[:clusterN], urls[clusterN:]

	cfg := integration2.NewEmbedConfig(t, "s1")
	cfg.InitialClusterToken = testClusterTkn
	cfg.ClusterState = "existing"
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = cURLs, cURLs
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = pURLs, pURLs
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, pURLs[0].String())

	sp := snapshot.NewV3(zaptest.NewLogger(t))
	pss := make([]string, 0, len(pURLs))
	for _, p := range pURLs {
		pss = append(pss, p.String())
	}
	err := sp.Restore(snapshot.RestoreConfig{
		SnapshotPath:        dbPath,
		Name:                cfg.Name,
		OutputDataDir:       cfg.Dir,
		InitialCluster:      cfg.InitialCluster,
		InitialClusterToken: cfg.InitialClusterToken,
		PeerURLs:            pss,
	})
	require.NoError(t, err)

	srv, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	defer func() {
		srv.Close()
	}()
	select {
	case <-srv.Server.ReadyNotify():
	case <-time.After(3 * time.Second):
		t.Fatalf("failed to start restored etcd member")
	}

	var cli *clientv3.Client
	cli, err = integration2.NewClient(t, clientv3.Config{Endpoints: []string{cfg.AdvertiseClientUrls[0].String()}})
	require.NoError(t, err)
	defer cli.Close()
	for i := range kvs {
		var gresp *clientv3.GetResponse
		gresp, err = cli.Get(context.Background(), kvs[i].k)
		require.NoError(t, err)
		if string(gresp.Kvs[0].Value) != kvs[i].v {
			t.Fatalf("#%d: value expected %s, got %s", i, kvs[i].v, gresp.Kvs[0].Value)
		}
	}
}

// TestSnapshotV3RestoreMulti ensures that multiple members
// can boot into the same cluster after being restored from a same
// snapshot file.
func TestSnapshotV3RestoreMulti(t *testing.T) {
	integration2.BeforeTest(t)
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)

	clusterN := 3
	cURLs, _, srvs := restoreCluster(t, clusterN, dbPath)
	defer func() {
		for i := 0; i < clusterN; i++ {
			srvs[i].Close()
		}
	}()

	// wait for leader election
	time.Sleep(time.Second)

	for i := 0; i < clusterN; i++ {
		cli, err := integration2.NewClient(t, clientv3.Config{Endpoints: []string{cURLs[i].String()}})
		require.NoError(t, err)
		defer cli.Close()
		for i := range kvs {
			var gresp *clientv3.GetResponse
			gresp, err = cli.Get(context.Background(), kvs[i].k)
			require.NoError(t, err)
			if string(gresp.Kvs[0].Value) != kvs[i].v {
				t.Fatalf("#%d: value expected %s, got %s", i, kvs[i].v, gresp.Kvs[0].Value)
			}
		}
	}
}

// TestCorruptedBackupFileCheck tests if we can correctly identify a corrupted backup file.
func TestCorruptedBackupFileCheck(t *testing.T) {
	if cpuutil.ByteOrder() == binary.BigEndian {
		t.Skipf("skipping on big endian platforms since testdata/corrupted_backup.db is in little endian format")
	}
	dbPath := testutils.MustAbsPath("testdata/corrupted_backup.db")
	integration2.BeforeTest(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("test file [%s] does not exist: %v", dbPath, err)
	}

	sp := snapshot.NewV3(zaptest.NewLogger(t))
	_, err := sp.Status(dbPath)
	expectedErrKeywords := "snapshot file integrity check failed"
	/* example error message:
	snapshot file integrity check failed. 2 errors found.
	page 3: already freed
	page 4: unreachable unfreed
	*/
	if err == nil {
		t.Error("expected error due to corrupted snapshot file, got no error")
	}
	if !strings.Contains(err.Error(), expectedErrKeywords) {
		t.Errorf("expected error message to contain the following keywords:\n%s\n"+
			"actual error message:\n%s",
			expectedErrKeywords, err.Error())
	}
}

type kv struct {
	k, v string
}

// creates a snapshot file and returns the file path.
func createSnapshotFile(t *testing.T, kvs []kv) string {
	testutil.SkipTestIfShortMode(t,
		"Snapshot creation tests are depending on embedded etcd server so are integration-level tests.")
	clusterN := 1
	urls := newEmbedURLs(t, clusterN*2)
	cURLs, pURLs := urls[:clusterN], urls[clusterN:]

	cfg := integration2.NewEmbedConfig(t, "default")
	cfg.ClusterState = "new"
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = cURLs, cURLs
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = pURLs, pURLs
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, pURLs[0].String())
	srv, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	defer func() {
		srv.Close()
	}()
	select {
	case <-srv.Server.ReadyNotify():
	case <-time.After(3 * time.Second):
		t.Fatalf("failed to start embed.Etcd for creating snapshots")
	}

	ccfg := clientv3.Config{Endpoints: []string{cfg.AdvertiseClientUrls[0].String()}}
	cli, err := integration2.NewClient(t, ccfg)
	require.NoError(t, err)
	defer cli.Close()
	for i := range kvs {
		ctx, cancel := context.WithTimeout(context.Background(), testutil.RequestTimeout)
		_, err = cli.Put(ctx, kvs[i].k, kvs[i].v)
		cancel()
		require.NoError(t, err)
	}

	sp := snapshot.NewV3(zaptest.NewLogger(t))
	dpPath := filepath.Join(t.TempDir(), fmt.Sprintf("snapshot%d.db", time.Now().Nanosecond()))
	_, err = sp.Save(context.Background(), ccfg, dpPath)
	require.NoError(t, err)
	return dpPath
}

const testClusterTkn = "tkn"

func restoreCluster(t *testing.T, clusterN int, dbPath string) (
	cURLs []url.URL,
	pURLs []url.URL,
	srvs []*embed.Etcd,
) {
	urls := newEmbedURLs(t, clusterN*2)
	cURLs, pURLs = urls[:clusterN], urls[clusterN:]

	ics := ""
	for i := 0; i < clusterN; i++ {
		ics += fmt.Sprintf(",m%d=%s", i, pURLs[i].String())
	}
	ics = ics[1:]

	cfgs := make([]*embed.Config, clusterN)
	for i := 0; i < clusterN; i++ {
		cfg := integration2.NewEmbedConfig(t, fmt.Sprintf("m%d", i))
		cfg.InitialClusterToken = testClusterTkn
		cfg.ClusterState = "existing"
		cfg.ListenClientUrls, cfg.AdvertiseClientUrls = []url.URL{cURLs[i]}, []url.URL{cURLs[i]}
		cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = []url.URL{pURLs[i]}, []url.URL{pURLs[i]}
		cfg.InitialCluster = ics

		sp := snapshot.NewV3(
			zaptest.NewLogger(t, zaptest.Level(zapcore.InfoLevel)).Named(cfg.Name).Named("sm"))

		err := sp.Restore(snapshot.RestoreConfig{
			SnapshotPath:        dbPath,
			Name:                cfg.Name,
			OutputDataDir:       cfg.Dir,
			PeerURLs:            []string{pURLs[i].String()},
			InitialCluster:      ics,
			InitialClusterToken: cfg.InitialClusterToken,
		})
		require.NoError(t, err)

		cfgs[i] = cfg
	}

	sch := make(chan *embed.Etcd, len(cfgs))
	for i := range cfgs {
		go func(idx int) {
			srv, err := embed.StartEtcd(cfgs[idx])
			if err != nil {
				t.Error(err)
			}

			<-srv.Server.ReadyNotify()
			sch <- srv
		}(i)
	}

	srvs = make([]*embed.Etcd, clusterN)
	for i := 0; i < clusterN; i++ {
		select {
		case srv := <-sch:
			srvs[i] = srv
		case <-time.After(5 * time.Second):
			t.Fatalf("#%d: failed to start embed.Etcd", i)
		}
	}
	return cURLs, pURLs, srvs
}

// TODO: TLS
func newEmbedURLs(t testutil.TB, n int) (urls []url.URL) {
	urls = make([]url.URL, n)
	for i := 0; i < n; i++ {
		l := integration2.NewLocalListener(t)
		defer l.Close()

		u, err := url.Parse(fmt.Sprintf("unix://%s", l.Addr()))
		require.NoError(t, err)
		urls[i] = *u
	}
	return urls
}
