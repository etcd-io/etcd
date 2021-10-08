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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func BeforeTestV2(t testing.TB) {
	e2e.BeforeTest(t)
	os.Setenv("ETCDCTL_API", "2")
	t.Cleanup(func() {
		os.Unsetenv("ETCDCTL_API")
	})
}

func TestCtlV2Set(t *testing.T)          { testCtlV2Set(t, e2e.NewConfigNoTLS(), false) }
func TestCtlV2SetQuorum(t *testing.T)    { testCtlV2Set(t, e2e.NewConfigNoTLS(), true) }
func TestCtlV2SetClientTLS(t *testing.T) { testCtlV2Set(t, e2e.NewConfigClientTLS(), false) }
func TestCtlV2SetPeerTLS(t *testing.T)   { testCtlV2Set(t, e2e.NewConfigPeerTLS(), false) }
func TestCtlV2SetTLS(t *testing.T)       { testCtlV2Set(t, e2e.NewConfigTLS(), false) }
func testCtlV2Set(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.EnableV2 = true
	epc := setupEtcdctlTest(t, cfg, quorum)
	defer cleanupEtcdProcessCluster(epc, t)

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlGet(epc, key, value, quorum); err != nil {
		t.Fatalf("failed get (%v)", err)
	}
}

func TestCtlV2Mk(t *testing.T)       { testCtlV2Mk(t, e2e.NewConfigNoTLS(), false) }
func TestCtlV2MkQuorum(t *testing.T) { testCtlV2Mk(t, e2e.NewConfigNoTLS(), true) }
func TestCtlV2MkTLS(t *testing.T)    { testCtlV2Mk(t, e2e.NewConfigTLS(), false) }
func testCtlV2Mk(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.EnableV2 = true
	epc := setupEtcdctlTest(t, cfg, quorum)
	defer cleanupEtcdProcessCluster(epc, t)

	key, value := "foo", "bar"

	if err := etcdctlMk(epc, key, value, true); err != nil {
		t.Fatalf("failed mk (%v)", err)
	}
	if err := etcdctlMk(epc, key, value, false); err != nil {
		t.Fatalf("failed mk (%v)", err)
	}

	if err := etcdctlGet(epc, key, value, quorum); err != nil {
		t.Fatalf("failed get (%v)", err)
	}
}

func TestCtlV2Rm(t *testing.T)    { testCtlV2Rm(t, e2e.NewConfigNoTLS()) }
func TestCtlV2RmTLS(t *testing.T) { testCtlV2Rm(t, e2e.NewConfigTLS()) }
func testCtlV2Rm(t *testing.T, cfg *e2e.EtcdProcessClusterConfig) {
	BeforeTestV2(t)

	cfg.EnableV2 = true
	epc := setupEtcdctlTest(t, cfg, true)
	defer cleanupEtcdProcessCluster(epc, t)

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlRm(epc, key, value, true); err != nil {
		t.Fatalf("failed rm (%v)", err)
	}
	if err := etcdctlRm(epc, key, value, false); err != nil {
		t.Fatalf("failed rm (%v)", err)
	}
}

func TestCtlV2Ls(t *testing.T)       { testCtlV2Ls(t, e2e.NewConfigNoTLS(), false) }
func TestCtlV2LsQuorum(t *testing.T) { testCtlV2Ls(t, e2e.NewConfigNoTLS(), true) }
func TestCtlV2LsTLS(t *testing.T)    { testCtlV2Ls(t, e2e.NewConfigTLS(), false) }
func testCtlV2Ls(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.EnableV2 = true
	epc := setupEtcdctlTest(t, cfg, quorum)
	defer cleanupEtcdProcessCluster(epc, t)

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlLs(epc, key, quorum); err != nil {
		t.Fatalf("failed ls (%v)", err)
	}
}

func TestCtlV2Watch(t *testing.T)    { testCtlV2Watch(t, e2e.NewConfigNoTLS(), false) }
func TestCtlV2WatchTLS(t *testing.T) { testCtlV2Watch(t, e2e.NewConfigTLS(), false) }

func testCtlV2Watch(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, noSync bool) {
	BeforeTestV2(t)

	cfg.EnableV2 = true
	epc := setupEtcdctlTest(t, cfg, true)
	defer cleanupEtcdProcessCluster(epc, t)

	key, value := "foo", "bar"
	errc := etcdctlWatch(epc, key, value, noSync)
	if err := etcdctlSet(epc, key, value); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("failed watch (%v)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("watch timed out")
	}
}

