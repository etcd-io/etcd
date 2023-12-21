// Copyright 2016 The etcd Authors
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

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/snapshot"
	v3rpc "go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/expect"
	"go.etcd.io/etcd/pkg/testutil"
)

func TestCtlV3Snapshot(t *testing.T) { testCtl(t, snapshotTest) }

func snapshotTest(cx ctlCtx) {
	maintenanceInitKeys(cx)

	leaseID, err := ctlV3LeaseGrant(cx, 100)
	if err != nil {
		cx.t.Fatalf("snapshot: ctlV3LeaseGrant error (%v)", err)
	}
	if err = ctlV3Put(cx, "withlease", "withlease", leaseID); err != nil {
		cx.t.Fatalf("snapshot: ctlV3Put error (%v)", err)
	}

	fpath := "test1.snapshot"
	defer os.RemoveAll(fpath)

	if err = ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotTest ctlV3SnapshotSave error (%v)", err)
	}

	st, err := getSnapshotStatus(cx, fpath)
	if err != nil {
		cx.t.Fatalf("snapshotTest getSnapshotStatus error (%v)", err)
	}
	if st.Revision != 5 {
		cx.t.Fatalf("expected 4, got %d", st.Revision)
	}
	if st.TotalKey < 4 {
		cx.t.Fatalf("expected at least 4, got %d", st.TotalKey)
	}
}

func TestCtlV3SnapshotCorrupt(t *testing.T) { testCtl(t, snapshotCorruptTest) }

func snapshotCorruptTest(cx ctlCtx) {
	fpath := "test2.snapshot"
	defer os.RemoveAll(fpath)

	if err := ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotTest ctlV3SnapshotSave error (%v)", err)
	}

	// corrupt file
	f, oerr := os.OpenFile(fpath, os.O_WRONLY, 0)
	if oerr != nil {
		cx.t.Fatal(oerr)
	}
	if _, err := f.Write(make([]byte, 512)); err != nil {
		cx.t.Fatal(err)
	}
	f.Close()

	defer os.RemoveAll("snap.etcd")
	serr := spawnWithExpect(
		append(cx.PrefixArgs(), "snapshot", "restore",
			"--data-dir", "snap.etcd",
			fpath),
		"expected sha256")

	if serr != nil {
		cx.t.Fatal(serr)
	}
}

// This test ensures that the snapshot status does not modify the snapshot file
func TestCtlV3SnapshotStatusBeforeRestore(t *testing.T) { testCtl(t, snapshotStatusBeforeRestoreTest) }

func snapshotStatusBeforeRestoreTest(cx ctlCtx) {
	fpath := "test3.snapshot"
	defer os.RemoveAll(fpath)

	if err := ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotTest ctlV3SnapshotSave error (%v)", err)
	}

	// snapshot status on the fresh snapshot file
	_, err := getSnapshotStatus(cx, fpath)
	if err != nil {
		cx.t.Fatalf("snapshotTest getSnapshotStatus error (%v)", err)
	}

	defer os.RemoveAll("snap.etcd")
	serr := spawnWithExpect(
		append(cx.PrefixArgs(), "snapshot", "restore",
			"--data-dir", "snap.etcd",
			fpath),
		"added member")
	if serr != nil {
		cx.t.Fatal(serr)
	}
}

func ctlV3SnapshotSave(cx ctlCtx, fpath string) error {
	cmdArgs := append(cx.PrefixArgs(), "snapshot", "save", fpath)
	return spawnWithExpect(cmdArgs, fmt.Sprintf("Snapshot saved at %s", fpath))
}

func getSnapshotStatus(cx ctlCtx, fpath string) (snapshot.Status, error) {
	cmdArgs := append(cx.PrefixArgs(), "--write-out", "json", "snapshot", "status", fpath)

	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return snapshot.Status{}, err
	}
	var txt string
	txt, err = proc.Expect("totalKey")
	if err != nil {
		return snapshot.Status{}, err
	}
	if err = proc.Close(); err != nil {
		return snapshot.Status{}, err
	}

	resp := snapshot.Status{}
	dec := json.NewDecoder(strings.NewReader(txt))
	if err := dec.Decode(&resp); err == io.EOF {
		return snapshot.Status{}, err
	}
	return resp, nil
}

