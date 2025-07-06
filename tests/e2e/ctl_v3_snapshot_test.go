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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/expect"
)

func TestCtlV3Snapshot(t *testing.T)        { testCtl(t, snapshotTest) }
func TestCtlV3SnapshotEtcdutl(t *testing.T) { testCtl(t, snapshotTest, withEtcdutl()) }

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

func TestCtlV3SnapshotCorrupt(t *testing.T)        { testCtl(t, snapshotCorruptTest) }
func TestCtlV3SnapshotCorruptEtcdutl(t *testing.T) { testCtl(t, snapshotCorruptTest, withEtcdutl()) }

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

	serr := spawnWithExpectWithEnv(
		append(cx.PrefixArgsUtl(), "snapshot", "restore",
			"--data-dir", datadir,
			fpath),
		cx.envMap,
		"expected sha256")

	if serr != nil {
		cx.t.Fatal(serr)
	}
}

// This test ensures that the snapshot status does not modify the snapshot file
func TestCtlV3SnapshotStatusBeforeRestore(t *testing.T) { testCtl(t, snapshotStatusBeforeRestoreTest) }
func TestCtlV3SnapshotStatusBeforeRestoreEtcdutl(t *testing.T) {
	testCtl(t, snapshotStatusBeforeRestoreTest, withEtcdutl())
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
	serr := spawnWithExpectWithEnv(
		append(cx.PrefixArgsUtl(), "snapshot", "restore",
			"--data-dir", dataDir,
			fpath),
		cx.envMap,
		"added member")
	if serr != nil {
		cx.t.Fatal(serr)
	}
}

func ctlV3SnapshotSave(cx ctlCtx, fpath string) error {
	cmdArgs := append(cx.PrefixArgs(), "snapshot", "save", fpath)
	return spawnWithExpectWithEnv(cmdArgs, cx.envMap, fmt.Sprintf("Snapshot saved at %s", fpath))
}

func getSnapshotStatus(cx ctlCtx, fpath string) (snapshot.Status, error) {
	cmdArgs := append(cx.PrefixArgsUtl(), "--write-out", "json", "snapshot", "status", fpath)

	proc, err := spawnCmd(cmdArgs, nil)
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

func TestIssue6361(t *testing.T)        { testIssue6361(t, false) }
func TestIssue6361etcdutl(t *testing.T) { testIssue6361(t, true) }

// TestIssue6361 ensures new member that starts with snapshot correctly
// syncs up with other members and serve correct data.
func testIssue6361(t *testing.T, etcdutl bool) {
	{
		// This tests is pretty flaky on semaphoreci as of 2021-01-10.
		// TODO: Remove when the flakiness source is identified.
		oldenv := os.Getenv("EXPECT_DEBUG")
		defer os.Setenv("EXPECT_DEBUG", oldenv)
		os.Setenv("EXPECT_DEBUG", "1")
	}

	BeforeTest(t)
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

	dialTimeout := 10 * time.Second
	prefixArgs := []string{ctlBinPath, "--endpoints", strings.Join(epc.EndpointsV3(), ","), "--dial-timeout", dialTimeout.String()}

	t.Log("Writing some keys...")
	kvs := []kv{{"foo1", "val1"}, {"foo2", "val2"}, {"foo3", "val3"}}
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "put", kvs[i].key, kvs[i].val), "OK"); err != nil {
			t.Fatal(err)
		}
	}

	fpath := filepath.Join(t.TempDir(), "test.snapshot")

	t.Log("etcdctl saving snapshot...")
	if err = spawnWithExpects(append(prefixArgs, "snapshot", "save", fpath),
		nil,
		fmt.Sprintf("Snapshot saved at %s", fpath),
	); err != nil {
		t.Fatal(err)
	}

	t.Log("Stopping the original server...")
	if err = epc.procs[0].Stop(); err != nil {
		t.Fatal(err)
	}

	newDataDir := filepath.Join(t.TempDir(), "test.data")

	uctlBinPath := ctlBinPath
	if etcdutl {
		uctlBinPath = utlBinPath
	}

	t.Log("etcdctl restoring the snapshot...")
	err = spawnWithExpect([]string{uctlBinPath, "snapshot", "restore", fpath, "--name", epc.procs[0].Config().name, "--initial-cluster", epc.procs[0].Config().initialCluster, "--initial-cluster-token", epc.procs[0].Config().initialToken, "--initial-advertise-peer-urls", epc.procs[0].Config().purl.String(), "--data-dir", newDataDir}, "added member")
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
	if err = epc.procs[0].Restart(); err != nil {
		t.Fatal(err)
	}

	t.Log("Ensuring the restored member has the correct data...")
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "get", kvs[i].key), kvs[i].val); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Adding new member into the cluster")
	clientURL := fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+30)
	peerURL := fmt.Sprintf("http://localhost:%d", etcdProcessBasePort+31)
	err = spawnWithExpect(append(prefixArgs, "member", "add", "newmember", fmt.Sprintf("--peer-urls=%s", peerURL)), " added to cluster ")
	if err != nil {
		t.Fatal(err)
	}

	newDataDir2 := t.TempDir()
	defer os.RemoveAll(newDataDir2)

	name2 := "infra2"
	initialCluster2 := epc.procs[0].Config().initialCluster + fmt.Sprintf(",%s=%s", name2, peerURL)

	t.Log("Starting the new member")
	// start the new member
	var nepc *expect.ExpectProcess
	nepc, err = spawnCmd([]string{epc.procs[0].Config().execPath, "--name", name2,
		"--listen-client-urls", clientURL, "--advertise-client-urls", clientURL,
		"--listen-peer-urls", peerURL, "--initial-advertise-peer-urls", peerURL,
		"--initial-cluster", initialCluster2, "--initial-cluster-state", "existing", "--data-dir", newDataDir2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = nepc.Expect("ready to serve client requests"); err != nil {
		t.Fatal(err)
	}

	prefixArgs = []string{ctlBinPath, "--endpoints", clientURL, "--dial-timeout", dialTimeout.String()}

	t.Log("Ensuring added member has data from incoming snapshot...")
	for i := range kvs {
		if err = spawnWithExpect(append(prefixArgs, "get", kvs[i].key), kvs[i].val); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("Stopping the second member")
	if err = nepc.Stop(); err != nil {
		t.Fatal(err)
	}
	t.Log("Test logic done")
}
