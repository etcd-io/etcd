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
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.uber.org/zap/zaptest"
)

func createV2store(t testing.TB, lastReleaseBinary string, dataDirPath string) {
	t.Log("Creating not-yet v2-deprecated etcd")

	cfg := e2e.ConfigStandalone(e2e.EtcdProcessClusterConfig{ExecPath: lastReleaseBinary, EnableV2: true, DataDirPath: dataDirPath, SnapshotCount: 5})
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, epc.Stop())
	}()

	// We need to exceed 'SnapshotCount' such that v2 snapshot is dumped.
	for i := 0; i < 10; i++ {
		if err := e2e.CURLPut(epc, e2e.CURLReq{
			Endpoint: "/v2/keys/foo", Value: "bar" + fmt.Sprint(i),
			Expected: `{"action":"set","node":{"key":"/foo","value":"bar` + fmt.Sprint(i)}); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}
	}
}

func assertVerifyCannotStartV2deprecationWriteOnly(t testing.TB, dataDirPath string) {
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=write-only mode")
	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--v2-deprecation=write-only", "--data-dir=" + dataDirPath}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect("detected disallowed custom content in v2store for stage --v2-deprecation=write-only")
	assert.NoError(t, err)
}

func assertVerifyCannotStartV2deprecationNotYet(t testing.TB, dataDirPath string) {
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=not-yet mode")
	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--v2-deprecation=not-yet", "--data-dir=" + dataDirPath}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect(`invalid value "not-yet" for flag -v2-deprecation: invalid value "not-yet"`)
	assert.NoError(t, err)
}

func TestV2DeprecationFlags(t *testing.T) {
	e2e.BeforeTest(t)
	dataDirPath := t.TempDir()

	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	t.Run("create-storev2-data", func(t *testing.T) {
		createV2store(t, lastReleaseBinary, dataDirPath)
	})

	t.Run("--v2-deprecation=not-yet fails", func(t *testing.T) {
		assertVerifyCannotStartV2deprecationNotYet(t, dataDirPath)
	})

	t.Run("--v2-deprecation=write-only fails", func(t *testing.T) {
		assertVerifyCannotStartV2deprecationWriteOnly(t, dataDirPath)
	})

}

func TestV2DeprecationSnapshotMatches(t *testing.T) {
	e2e.BeforeTest(t)
	lastReleaseData := t.TempDir()
	currentReleaseData := t.TempDir()

	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	currentReleaseBinary := e2e.BinDir + "/etcd"

	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}
	snapshotCount := 10
	epc := runEtcdAndCreateSnapshot(t, lastReleaseBinary, lastReleaseData, snapshotCount)
	members1 := addAndRemoveKeysAndMembers(t, e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3()), snapshotCount)
	assert.NoError(t, epc.Close())
	epc = runEtcdAndCreateSnapshot(t, currentReleaseBinary, currentReleaseData, snapshotCount)
	members2 := addAndRemoveKeysAndMembers(t, e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3()), snapshotCount)
	assert.NoError(t, epc.Close())

	assertSnapshotsMatch(t, lastReleaseData, currentReleaseData, func(data []byte) []byte {
		// Patch cluster version
		data = bytes.Replace(data, []byte("3.5.0"), []byte("X.X.X"), -1)
		data = bytes.Replace(data, []byte("3.6.0"), []byte("X.X.X"), -1)
		// Patch members ids
		for i, mid := range members1 {
			data = bytes.Replace(data, []byte(fmt.Sprintf("%x", mid)), []byte(fmt.Sprintf("member%d", i+1)), -1)
		}
		for i, mid := range members2 {
			data = bytes.Replace(data, []byte(fmt.Sprintf("%x", mid)), []byte(fmt.Sprintf("member%d", i+1)), -1)
		}
		return data
	})
}

