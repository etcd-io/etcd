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
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/pkg/testutil"

	"go.uber.org/zap"
)

// TestSnapshotV3RestoreSingle tests single node cluster restoring
// from a snapshot file.
func TestSnapshotV3RestoreSingle(t *testing.T) {
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)
	defer os.RemoveAll(dbPath)

	clusterN := 1
	urls := newEmbedURLs(clusterN * 2)
	cURLs, pURLs := urls[:clusterN], urls[clusterN:]

	cfg := embed.NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.Debug = false
	cfg.Name = "s1"
	cfg.InitialClusterToken = testClusterTkn
	cfg.ClusterState = "existing"
	cfg.LCUrls, cfg.ACUrls = cURLs, cURLs
	cfg.LPUrls, cfg.APUrls = pURLs, pURLs
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, pURLs[0].String())
	cfg.Dir = filepath.Join(os.TempDir(), fmt.Sprint(time.Now().Nanosecond()))

	sp := NewV3(zap.NewExample())
	pss := make([]string, 0, len(pURLs))
	for _, p := range pURLs {
		pss = append(pss, p.String())
	}
	if err := sp.Restore(RestoreConfig{
		SnapshotPath:        dbPath,
		Name:                cfg.Name,
		OutputDataDir:       cfg.Dir,
		InitialCluster:      cfg.InitialCluster,
		InitialClusterToken: cfg.InitialClusterToken,
		PeerURLs:            pss,
	}); err != nil {
		t.Fatal(err)
	}

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
	case <-time.After(3 * time.Second):
		t.Fatalf("failed to start restored etcd member")
	}

	var cli *clientv3.Client
	cli, err = clientv3.New(clientv3.Config{Endpoints: []string{cfg.ACUrls[0].String()}})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	for i := range kvs {
		var gresp *clientv3.GetResponse
		gresp, err = cli.Get(context.Background(), kvs[i].k)
		if err != nil {
			t.Fatal(err)
		}
		if string(gresp.Kvs[0].Value) != kvs[i].v {
			t.Fatalf("#%d: value expected %s, got %s", i, kvs[i].v, string(gresp.Kvs[0].Value))
		}
	}
}

// TestSnapshotV3RestoreMulti ensures that multiple members
// can boot into the same cluster after being restored from a same
// snapshot file.
func TestSnapshotV3RestoreMulti(t *testing.T) {
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)
	defer os.RemoveAll(dbPath)

	clusterN := 3
	cURLs, _, srvs := restoreCluster(t, clusterN, dbPath)
	defer func() {
		for i := 0; i < clusterN; i++ {
			os.RemoveAll(srvs[i].Config().Dir)
			srvs[i].Close()
		}
	}()

	// wait for leader election
	time.Sleep(time.Second)

	for i := 0; i < clusterN; i++ {
		cli, err := clientv3.New(clientv3.Config{Endpoints: []string{cURLs[i].String()}})
		if err != nil {
			t.Fatal(err)
		}
		defer cli.Close()
		for i := range kvs {
			var gresp *clientv3.GetResponse
			gresp, err = cli.Get(context.Background(), kvs[i].k)
			if err != nil {
				t.Fatal(err)
			}
			if string(gresp.Kvs[0].Value) != kvs[i].v {
				t.Fatalf("#%d: value expected %s, got %s", i, kvs[i].v, string(gresp.Kvs[0].Value))
			}
		}
	}
}

// TestSnapshotFilePermissions ensures that the snapshot is saved with
// the correct file permissions.
func TestSnapshotFilePermissions(t *testing.T) {
	expectedFileMode := os.FileMode(fileutil.PrivateFileMode)
	kvs := []kv{{"foo1", "bar1"}, {"foo2", "bar2"}, {"foo3", "bar3"}}
	dbPath := createSnapshotFile(t, kvs)
	defer os.RemoveAll(dbPath)

	dbInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to get test snapshot file status: %v", err)
	}
	actualFileMode := dbInfo.Mode()

	if expectedFileMode != actualFileMode {
		t.Fatalf("expected test snapshot file mode %s, got %s:", expectedFileMode, actualFileMode)
	}
}