func TestCtlV2GetRoleUser(t *testing.T) {
	BeforeTestV2(t)

	copied := e2e.NewConfigNoTLS()
	copied.EnableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer cleanupEtcdProcessCluster(epc, t)

	if err := etcdctlRoleAdd(epc, "foo"); err != nil {
		t.Fatalf("failed to add role (%v)", err)
	}
	if err := etcdctlUserAdd(epc, "username", "password"); err != nil {
		t.Fatalf("failed to add user (%v)", err)
	}
	if err := etcdctlUserGrant(epc, "username", "foo"); err != nil {
		t.Fatalf("failed to grant role (%v)", err)
	}
	if err := etcdctlUserGet(epc, "username"); err != nil {
		t.Fatalf("failed to get user (%v)", err)
	}

	// ensure double grant gives an error; was crashing in 2.3.1
	regrantArgs := etcdctlPrefixArgs(epc)
	regrantArgs = append(regrantArgs, "user", "grant", "--roles", "foo", "username")
	if err := e2e.SpawnWithExpect(regrantArgs, "duplicate"); err != nil {
		t.Fatalf("missing duplicate error on double grant role (%v)", err)
	}
}

func TestCtlV2UserListUsername(t *testing.T) { testCtlV2UserList(t, "username") }
func TestCtlV2UserListRoot(t *testing.T)     { testCtlV2UserList(t, "root") }
func testCtlV2UserList(t *testing.T, username string) {
	BeforeTestV2(t)

	copied := e2e.NewConfigNoTLS()
	copied.EnableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer cleanupEtcdProcessCluster(epc, t)

	if err := etcdctlUserAdd(epc, username, "password"); err != nil {
		t.Fatalf("failed to add user (%v)", err)
	}
	if err := etcdctlUserList(epc, username); err != nil {
		t.Fatalf("failed to list users (%v)", err)
	}
}

func TestCtlV2RoleList(t *testing.T) {
	BeforeTestV2(t)

	copied := e2e.NewConfigNoTLS()
	copied.EnableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer cleanupEtcdProcessCluster(epc, t)

	if err := etcdctlRoleAdd(epc, "foo"); err != nil {
		t.Fatalf("failed to add role (%v)", err)
	}
	if err := etcdctlRoleList(epc, "foo"); err != nil {
		t.Fatalf("failed to list roles (%v)", err)
	}
}

func TestUtlCtlV2Backup(t *testing.T) {
	for snap := range []int{0, 1} {
		for _, v3 := range []bool{true, false} {
			for _, utl := range []bool{true, false} {
				t.Run(fmt.Sprintf("etcdutl:%v;snap:%v;v3:%v", utl, snap, v3),
					func(t *testing.T) {
						testUtlCtlV2Backup(t, snap, v3, utl)
					})
			}
		}
	}
}