func TestV2DeprecationSnapshotRecover(t *testing.T) {
	e2e.BeforeTest(t)
	dataDir := t.TempDir()

	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	currentReleaseBinary := e2e.BinDir + "/etcd"

	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}
	epc := runEtcdAndCreateSnapshot(t, lastReleaseBinary, dataDir, 10)

	cc := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())

	lastReleaseGetResponse, err := cc.Get("", config.GetOptions{Prefix: true})
	assert.NoError(t, err)

	lastReleaseMemberListResponse, err := cc.MemberList()
	assert.NoError(t, err)

	assert.NoError(t, epc.Close())
	cfg := e2e.ConfigStandalone(e2e.EtcdProcessClusterConfig{ExecPath: currentReleaseBinary, DataDirPath: dataDir})
	epc, err = e2e.NewEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)

	cc = e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())
	currentReleaseGetResponse, err := cc.Get("", config.GetOptions{Prefix: true})
	assert.NoError(t, err)

	currentReleaseMemberListResponse, err := cc.MemberList()
	assert.NoError(t, err)

	assert.Equal(t, lastReleaseGetResponse.Kvs, currentReleaseGetResponse.Kvs)
	assert.Equal(t, lastReleaseMemberListResponse.Members, currentReleaseMemberListResponse.Members)
	assert.NoError(t, epc.Close())
}

func runEtcdAndCreateSnapshot(t testing.TB, binary, dataDir string, snapshotCount int) *e2e.EtcdProcessCluster {
	cfg := e2e.ConfigStandalone(e2e.EtcdProcessClusterConfig{ExecPath: binary, DataDirPath: dataDir, SnapshotCount: snapshotCount, KeepDataDir: true})
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)
	return epc
}

func addAndRemoveKeysAndMembers(t testing.TB, cc *e2e.EtcdctlV3, snapshotCount int) (members []uint64) {
	// Execute some non-trivial key&member operation
	for i := 0; i < snapshotCount*3; i++ {
		err := cc.Put(fmt.Sprintf("%d", i), "1", config.PutOptions{})
		assert.NoError(t, err)
	}
	member1, err := cc.MemberAddAsLearner("member1", []string{"http://127.0.0.1:2000"})
	assert.NoError(t, err)
	members = append(members, member1.Member.ID)

	for i := 0; i < snapshotCount*2; i++ {
		_, err = cc.Delete(fmt.Sprintf("%d", i), config.DeleteOptions{})
		assert.NoError(t, err)
	}
	_, err = cc.MemberRemove(member1.Member.ID)
	assert.NoError(t, err)

	for i := 0; i < snapshotCount; i++ {
		err = cc.Put(fmt.Sprintf("%d", i), "2", config.PutOptions{})
		assert.NoError(t, err)
	}
	member2, err := cc.MemberAddAsLearner("member2", []string{"http://127.0.0.1:2001"})
	assert.NoError(t, err)
	members = append(members, member2.Member.ID)

	for i := 0; i < snapshotCount/2; i++ {
		err = cc.Put(fmt.Sprintf("%d", i), "3", config.PutOptions{})
		assert.NoError(t, err)
	}
	return members
}

func filterSnapshotFiles(path string) bool {
	return strings.HasSuffix(path, ".snap")
}

func assertSnapshotsMatch(t testing.TB, firstDataDir, secondDataDir string, patch func([]byte) []byte) {
	lg := zaptest.NewLogger(t)
	firstFiles, err := fileutil.ListFiles(firstDataDir, filterSnapshotFiles)
	if err != nil {
		t.Fatal(err)
	}
	secondFiles, err := fileutil.ListFiles(secondDataDir, filterSnapshotFiles)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, firstFiles)
	assert.NotEmpty(t, secondFiles)
	assert.Equal(t, len(firstFiles), len(secondFiles))
	sort.Strings(firstFiles)
	sort.Strings(secondFiles)
	for i := 0; i < len(firstFiles); i++ {
		firstSnapshot, err := snap.Read(lg, firstFiles[i])
		if err != nil {
			t.Fatal(err)
		}
		secondSnapshot, err := snap.Read(lg, secondFiles[i])
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, openSnap(patch(firstSnapshot.Data)), openSnap(patch(secondSnapshot.Data)))
	}
}

func openSnap(data []byte) v2store.Store {
	st := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
	st.Recovery(data)
	return st
}
