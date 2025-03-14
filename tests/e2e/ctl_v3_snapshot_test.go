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
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	if st.TotalKey < 2 {
		cx.t.Fatalf("expected at least 2, got %d", st.TotalKey)
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
	require.NoError(cx.t, oerr)
	_, err := f.Write(make([]byte, 512))
	require.NoError(cx.t, err)
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
	require.NoError(cx.t, serr)
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
	if err := dec.Decode(&resp); errors.Is(err, io.EOF) {
		return snapshot.Status{}, err
	}
	return resp, nil
}

func TestIssue6361(t *testing.T) { testIssue6361(t) }

// TestIssue6361 ensures new member that starts with snapshot correctly
// syncs up with other members and serve correct data.
func testIssue6361(t *testing.T) {
	// This tests is pretty flaky on semaphoreci as of 2021-01-10.
	// TODO: Remove when the flakiness source is identified.
	t.Setenv("EXPECT_DEBUG", "1")

	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
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
		err = e2e.SpawnWithExpect(append(prefixArgs, "put", kvs[i].key, kvs[i].val), expect.ExpectedResponse{Value: "OK"})
		require.NoError(t, err)
	}

	fpath := filepath.Join(t.TempDir(), "test.snapshot")

	t.Log("etcdctl saving snapshot...")
	require.NoError(t, e2e.SpawnWithExpects(append(prefixArgs, "snapshot", "save", fpath),
		nil,
		expect.ExpectedResponse{Value: fmt.Sprintf("Snapshot saved at %s", fpath)},
	))

	t.Log("Stopping the original server...")
	require.NoError(t, epc.Procs[0].Stop())

	newDataDir := filepath.Join(t.TempDir(), "test.data")
	t.Log("etcdctl restoring the snapshot...")
	require.NoError(t, e2e.SpawnWithExpect([]string{
		e2e.BinPath.Etcdutl, "snapshot", "restore", fpath,
		"--name", epc.Procs[0].Config().Name,
		"--initial-cluster", epc.Procs[0].Config().InitialCluster,
		"--initial-cluster-token", epc.Procs[0].Config().InitialToken,
		"--initial-advertise-peer-urls", epc.Procs[0].Config().PeerURL.String(),
		"--data-dir", newDataDir,
	},
		expect.ExpectedResponse{Value: "added member"}))

	t.Log("(Re)starting the etcd member using the restored snapshot...")
	epc.Procs[0].Config().DataDirPath = newDataDir
	for i := range epc.Procs[0].Config().Args {
		if epc.Procs[0].Config().Args[i] == "--data-dir" {
			epc.Procs[0].Config().Args[i+1] = newDataDir
		}
	}
	require.NoError(t, epc.Procs[0].Restart(t.Context()))

	t.Log("Ensuring the restored member has the correct data...")
	for i := range kvs {
		require.NoError(t, e2e.SpawnWithExpect(append(prefixArgs, "get", kvs[i].key), expect.ExpectedResponse{Value: kvs[i].val}))
	}

	t.Log("Adding new member into the cluster")
	clientURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+30)
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+31)
	require.NoError(t, e2e.SpawnWithExpect(append(prefixArgs, "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL)), expect.ExpectedResponse{Value: " added to cluster "}))

	newDataDir2 := t.TempDir()
	defer os.RemoveAll(newDataDir2)

	name2 := "infra2"
	initialCluster2 := epc.Procs[0].Config().InitialCluster + fmt.Sprintf(",%s=%s", name2, peerURL)

	t.Log("Starting the new member")
	// start the new member
	var nepc *expect.ExpectProcess
	nepc, err = e2e.SpawnCmd([]string{
		epc.Procs[0].Config().ExecPath, "--name", name2,
		"--listen-client-urls", clientURL, "--advertise-client-urls", clientURL,
		"--listen-peer-urls", peerURL, "--initial-advertise-peer-urls", peerURL,
		"--initial-cluster", initialCluster2, "--initial-cluster-state", "existing", "--data-dir", newDataDir2,
	}, nil)
	require.NoError(t, err)
	_, err = nepc.Expect("ready to serve client requests")
	require.NoError(t, err)

	prefixArgs = []string{e2e.BinPath.Etcdctl, "--endpoints", clientURL, "--dial-timeout", dialTimeout.String()}

	t.Log("Ensuring added member has data from incoming snapshot...")
	for i := range kvs {
		require.NoError(t, e2e.SpawnWithExpect(append(prefixArgs, "get", kvs[i].key), expect.ExpectedResponse{Value: kvs[i].val}))
	}

	t.Log("Stopping the second member")
	require.NoError(t, nepc.Stop())
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

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
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

	watchCh := ctl.Watch(t.Context(), "foo", config.WatchOptions{Prefix: true})
	// flake-fix: the watch can sometimes miss the first put below causing test failure
	time.Sleep(100 * time.Millisecond)

	kvs := []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}}
	for i := range kvs {
		require.NoError(t, ctl.Put(t.Context(), kvs[i].Key, kvs[i].Val, config.PutOptions{}))
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
		require.NoError(t, ctl.Put(t.Context(), unsnappedKVs[i].Key, unsnappedKVs[i].Val, config.PutOptions{}))
	}

	membersBefore, err := ctl.MemberList(t.Context(), false)
	require.NoError(t, err)

	t.Log("Stopping the original server...")
	require.NoError(t, epc.Stop())

	newDataDir := filepath.Join(t.TempDir(), "test.data")
	t.Log("etcdctl restoring the snapshot...")
	bumpAmount := 10000
	require.NoError(t, e2e.SpawnWithExpect([]string{
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
	}, expect.ExpectedResponse{Value: "added member"}))

	t.Log("(Re)starting the etcd member using the restored snapshot...")
	epc.Procs[0].Config().DataDirPath = newDataDir

	for i := range epc.Procs[0].Config().Args {
		if epc.Procs[0].Config().Args[i] == "--data-dir" {
			epc.Procs[0].Config().Args[i+1] = newDataDir
		}
	}

	// Verify that initial snapshot is created by the restore operation
	verifySnapshotMembers(t, epc, membersBefore)

	require.NoError(t, epc.Restart(t.Context()))

	t.Log("Ensuring the restored member has the correct data...")
	hasKVs(t, ctl, kvs, currentRev, baseRev)
	for i := range unsnappedKVs {
		v, gerr := ctl.Get(t.Context(), unsnappedKVs[i].Key, config.GetOptions{})
		require.NoError(t, gerr)
		require.Equal(t, int64(0), v.Count)
	}

	cancelResult, ok := <-watchCh
	require.Truef(t, ok, "watchChannel should be open")
	require.Equal(t, v3rpc.ErrCompacted, cancelResult.Err())
	require.Truef(t, cancelResult.Canceled, "expected ongoing watch to be cancelled after restoring with --mark-compacted")
	require.Equal(t, int64(bumpAmount+currentRev), cancelResult.CompactRevision)
	_, ok = <-watchCh
	require.Falsef(t, ok, "watchChannel should be closed after restoring with --mark-compacted")

	// clients might restart the watch at the old base revision, that should not yield any new data
	// everything up until bumpAmount+currentRev should return "already compacted"
	for i := bumpAmount - 2; i < bumpAmount+currentRev; i++ {
		watchCh = ctl.Watch(t.Context(), "foo", config.WatchOptions{Prefix: true, Revision: int64(i)})
		cancelResult := <-watchCh
		require.Equal(t, v3rpc.ErrCompacted, cancelResult.Err())
		require.Truef(t, cancelResult.Canceled, "expected ongoing watch to be cancelled after restoring with --mark-compacted")
		require.Equal(t, int64(bumpAmount+currentRev), cancelResult.CompactRevision)
	}

	// a watch after that revision should yield successful results when a new put arrives
	ctx, cancel := context.WithTimeout(t.Context(), watchTimeout*5)
	defer cancel()
	watchCh = ctl.Watch(ctx, "foo", config.WatchOptions{Prefix: true, Revision: int64(bumpAmount + currentRev + 1)})
	require.NoError(t, ctl.Put(t.Context(), "foo4", "val4", config.PutOptions{}))
	watchRes, err = testutils.KeyValuesFromWatchChan(watchCh, 1, watchTimeout)
	require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
	require.Equal(t, []testutils.KV{{Key: "foo4", Val: "val4"}}, watchRes)
}

