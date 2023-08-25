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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v3rpc "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
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

	fpath := filepath.Join(cx.t.TempDir(), "snapshot")
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
	fpath := filepath.Join(cx.t.TempDir(), "snapshot")
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

	datadir := cx.t.TempDir()

	serr := e2e.SpawnWithExpectWithEnv(
		append(cx.PrefixArgsUtl(), "snapshot", "restore",
			"--data-dir", datadir,
			fpath),
		cx.envMap,
		expect.ExpectedResponse{Value: "expected sha256"})
	require.ErrorContains(cx.t, serr, "Error: expected sha256")
}

// TestCtlV3SnapshotStatusBeforeRestore ensures that the snapshot
// status does not modify the snapshot file
func TestCtlV3SnapshotStatusBeforeRestore(t *testing.T) {
	testCtl(t, snapshotStatusBeforeRestoreTest)
}

func snapshotStatusBeforeRestoreTest(cx ctlCtx) {
	fpath := filepath.Join(cx.t.TempDir(), "snapshot")
	defer os.RemoveAll(fpath)

	if err := ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotTest ctlV3SnapshotSave error (%v)", err)
	}

	// snapshot status on the fresh snapshot file
	_, err := getSnapshotStatus(cx, fpath)
	if err != nil {
		cx.t.Fatalf("snapshotTest getSnapshotStatus error (%v)", err)
	}

	dataDir := cx.t.TempDir()
	defer os.RemoveAll(dataDir)
	serr := e2e.SpawnWithExpectWithEnv(
		append(cx.PrefixArgsUtl(), "snapshot", "restore",
			"--data-dir", dataDir,
			fpath),
		cx.envMap,
		expect.ExpectedResponse{Value: "added member"})
	if serr != nil {
		cx.t.Fatal(serr)
	}
}

func ctlV3SnapshotSave(cx ctlCtx, fpath string) error {
	cmdArgs := append(cx.PrefixArgs(), "snapshot", "save", fpath)
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: fmt.Sprintf("Snapshot saved at %s", fpath)})
}