func testUtlCtlV2Backup(t *testing.T, snapCount int, v3 bool, utl bool) {
	BeforeTestV2(t)

	backupDir, err := ioutil.TempDir(t.TempDir(), "testbackup0.etcd")
	if err != nil {
		t.Fatal(err)
	}

	etcdCfg := e2e.NewConfigNoTLS()
	etcdCfg.SnapshotCount = snapCount
	etcdCfg.EnableV2 = true
	t.Log("Starting etcd-1")
	epc1 := setupEtcdctlTest(t, etcdCfg, false)

	// v3 put before v2 set so snapshot happens after v3 operations to confirm
	// v3 data is preserved after snapshot.
	os.Setenv("ETCDCTL_API", "3")
	if err := ctlV3Put(ctlCtx{t: t, epc: epc1}, "v3key", "123", ""); err != nil {
		t.Fatal(err)
	}
	os.Setenv("ETCDCTL_API", "2")

	t.Log("Setting key in etcd-1")
	if err := etcdctlSet(epc1, "foo1", "bar1"); err != nil {
		t.Fatal(err)
	}

	if v3 {
		t.Log("Stopping etcd-1")
		// v3 must lock the db to backup, so stop process
		if err := epc1.Stop(); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("Triggering etcd backup")
	if err := etcdctlBackup(t, epc1, epc1.Procs[0].Config().DataDirPath, backupDir, v3, utl); err != nil {
		t.Fatal(err)
	}
	t.Log("Closing etcd-1 backup")
	if err := epc1.Close(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}

	t.Logf("Backup directory: %s", backupDir)

	t.Log("Starting etcd-2 (post backup)")
	// restart from the backup directory
	cfg2 := e2e.NewConfigNoTLS()
	cfg2.DataDirPath = backupDir
	cfg2.KeepDataDir = true
	cfg2.ForceNewCluster = true
	cfg2.EnableV2 = true
	epc2 := setupEtcdctlTest(t, cfg2, false)
	// Make sure a failing test is not leaking resources (running server).
	defer epc2.Close()

	t.Log("Getting examplar key")
	// check if backup went through correctly
	if err := etcdctlGet(epc2, "foo1", "bar1", false); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ETCDCTL_API", "3")
	ctx2 := ctlCtx{t: t, epc: epc2}
	if v3 {
		t.Log("Getting v3 examplar key")
		if err := ctlV3Get(ctx2, []string{"v3key"}, kv{"v3key", "123"}); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := ctlV3Get(ctx2, []string{"v3key"}); err != nil {
			t.Fatal(err)
		}
	}
	os.Setenv("ETCDCTL_API", "2")

	t.Log("Getting examplar key foo2")
	// check if it can serve client requests
	if err := etcdctlSet(epc2, "foo2", "bar2"); err != nil {
		t.Fatal(err)
	}
	if err := etcdctlGet(epc2, "foo2", "bar2", false); err != nil {
		t.Fatal(err)
	}

	t.Log("Closing etcd-2")
	if err := epc2.Close(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}
}

func TestCtlV2AuthWithCommonName(t *testing.T) {
	BeforeTestV2(t)

	copiedCfg := e2e.NewConfigClientTLS()
	copiedCfg.ClientCertAuthEnabled = true
	copiedCfg.EnableV2 = true
	epc := setupEtcdctlTest(t, copiedCfg, false)
	defer cleanupEtcdProcessCluster(epc, t)

	if err := etcdctlRoleAdd(epc, "testrole"); err != nil {
		t.Fatalf("failed to add role (%v)", err)
	}
	if err := etcdctlRoleGrant(epc, "testrole", "--rw", "--path=/foo"); err != nil {
		t.Fatalf("failed to grant role (%v)", err)
	}
	if err := etcdctlUserAdd(epc, "root", "123"); err != nil {
		t.Fatalf("failed to add user (%v)", err)
	}
	if err := etcdctlUserAdd(epc, "Autogenerated CA", "123"); err != nil {
		t.Fatalf("failed to add user (%v)", err)
	}
	if err := etcdctlUserGrant(epc, "Autogenerated CA", "testrole"); err != nil {
		t.Fatalf("failed to grant role (%v)", err)
	}
	if err := etcdctlAuthEnable(epc); err != nil {
		t.Fatalf("failed to enable auth (%v)", err)
	}
	if err := etcdctlSet(epc, "foo", "bar"); err != nil {
		t.Fatalf("failed to write (%v)", err)
	}
}

func TestCtlV2ClusterHealth(t *testing.T) {
	BeforeTestV2(t)

	copied := e2e.NewConfigNoTLS()
	copied.EnableV2 = true
	epc := setupEtcdctlTest(t, copied, true)
	defer cleanupEtcdProcessCluster(epc, t)

	// all members available
	if err := etcdctlClusterHealth(epc, "cluster is healthy"); err != nil {
		t.Fatalf("cluster-health expected to be healthy (%v)", err)
	}

	// missing members, has quorum
	epc.Procs[0].Stop()

	for i := 0; i < 3; i++ {
		err := etcdctlClusterHealth(epc, "cluster is degraded")
		if err == nil {
			break
		} else if i == 2 {
			t.Fatalf("cluster-health expected to be degraded (%v)", err)
		}
		// possibly no leader yet; retry
		time.Sleep(time.Second)
	}

	// no quorum
	epc.Procs[1].Stop()
	if err := etcdctlClusterHealth(epc, "cluster is unavailable"); err != nil {
		t.Fatalf("cluster-health expected to be unavailable (%v)", err)
	}

	epc.Procs[0], epc.Procs[1] = nil, nil
}

func etcdctlPrefixArgs(clus *e2e.EtcdProcessCluster) []string {
	endpoints := strings.Join(clus.EndpointsV2(), ",")
	cmdArgs := []string{e2e.CtlBinPath}

	cmdArgs = append(cmdArgs, "--endpoints", endpoints)
	if clus.Cfg.ClientTLS == e2e.ClientTLS {
		cmdArgs = append(cmdArgs, "--ca-file", e2e.CaPath, "--cert-file", e2e.CertPath, "--key-file", e2e.PrivateKeyPath)
	}
	return cmdArgs
}

func etcductlPrefixArgs(utl bool) []string {
	if utl {
		return []string{e2e.UtlBinPath}
	}
	return []string{e2e.CtlBinPath}
}

func etcdctlClusterHealth(clus *e2e.EtcdProcessCluster, val string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "cluster-health")
	return e2e.SpawnWithExpect(cmdArgs, val)
}

