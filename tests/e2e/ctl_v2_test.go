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
	"os"
	"strings"
	"testing"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/testutil"
)

func testCtlV2Set(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) {
	os.Setenv("ETCDCTL_API", "2")
	defer os.Unsetenv("ETCDCTL_API")
	defer testutil.AfterTest(t)

	cfg.enableV2 = true
	epc := setupEtcdctlTest(t, cfg, quorum)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlGet(epc, key, value, quorum); err != nil {
		t.Fatalf("failed get (%v)", err)
	}
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

func etcdctlBackup(clus *etcdProcessCluster, dataDir, backupDir string, v3 bool) error {
	cmdArgs := append(etcdctlPrefixArgs(clus), "backup", "--data-dir", dataDir, "--backup-dir", backupDir)
	if v3 {
		cmdArgs = append(cmdArgs, "--with-v3")
	}
	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}
	return proc.Close()
}

func mustEtcdctl(t *testing.T) {
	if !fileutil.Exist(binDir + "/etcdctl") {
		t.Fatalf("could not find etcdctl binary")
	}
}

func setupEtcdctlTest(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) *etcdProcessCluster {
	mustEtcdctl(t)
	if !quorum {
		cfg = configStandalone(*cfg)
	}
	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}