// TestCorruptedBackupFileCheck tests if we can correctly identify a corrupted backup file.
func TestCorruptedBackupFileCheck(t *testing.T) {
	dbPath := "testdata/corrupted_backup.db"
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("test file [%s] does not exist: %v", dbPath, err)
	}

	sp := NewV3(zap.NewExample())
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
	clusterN := 1
	urls := newEmbedURLs(clusterN * 2)
	cURLs, pURLs := urls[:clusterN], urls[clusterN:]

	cfg := embed.NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.Debug = false
	cfg.Name = "default"
	cfg.ClusterState = "new"
	cfg.LCUrls, cfg.ACUrls = cURLs, cURLs
	cfg.LPUrls, cfg.APUrls = pURLs, pURLs
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, pURLs[0].String())
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
	case <-time.After(3 * time.Second):
		t.Fatalf("failed to start embed.Etcd for creating snapshots")
	}

	ccfg := clientv3.Config{Endpoints: []string{cfg.ACUrls[0].String()}}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	for i := range kvs {
		ctx, cancel := context.WithTimeout(context.Background(), testutil.RequestTimeout)
		_, err = cli.Put(ctx, kvs[i].k, kvs[i].v)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}

	sp := NewV3(zap.NewExample())
	dpPath := filepath.Join(os.TempDir(), fmt.Sprintf("snapshot%d.db", time.Now().Nanosecond()))
	if err = sp.Save(context.Background(), ccfg, dpPath); err != nil {
		t.Fatal(err)
	}

	os.RemoveAll(cfg.Dir)
	srv.Close()
	return dpPath
}

const testClusterTkn = "tkn"

func restoreCluster(t *testing.T, clusterN int, dbPath string) (
	cURLs []url.URL,
	pURLs []url.URL,
	srvs []*embed.Etcd) {
	urls := newEmbedURLs(clusterN * 2)
	cURLs, pURLs = urls[:clusterN], urls[clusterN:]

	ics := ""
	for i := 0; i < clusterN; i++ {
		ics += fmt.Sprintf(",%d=%s", i, pURLs[i].String())
	}
	ics = ics[1:]

	cfgs := make([]*embed.Config, clusterN)
	for i := 0; i < clusterN; i++ {
		cfg := embed.NewConfig()
		cfg.Logger = "zap"
		cfg.LogOutputs = []string{"/dev/null"}
		cfg.Debug = false
		cfg.Name = fmt.Sprintf("%d", i)
		cfg.InitialClusterToken = testClusterTkn
		cfg.ClusterState = "existing"
		cfg.LCUrls, cfg.ACUrls = []url.URL{cURLs[i]}, []url.URL{cURLs[i]}
		cfg.LPUrls, cfg.APUrls = []url.URL{pURLs[i]}, []url.URL{pURLs[i]}
		cfg.InitialCluster = ics
		cfg.Dir = filepath.Join(os.TempDir(), fmt.Sprint(time.Now().Nanosecond()+i))

		sp := NewV3(zap.NewExample())
		if err := sp.Restore(RestoreConfig{
			SnapshotPath:        dbPath,
			Name:                cfg.Name,
			OutputDataDir:       cfg.Dir,
			PeerURLs:            []string{pURLs[i].String()},
			InitialCluster:      ics,
			InitialClusterToken: cfg.InitialClusterToken,
		}); err != nil {
			t.Fatal(err)
		}
		cfgs[i] = cfg
	}

	sch := make(chan *embed.Etcd)
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
func newEmbedURLs(n int) (urls []url.URL) {
	urls = make([]url.URL, n)
	for i := 0; i < n; i++ {
		rand.Seed(int64(time.Now().Nanosecond()))
		u, _ := url.Parse(fmt.Sprintf("unix://localhost:%d", rand.Intn(45000)))
		urls[i] = *u
	}
	return urls
}