// TestIssue6361 ensures new member that starts with snapshot correctly
// syncs up with other members and serve correct data.
func TestIssue6361(t *testing.T) {
	defer testutil.AfterTest(t)
	mustEtcdctl(t)
	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")

	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:  1,
		initialToken: "new",
		keepDataDir:  true,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	dialTimeout := 7 * time.Second
	prefixArgs := []string{ctlBinPath, "--endpoints", strings.Join(epc.EndpointsV3(), ","), "--dial-timeout", dialTimeout.String()}

	// write some keys
	kvs := []kv{{"foo1", "val1"}, {"foo2", "val2"}, {"foo3", "val3"}}
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "put", kvs[i].key, kvs[i].val), "OK"); err != nil {
			t.Fatal(err)
		}
	}

	fpath := filepath.Join(os.TempDir(), "test.snapshot")
	defer os.RemoveAll(fpath)

	// etcdctl save snapshot
	if err = spawnWithExpect(append(prefixArgs, "snapshot", "save", fpath), fmt.Sprintf("Snapshot saved at %s", fpath)); err != nil {
		t.Fatal(err)
	}

	if err = epc.procs[0].Stop(); err != nil {
		t.Fatal(err)
	}

	newDataDir := filepath.Join(os.TempDir(), "test.data")
	defer os.RemoveAll(newDataDir)

	// etcdctl restore the snapshot
	err = spawnWithExpect([]string{ctlBinPath, "snapshot", "restore", fpath, "--name", epc.procs[0].Config().name, "--initial-cluster", epc.procs[0].Config().initialCluster, "--initial-cluster-token", epc.procs[0].Config().initialToken, "--initial-advertise-peer-urls", epc.procs[0].Config().purl.String(), "--data-dir", newDataDir}, "added member")
	if err != nil {
		t.Fatal(err)
	}

	// start the etcd member using the restored snapshot
	epc.procs[0].Config().dataDirPath = newDataDir
	for i := range epc.procs[0].Config().args {
		if epc.procs[0].Config().args[i] == "--data-dir" {
			epc.procs[0].Config().args[i+1] = newDataDir
		}
	}
	if err = epc.procs[0].Restart(); err != nil {
		t.Fatal(err)
	}

	// ensure the restored member has the correct data
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "get", kvs[i].key), kvs[i].val); err != nil {
			t.Fatal(err)
		}
	}

	// add a new member into the cluster
	clientURL := fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+30)
	peerURL := fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+31)
	err = spawnWithExpect(append(prefixArgs, "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL)), " added to cluster ")
	if err != nil {
		t.Fatal(err)
	}

	var newDataDir2 string
	newDataDir2, err = ioutil.TempDir("", "newdata2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(newDataDir2)

	name2 := "infra2"
	initialCluster2 := epc.procs[0].Config().initialCluster + fmt.Sprintf(",%s=%s", name2, peerURL)

	// start the new member
	var nepc *expect.ExpectProcess
	nepc, err = spawnCmd([]string{epc.procs[0].Config().execPath, "--name", name2,
		"--listen-client-urls", clientURL, "--advertise-client-urls", clientURL,
		"--listen-peer-urls", peerURL, "--initial-advertise-peer-urls", peerURL,
		"--initial-cluster", initialCluster2, "--initial-cluster-state", "existing", "--data-dir", newDataDir2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = nepc.Expect("enabled capabilities for version"); err != nil {
		t.Fatal(err)
	}

	prefixArgs = []string{ctlBinPath, "--endpoints", clientURL, "--dial-timeout", dialTimeout.String()}

	// ensure added member has data from incoming snapshot
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "get", kvs[i].key), kvs[i].val); err != nil {
			t.Fatal(err)
		}
	}

	if err = nepc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreCompactionRevBump(t *testing.T) {
	defer testutil.AfterTest(t)

	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:  1,
		initialToken: "new",
		keepDataDir:  true,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	dialTimeout := 10 * time.Second
	prefixArgs := []string{ctlBinPath, "--endpoints", strings.Join(epc.EndpointsV3(), ","), "--dial-timeout", dialTimeout.String()}

	ctl := newClient(t, epc.EndpointsV3(), epc.cfg.clientTLS, epc.cfg.isClientAutoTLS)
	watchCh := ctl.Watch(context.Background(), "foo", clientv3.WithPrefix())
	// flake-fix: the watch can sometimes miss the first put below causing test failure
	time.Sleep(100 * time.Millisecond)

	kvs := []kv{{"foo1", "val1"}, {"foo2", "val2"}, {"foo3", "val3"}}
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "put", kvs[i].key, kvs[i].val), "OK"); err != nil {
			t.Fatal(err)
		}
	}

	watchTimeout := 1 * time.Second
	watchRes, err := keyValuesFromWatchChan(watchCh, len(kvs), watchTimeout)
	require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
	require.Equal(t, kvs, watchRes)

	// ensure we get the right revision back for each of the keys
	currentRev := 4
	baseRev := 2
	hasKVs(t, ctl, kvs, currentRev, baseRev)

	fpath := filepath.Join(t.TempDir(), "test.snapshot")

	t.Log("etcdctl saving snapshot...")
	require.NoError(t, spawnWithExpect(append(prefixArgs, "snapshot", "save", fpath), fmt.Sprintf("Snapshot saved at %s", fpath)))

	// add some more kvs that are not in the snapshot that will be lost after restore
	unsnappedKVs := []kv{{"unsnapped1", "one"}, {"unsnapped2", "two"}, {"unsnapped3", "three"}}
	for i := range unsnappedKVs {
		if err = spawnWithExpect(append(prefixArgs, "put", unsnappedKVs[i].key, unsnappedKVs[i].val), "OK"); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Stopping the original server...")
	require.NoError(t, epc.Stop())

	newDataDir := filepath.Join(t.TempDir(), "test.data")
	t.Log("etcdctl restoring the snapshot...")
	bumpAmount := 10000

	err = spawnWithExpect([]string{
		ctlBinPath,
		"snapshot",
		"restore", fpath,
		"--name", epc.procs[0].Config().name,
		"--initial-cluster", epc.procs[0].Config().initialCluster,
		"--initial-cluster-token", epc.procs[0].Config().initialToken,
		"--initial-advertise-peer-urls", epc.procs[0].Config().purl.String(),
		"--bump-revision", fmt.Sprintf("%d", bumpAmount),
		"--mark-compacted",
		"--data-dir", newDataDir,
	}, "added member")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("(Re)starting the etcd member using the restored snapshot...")
	epc.procs[0].Config().dataDirPath = newDataDir
	for i := range epc.procs[0].Config().args {
		if epc.procs[0].Config().args[i] == "--data-dir" {
			epc.procs[0].Config().args[i+1] = newDataDir
		}
	}

	require.NoError(t, epc.Restart())

	t.Log("Ensuring the restored member has the correct data...")
	hasKVs(t, ctl, kvs, currentRev, baseRev)

	for i := range unsnappedKVs {
		v, err := ctl.Get(context.Background(), unsnappedKVs[i].key)
		require.NoError(t, err)
		require.Equal(t, int64(0), v.Count)
	}

	cancelResult, ok := <-watchCh
	require.True(t, ok, "watchChannel should be open")
	require.Equal(t, v3rpc.ErrCompacted, cancelResult.Err())
	require.Truef(t, cancelResult.Canceled, "expected ongoing watch to be cancelled after restoring with --mark-compacted")
	require.Equal(t, int64(bumpAmount+currentRev), cancelResult.CompactRevision)
	_, ok = <-watchCh
	require.False(t, ok, "watchChannel should be closed after restoring with --mark-compacted")

	// clients might restart the watch at the old base revision, that should not yield any new data
	// everything up until bumpAmount+currentRev should return "already compacted"
	for i := bumpAmount - 2; i < bumpAmount+currentRev; i++ {
		watchCh = ctl.Watch(context.Background(), "foo", clientv3.WithPrefix(), clientv3.WithRev(int64(i)))
		cancelResult := <-watchCh
		require.Equal(t, v3rpc.ErrCompacted, cancelResult.Err())
		require.Truef(t, cancelResult.Canceled, "expected ongoing watch to be cancelled after restoring with --mark-compacted")
		require.Equal(t, int64(bumpAmount+currentRev), cancelResult.CompactRevision)
	}

	// a watch after that revision should yield successful results when a new put arrives
	ctx, cancel := context.WithTimeout(context.Background(), watchTimeout*5)
	defer cancel()
	watchCh = ctl.Watch(ctx, "foo", clientv3.WithPrefix(), clientv3.WithRev(int64(bumpAmount+currentRev+1)))
	if err = spawnWithExpect(append(prefixArgs, "put", "foo4", "val4"), "OK"); err != nil {
		t.Fatal(err)
	}
	watchRes, err = keyValuesFromWatchChan(watchCh, 1, watchTimeout)
	require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
	require.Equal(t, []kv{{"foo4", "val4"}}, watchRes)

}

