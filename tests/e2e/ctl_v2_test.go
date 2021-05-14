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
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func BeforeTestV2(t testing.TB) {
	BeforeTest(t)
	os.Setenv("ETCDCTL_API", "2")
	t.Cleanup(func() {
		os.Unsetenv("ETCDCTL_API")
	})
}

func TestCtlV2Set(t *testing.T)          { testCtlV2Set(t, newConfigNoTLS(), false) }
func TestCtlV2SetQuorum(t *testing.T)    { testCtlV2Set(t, newConfigNoTLS(), true) }
func TestCtlV2SetClientTLS(t *testing.T) { testCtlV2Set(t, newConfigClientTLS(), false) }
func TestCtlV2SetPeerTLS(t *testing.T)   { testCtlV2Set(t, newConfigPeerTLS(), false) }
func TestCtlV2SetTLS(t *testing.T)       { testCtlV2Set(t, newConfigTLS(), false) }
func testCtlV2Set(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.enableV2 = true
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

func TestCtlV2Mk(t *testing.T)       { testCtlV2Mk(t, newConfigNoTLS(), false) }
func TestCtlV2MkQuorum(t *testing.T) { testCtlV2Mk(t, newConfigNoTLS(), true) }
func TestCtlV2MkTLS(t *testing.T)    { testCtlV2Mk(t, newConfigTLS(), false) }
func testCtlV2Mk(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.enableV2 = true
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

func TestCtlV2Rm(t *testing.T)    { testCtlV2Rm(t, newConfigNoTLS()) }
func TestCtlV2RmTLS(t *testing.T) { testCtlV2Rm(t, newConfigTLS()) }
func testCtlV2Rm(t *testing.T, cfg *etcdProcessClusterConfig) {
	BeforeTestV2(t)

	cfg.enableV2 = true
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

func TestCtlV2Ls(t *testing.T)       { testCtlV2Ls(t, newConfigNoTLS(), false) }
func TestCtlV2LsQuorum(t *testing.T) { testCtlV2Ls(t, newConfigNoTLS(), true) }
func TestCtlV2LsTLS(t *testing.T)    { testCtlV2Ls(t, newConfigTLS(), false) }
func testCtlV2Ls(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) {
	BeforeTestV2(t)

	cfg.enableV2 = true
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

func TestCtlV2Watch(t *testing.T)    { testCtlV2Watch(t, newConfigNoTLS(), false) }
func TestCtlV2WatchTLS(t *testing.T) { testCtlV2Watch(t, newConfigTLS(), false) }

func testCtlV2Watch(t *testing.T, cfg *etcdProcessClusterConfig, noSync bool) {
	BeforeTestV2(t)

	cfg.enableV2 = true
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

	copied := newConfigNoTLS()
	copied.enableV2 = true
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
	if err := spawnWithExpect(regrantArgs, "duplicate"); err != nil {
		t.Fatalf("missing duplicate error on double grant role (%v)", err)
	}
}

func TestCtlV2UserListUsername(t *testing.T) { testCtlV2UserList(t, "username") }
func TestCtlV2UserListRoot(t *testing.T)     { testCtlV2UserList(t, "root") }
func testCtlV2UserList(t *testing.T, username string) {
	BeforeTestV2(t)

	copied := newConfigNoTLS()
	copied.enableV2 = true
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

	copied := newConfigNoTLS()
	copied.enableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer cleanupEtcdProcessCluster(epc, t)

	if err := etcdctlRoleAdd(epc, "foo"); err != nil {
		t.Fatalf("failed to add role (%v)", err)
	}
	if err := etcdctlRoleList(epc, "foo"); err != nil {
		t.Fatalf("failed to list roles (%v)", err)
	}
}

func TestCtlV2Backup(t *testing.T)         { testCtlV2Backup(t, 0, false) }
func TestCtlV2BackupSnapshot(t *testing.T) { testCtlV2Backup(t, 1, false) }

func TestCtlV2BackupV3(t *testing.T)         { testCtlV2Backup(t, 0, true) }
func TestCtlV2BackupV3Snapshot(t *testing.T) { testCtlV2Backup(t, 1, true) }

func testCtlV2Backup(t *testing.T, snapCount int, v3 bool) {
	BeforeTestV2(t)

	backupDir, err := ioutil.TempDir(t.TempDir(), "testbackup0.etcd")
	if err != nil {
		t.Fatal(err)
	}

	etcdCfg := newConfigNoTLS()
	etcdCfg.snapshotCount = snapCount
	etcdCfg.enableV2 = true
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
	if err := etcdctlBackup(t, epc1, epc1.procs[0].Config().dataDirPath, backupDir, v3); err != nil {
		t.Fatal(err)
	}
	t.Log("Closing etcd-1 backup")
	if err := epc1.Close(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}

	t.Logf("Backup directory: %s", backupDir)

	t.Log("Starting etcd-2 (post backup)")
	// restart from the backup directory
	cfg2 := newConfigNoTLS()
	cfg2.dataDirPath = backupDir
	cfg2.keepDataDir = true
	cfg2.forceNewCluster = true
	cfg2.enableV2 = true
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

	copiedCfg := newConfigClientTLS()
	copiedCfg.clientCertAuthEnabled = true
	copiedCfg.enableV2 = true
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

	copied := newConfigNoTLS()
	copied.enableV2 = true
	epc := setupEtcdctlTest(t, copied, true)
	defer cleanupEtcdProcessCluster(epc, t)

	// all members available
	if err := etcdctlClusterHealth(epc, "cluster is healthy"); err != nil {
		t.Fatalf("cluster-health expected to be healthy (%v)", err)
	}

	// missing members, has quorum
	epc.procs[0].Stop()

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
	epc.procs[1].Stop()
	if err := etcdctlClusterHealth(epc, "cluster is unavailable"); err != nil {
		t.Fatalf("cluster-health expected to be unavailable (%v)", err)
	}

	epc.procs[0], epc.procs[1] = nil, nil
}

func etcdctlPrefixArgs(clus *etcdProcessCluster) []string {
	endpoints := strings.Join(clus.EndpointsV2(), ",")
	cmdArgs := []string{ctlBinPath, "--endpoints", endpoints}
	if clus.cfg.clientTLS == clientTLS {
		cmdArgs = append(cmdArgs, "--ca-file", caPath, "--cert-file", certPath, "--key-file", privateKeyPath)
	}
	return cmdArgs
}

func etcdctlClusterHealth(clus *etcdProcessCluster, val string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "cluster-health")
	return spawnWithExpect(cmdArgs, val)
}

func etcdctlSet(clus *etcdProcessCluster, key, value string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "set", key, value)
	return spawnWithExpect(cmdArgs, value)
}

func etcdctlMk(clus *etcdProcessCluster, key, value string, first bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "mk", key, value)
	if first {
		return spawnWithExpect(cmdArgs, value)
	}
	return spawnWithExpect(cmdArgs, "Error:  105: Key already exists")
}

func etcdctlGet(clus *etcdProcessCluster, key, value string, quorum bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "get", key)
	if quorum {
		cmdArgs = append(cmdArgs, "--quorum")
	}
	return spawnWithExpect(cmdArgs, value)
}

