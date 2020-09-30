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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/clientv3/snapshot"
	"go.etcd.io/etcd/v3/pkg/expect"
	"go.etcd.io/etcd/v3/pkg/testutil"
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