func hasKVs(t *testing.T, ctl *e2e.EtcdctlV3, kvs []testutils.KV, currentRev int, baseRev int) {
	for i := range kvs {
		v, err := ctl.Get(t.Context(), kvs[i].Key, config.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int64(1), v.Count)
		require.Equal(t, kvs[i].Val, string(v.Kvs[0].Value))
		require.Equal(t, int64(baseRev+i), v.Kvs[0].CreateRevision)
		require.Equal(t, int64(baseRev+i), v.Kvs[0].ModRevision)
		require.Equal(t, int64(1), v.Kvs[0].Version)
		require.GreaterOrEqual(t, int64(currentRev), v.Kvs[0].ModRevision)
	}
}

func TestBreakConsistentIndexNewerThanSnapshot(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	var snapshotCount uint64 = 50
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithKeepDataDir(true),
		e2e.WithSnapshotCount(snapshotCount),
	)
	require.NoError(t, err)
	defer epc.Close()
	member := epc.Procs[0]

	t.Log("Stop member and copy out the db file to tmp directory")
	err = member.Stop()
	require.NoError(t, err)
	dbPath := path.Join(member.Config().DataDirPath, "member", "snap", "db")
	tmpFile := path.Join(t.TempDir(), "db")
	err = copyFile(dbPath, tmpFile)
	require.NoError(t, err)

	t.Log("Ensure snapshot there is a newer snapshot")
	err = member.Start(ctx)
	require.NoError(t, err)
	generateSnapshot(t, snapshotCount, member.Etcdctl())
	_, err = member.Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: "saved snapshot"})
	require.NoError(t, err)
	err = member.Stop()
	require.NoError(t, err)

	t.Log("Start etcd with older db file")
	err = copyFile(tmpFile, dbPath)
	require.NoError(t, err)
	err = member.Start(ctx)
	require.Error(t, err)
	_, err = member.Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: "failed to find database snapshot file (snap: snapshot file doesn't exist)"})
	assert.NoError(t, err)
}

func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	if _, err = io.Copy(w, f); err != nil {
		return err
	}
	return w.Sync()
}