func etcdctlRm(clus *etcdProcessCluster, key, value string, first bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "rm", key)
	if first {
		return spawnWithExpect(cmdArgs, "PrevNode.Value: "+value)
	}
	return spawnWithExpect(cmdArgs, "Error:  100: Key not found")
}

func etcdctlLs(clus *etcdProcessCluster, key string, quorum bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "ls")
	if quorum {
		cmdArgs = append(cmdArgs, "--quorum")
	}
	return spawnWithExpect(cmdArgs, key)
}

func etcdctlWatch(clus *etcdProcessCluster, key, value string, noSync bool) <-chan error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "watch", "--after-index=1", key)
	if noSync {
		cmdArgs = append(cmdArgs, "--no-sync")
	}
	errc := make(chan error, 1)
	go func() {
		errc <- spawnWithExpect(cmdArgs, value)
	}()
	return errc
}

func etcdctlRoleAdd(clus *etcdProcessCluster, role string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "add", role)
	return spawnWithExpect(cmdArgs, role)
}

func etcdctlRoleGrant(clus *etcdProcessCluster, role string, perms ...string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "grant")
	cmdArgs = append(cmdArgs, perms...)
	cmdArgs = append(cmdArgs, role)
	return spawnWithExpect(cmdArgs, role)
}

func etcdctlRoleList(clus *etcdProcessCluster, expectedRole string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "role", "list")
	return spawnWithExpect(cmdArgs, expectedRole)
}

func etcdctlUserAdd(clus *etcdProcessCluster, user, pass string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "add", user+":"+pass)
	return spawnWithExpect(cmdArgs, "User "+user+" created")
}

func etcdctlUserGrant(clus *etcdProcessCluster, user, role string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "grant", "--roles", role, user)
	return spawnWithExpect(cmdArgs, "User "+user+" updated")
}

func etcdctlUserGet(clus *etcdProcessCluster, user string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "get", user)
	return spawnWithExpect(cmdArgs, "User: "+user)
}

func etcdctlUserList(clus *etcdProcessCluster, expectedUser string) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "user", "list")
	return spawnWithExpect(cmdArgs, expectedUser)
}

func etcdctlAuthEnable(clus *etcdProcessCluster) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "auth", "enable")
	return spawnWithExpect(cmdArgs, "Authentication Enabled")
}

func etcdctlBackup(t testing.TB, clus *etcdProcessCluster, dataDir, backupDir string, v3 bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "backup", "--data-dir", dataDir, "--backup-dir", backupDir)
	if v3 {
		cmdArgs = append(cmdArgs, "--with-v3")
	}
	t.Logf("Running: %v", cmdArgs)
	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}
	err = proc.Close()
	if err != nil {
		return err
	}
	return proc.ProcessError()
}

func setupEtcdctlTest(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) *etcdProcessCluster {
	if !quorum {
		cfg = configStandalone(*cfg)
	}
	epc, err := newEtcdProcessCluster(t, cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}

func cleanupEtcdProcessCluster(epc *etcdProcessCluster, t *testing.T) {
	if errC := epc.Close(); errC != nil {
		t.Fatalf("error closing etcd processes (%v)", errC)
	}
}