func etcdctlSet(clus *e2e.EtcdProcessCluster, key, value string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "set", key, value)
	return e2e.SpawnWithExpect(cmdArgs, value)
}

func etcdctlMk(clus *e2e.EtcdProcessCluster, key, value string, first bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "mk", key, value)
	if first {
		return e2e.SpawnWithExpect(cmdArgs, value)
	}
	return e2e.SpawnWithExpect(cmdArgs, "Error:  105: Key already exists")
}

func etcdctlGet(clus *e2e.EtcdProcessCluster, key, value string, quorum bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "get", key)
	if quorum {
		cmdArgs = append(cmdArgs, "--quorum")
	}
	return e2e.SpawnWithExpect(cmdArgs, value)
}

func etcdctlRm(clus *e2e.EtcdProcessCluster, key, value string, first bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "rm", key)
	if first {
		return e2e.SpawnWithExpect(cmdArgs, "PrevNode.Value: "+value)
	}
	return e2e.SpawnWithExpect(cmdArgs, "Error:  100: Key not found")
}

func etcdctlLs(clus *e2e.EtcdProcessCluster, key string, quorum bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "ls")
	if quorum {
		cmdArgs = append(cmdArgs, "--quorum")
	}
	return e2e.SpawnWithExpect(cmdArgs, key)
}

func etcdctlWatch(clus *e2e.EtcdProcessCluster, key, value string, noSync bool) <-chan error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "watch", "--after-index=1", key)
	if noSync {
		cmdArgs = append(cmdArgs, "--no-sync")
	}
	errc := make(chan error, 1)
	go func() {
		errc <- e2e.SpawnWithExpect(cmdArgs, value)
	}()
	return errc
}

func etcdctlRoleAdd(clus *e2e.EtcdProcessCluster, role string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "add", role)
	return e2e.SpawnWithExpect(cmdArgs, role)
}

func etcdctlRoleGrant(clus *e2e.EtcdProcessCluster, role string, perms ...string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "grant")
	cmdArgs = append(cmdArgs, perms...)
	cmdArgs = append(cmdArgs, role)
	return e2e.SpawnWithExpect(cmdArgs, role)
}

func etcdctlRoleList(clus *e2e.EtcdProcessCluster, expectedRole string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "list")
	return e2e.SpawnWithExpect(cmdArgs, expectedRole)
}

func etcdctlUserAdd(clus *e2e.EtcdProcessCluster, user, pass string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "add", user+":"+pass)
	return e2e.SpawnWithExpect(cmdArgs, "User "+user+" created")
}

func etcdctlUserGrant(clus *e2e.EtcdProcessCluster, user, role string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "grant", "--roles", role, user)
	return e2e.SpawnWithExpect(cmdArgs, "User "+user+" updated")
}

func etcdctlUserGet(clus *e2e.EtcdProcessCluster, user string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "get", user)
	return e2e.SpawnWithExpect(cmdArgs, "User: "+user)
}

func etcdctlUserList(clus *e2e.EtcdProcessCluster, expectedUser string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "list")
	return e2e.SpawnWithExpect(cmdArgs, expectedUser)
}

func etcdctlAuthEnable(clus *e2e.EtcdProcessCluster) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "auth", "enable")
	return e2e.SpawnWithExpect(cmdArgs, "Authentication Enabled")
}

func etcdctlBackup(t testing.TB, clus *e2e.EtcdProcessCluster, dataDir, backupDir string, v3 bool, utl bool) error {
	cmdArgs := append(etcductlPrefixArgs(utl), "backup", "--data-dir", dataDir, "--backup-dir", backupDir)
	if v3 {
		cmdArgs = append(cmdArgs, "--with-v3")
	} else if utl {
		cmdArgs = append(cmdArgs, "--with-v3=false")
	}
	t.Logf("Running: %v", cmdArgs)
	proc, err := e2e.SpawnCmd(cmdArgs, nil)
	if err != nil {
		return err
	}
	err = proc.Close()
	if err != nil {
		return err
	}
	return proc.ProcessError()
}

func setupEtcdctlTest(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) *e2e.EtcdProcessCluster {
	if !quorum {
		cfg = e2e.ConfigStandalone(*cfg)
	}
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}

func cleanupEtcdProcessCluster(epc *e2e.EtcdProcessCluster, t *testing.T) {
	if errC := epc.Close(); errC != nil {
		t.Fatalf("error closing etcd processes (%v)", errC)
	}
}