func hasKVs(t *testing.T, ctl *clientv3.Client, kvs []kv, currentRev int, baseRev int) {
	for i := range kvs {
		v, err := ctl.Get(context.Background(), kvs[i].key)
		require.NoError(t, err)
		require.Equal(t, int64(1), v.Count)
		require.Equal(t, kvs[i].val, string(v.Kvs[0].Value))
		require.Equal(t, int64(baseRev+i), v.Kvs[0].CreateRevision)
		require.Equal(t, int64(baseRev+i), v.Kvs[0].ModRevision)
		require.Equal(t, int64(1), v.Kvs[0].Version)
	}
}

func keyValuesFromWatchResponse(resp clientv3.WatchResponse) (kvs []kv) {
	for _, event := range resp.Events {
		kvs = append(kvs, kv{string(event.Kv.Key), string(event.Kv.Value)})
	}
	return kvs
}

func keyValuesFromWatchChan(wch clientv3.WatchChan, wantedLen int, timeout time.Duration) (kvs []kv, err error) {
	for {
		select {
		case watchResp, ok := <-wch:
			if ok {
				kvs = append(kvs, keyValuesFromWatchResponse(watchResp)...)
				if len(kvs) == wantedLen {
					return kvs, nil
				}
			}
		case <-time.After(timeout):
			return nil, errors.New("closed watcher channel should not block")
		}
	}
}