func getSnapshotStatus(cx ctlCtx, fpath string) (snapshot.Status, error) {
	cmdArgs := append(cx.PrefixArgsUtl(), "--write-out", "json", "snapshot", "status", fpath)

	proc, err := e2e.SpawnCmd(cmdArgs, nil)
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

func TestIssue6361(t *testing.T) { testIssue6361(t) }

// TestIssue6361 ensures new member that starts with snapshot correctly
// syncs up with other members and serve correct data.
func testIssue6361(t *testing.T) {
	{
		// This tests is pretty flaky on semaphoreci as of 2021-01-10.
		// TODO: Remove when the flakiness source is identified.
		oldenv := os.Getenv("EXPECT_DEBUG")
		defer os.Setenv("EXPECT_DEBUG", oldenv)
		os.Setenv("EXPECT_DEBUG", "1")
	}

	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(1),
		e2e.WithKeepDataDir(true),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	dialTimeout := 10 * time.Second
	prefixArgs := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ","), "--dial-timeout", dialTimeout.String()}

	t.Log("Writing some keys...")
	kvs := []kv{{"foo1", "val1"}, {"foo2", "val2"}, {"foo3", "val3"}}
	for i := range kvs {
		if err = e2e.SpawnWithExpect(append(prefixArgs, "put", kvs[i].key, kvs[i].val), expect.ExpectedResponse{Value: "OK"}); err != nil {
			t.Fatal(err)
		}
	}

	fpath := filepath.Join(t.TempDir(), "test.snapshot")

	t.Log("etcdctl saving snapshot...")
	if err = e2e.SpawnWithExpects(append(prefixArgs, "snapshot", "save", fpath),
		nil,
		expect.ExpectedResponse{Value: fmt.Sprintf("Snapshot saved at %s", fpath)},
	); err != nil {
		t.Fatal(err)
	}

	t.Log("Stopping the original server...")
	if err = epc.Procs[0].Stop(); err != nil {
		t.Fatal(err)
	}

	newDataDir := filepath.Join(t.TempDir(), "test.data")
	t.Log("etcdctl restoring the snapshot...")
	err = e2e.SpawnWithExpect([]string{
		e2e.BinPath.Etcdutl, "snapshot", "restore", fpath,
		"--name", epc.Procs[0].Config().Name,
		"--initial-cluster", epc.Procs[0].Config().InitialCluster,
		"--initial-cluster-token", epc.Procs[0].Config().InitialToken,
		"--initial-advertise-peer-urls", epc.Procs[0].Config().PeerURL.String(),
		"--data-dir", newDataDir},
		expect.ExpectedResponse{Value: "added member"})
	if err != nil {
		t.Fatal(err)
	}

	t.Log("(Re)starting the etcd member using the restored snapshot...")
	epc.Procs[0].Config().DataDirPath = newDataDir
	for i := range epc.Procs[0].Config().Args {
		if epc.Procs[0].Config().Args[i] == "--data-dir" {
			epc.Procs[0].Config().Args[i+1] = newDataDir
		}
	}
	if err = epc.Procs[0].Restart(context.TODO()); err != nil {
		t.Fatal(err)
	}

	t.Log("Ensuring the restored member has the correct data...")
	for i := range kvs {
		if err = e2e.SpawnWithExpect(append(prefixArgs, "get", kvs[i].key), expect.ExpectedResponse{Value: kvs[i].val}); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Adding new member into the cluster")
	clientURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+30)
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+31)
	err = e2e.SpawnWithExpect(append(prefixArgs, "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL)), expect.ExpectedResponse{Value: " added to cluster "})
	if err != nil {
		t.Fatal(err)
	}

	newDataDir2 := t.TempDir()
	defer os.RemoveAll(newDataDir2)

	name2 := "infra2"
	initialCluster2 := epc.Procs[0].Config().InitialCluster + fmt.Sprintf(",%s=%s", name2, peerURL)

	t.Log("Starting the new member")
	// start the new member
	var nepc *expect.ExpectProcess
	nepc, err = e2e.SpawnCmd([]string{epc.Procs[0].Config().ExecPath, "--name", name2,
		"--listen-client-urls", clientURL, "--advertise-client-urls", clientURL,
		"--listen-peer-urls", peerURL, "--initial-advertise-peer-urls", peerURL,
		"--initial-cluster", initialCluster2, "--initial-cluster-state", "existing", "--data-dir", newDataDir2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = nepc.Expect("ready to serve client requests"); err != nil {
		t.Fatal(err)
	}

	prefixArgs = []string{e2e.BinPath.Etcdctl, "--endpoints", clientURL, "--dial-timeout", dialTimeout.String()}

	t.Log("Ensuring added member has data from incoming snapshot...")
	for i := range kvs {
		if err = e2e.SpawnWithExpect(append(prefixArgs, "get", kvs[i].key), expect.ExpectedResponse{Value: kvs[i].val}); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Stopping the second member")
	if err = nepc.Stop(); err != nil {
		t.Fatal(err)
	}
	t.Log("Test logic done")
}

// TestCtlV3SnapshotVersion is for storageVersion to be stored, all fields
// expected 3.6 fields need to be set. This happens after first WAL snapshot.
// In this test we lower SnapshotCount to 1 to ensure WAL snapshot is triggered.
func TestCtlV3SnapshotVersion(t *testing.T) {
	testCtl(t, snapshotVersionTest, withCfg(*e2e.NewConfig(e2e.WithSnapshotCount(1))))
}

func snapshotVersionTest(cx ctlCtx) {
	maintenanceInitKeys(cx)

	fpath := filepath.Join(cx.t.TempDir(), "snapshot")
	defer os.RemoveAll(fpath)

	if err := ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotVersionTest ctlV3SnapshotSave error (%v)", err)
	}

	st, err := getSnapshotStatus(cx, fpath)
	if err != nil {
		cx.t.Fatalf("snapshotVersionTest getSnapshotStatus error (%v)", err)
	}
	if st.Version != "3.6.0" {
		cx.t.Fatalf("expected %q, got %q", "3.6.0", st.Version)
	}
}

func TestRestoreCompactionRevBump(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(1),
		e2e.WithKeepDataDir(true),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	ctl := epc.Etcdctl()

	watchCh := ctl.Watch(context.Background(), "foo", config.WatchOptions{Prefix: true})
	// flake-fix: the watch can sometimes miss the first put below causing test failure
	time.Sleep(100 * time.Millisecond)

	kvs := []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}}
	for i := range kvs {
		require.NoError(t, ctl.Put(context.Background(), kvs[i].Key, kvs[i].Val, config.PutOptions{}))
	}

	watchTimeout := 1 * time.Second
	watchRes, err := testutils.KeyValuesFromWatchChan(watchCh, len(kvs), watchTimeout)
	require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
	require.Equal(t, kvs, watchRes)

	// ensure we get the right revision back for each of the keys
	currentRev := 4
	baseRev := 2
	hasKVs(t, ctl, kvs, currentRev, baseRev)

	fpath := filepath.Join(t.TempDir(), "test.snapshot")

	t.Log("etcdctl saving snapshot...")
	cmdPrefix := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ",")}
	require.NoError(t, e2e.SpawnWithExpects(append(cmdPrefix, "snapshot", "save", fpath), nil, expect.ExpectedResponse{Value: fmt.Sprintf("Snapshot saved at %s", fpath)}))

	// add some more kvs that are not in the snapshot that will be lost after restore
	unsnappedKVs := []testutils.KV{{Key: "unsnapped1", Val: "one"}, {Key: "unsnapped2", Val: "two"}, {Key: "unsnapped3", Val: "three"}}
	for i := range unsnappedKVs {
		require.NoError(t, ctl.Put(context.Background(), unsnappedKVs[i].Key, unsnappedKVs[i].Val, config.PutOptions{}))
	}

	membersBefore, err := ctl.MemberList(context.Background(), false)
	require.NoError(t, err)

	t.Log("Stopping the original server...")
	require.NoError(t, epc.Stop())

	newDataDir := filepath.Join(t.TempDir(), "test.data")
	t.Log("etcdctl restoring the snapshot...")
	bumpAmount := 10000
	err = e2e.SpawnWithExpect([]string{
		e2e.BinPath.Etcdutl,
		"snapshot",
		"restore", fpath,
		"--name", epc.Procs[0].Config().Name,
		"--initial-cluster", epc.Procs[0].Config().InitialCluster,
		"--initial-cluster-token", epc.Procs[0].Config().InitialToken,
		"--initial-advertise-peer-urls", epc.Procs[0].Config().PeerURL.String(),
		"--bump-revision", fmt.Sprintf("%d", bumpAmount),
		"--mark-compacted",
		"--data-dir", newDataDir,
	}, expect.ExpectedResponse{Value: "added member"})
	require.NoError(t, err)

	t.Log("(Re)starting the etcd member using the restored snapshot...")
	epc.Procs[0].Config().DataDirPath = newDataDir

	for i := range epc.Procs[0].Config().Args {
		if epc.Procs[0].Config().Args[i] == "--data-dir" {
			epc.Procs[0].Config().Args[i+1] = newDataDir
		}
	}

	// Verify that initial snapshot is created by the restore operation
	verifySnapshotMembers(t, epc, membersBefore)

	require.NoError(t, epc.Restart(context.Background()))

	t.Log("Ensuring the restored member has the correct data...")
	hasKVs(t, ctl, kvs, currentRev, baseRev)
	for i := range unsnappedKVs {
		v, err := ctl.Get(context.Background(), unsnappedKVs[i].Key, config.GetOptions{})
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
		watchCh = ctl.Watch(context.Background(), "foo", config.WatchOptions{Prefix: true, Revision: int64(i)})
		cancelResult := <-watchCh
		require.Equal(t, v3rpc.ErrCompacted, cancelResult.Err())
		require.Truef(t, cancelResult.Canceled, "expected ongoing watch to be cancelled after restoring with --mark-compacted")
		require.Equal(t, int64(bumpAmount+currentRev), cancelResult.CompactRevision)
	}

	// a watch after that revision should yield successful results when a new put arrives
	ctx, cancel := context.WithTimeout(context.Background(), watchTimeout*5)
	defer cancel()
	watchCh = ctl.Watch(ctx, "foo", config.WatchOptions{Prefix: true, Revision: int64(bumpAmount + currentRev + 1)})
	require.NoError(t, ctl.Put(context.Background(), "foo4", "val4", config.PutOptions{}))
	watchRes, err = testutils.KeyValuesFromWatchChan(watchCh, 1, watchTimeout)
	require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
	require.Equal(t, []testutils.KV{{Key: "foo4", Val: "val4"}}, watchRes)
}

func hasKVs(t *testing.T, ctl *e2e.EtcdctlV3, kvs []testutils.KV, currentRev int, baseRev int) {
	for i := range kvs {
		v, err := ctl.Get(context.Background(), kvs[i].Key, config.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int64(1), v.Count)
		require.Equal(t, kvs[i].Val, string(v.Kvs[0].Value))
		require.Equal(t, int64(baseRev+i), v.Kvs[0].CreateRevision)
		require.Equal(t, int64(baseRev+i), v.Kvs[0].ModRevision)
		require.Equal(t, int64(1), v.Kvs[0].Version)
	}
}
