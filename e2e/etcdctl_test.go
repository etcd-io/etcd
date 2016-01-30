// Copyright 2016 CoreOS, Inc.
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
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestCtlV2Set(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlSetGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		false,
	)
}

func TestCtlV2SetNoSync(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlSetGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		true,
	)
}

func etcdctlPrefixArgs(epc *etcdProcessCluster, noSync bool) []string {
	endpoint := ""
	if proxies := epc.proxies(); len(proxies) != 0 {
		endpoint = proxies[0].cfg.acurl.String()
	} else if backends := epc.backends(); len(backends) != 0 {
		endpoint = backends[0].cfg.acurl.String()
	}
	args := []string{"../bin/etcdctl", "--endpoint", endpoint}
	if noSync {
		args = append(args, "--no-sync")
	}
	return args
}

func etcdctlSet(epc *etcdProcessCluster, key, value string, noSync bool) error {
	args := append(etcdctlPrefixArgs(epc, noSync), "set", key, value)
	return spawnWithExpect(args, value)
}

func etcdctlGet(epc *etcdProcessCluster, key, value string, noSync bool) error {
	args := append(etcdctlPrefixArgs(epc, noSync), "get", key)
	return spawnWithExpect(args, value)
}

func testProcessClusterV2CtlSetGet(t *testing.T, cfg *etcdProcessClusterConfig, noSync bool) {
	if fileutil.Exist("../bin/etcdctl") == false {
		t.Fatalf("could not find etcdctl binary")
	}

	epc, errC := newEtcdProcessCluster(cfg)
	if errC != nil {
		t.Fatalf("could not start etcd process cluster (%v)", errC)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value, noSync); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlGet(epc, key, value, noSync); err != nil {
		t.Fatalf("failed set (%v)", err)
	}
}

func TestCtlV2Ls(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlLs(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		false,
	)
}

func TestCtlV2LsNoSync(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlLs(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		true,
	)
}

func etcdctlLs(epc *etcdProcessCluster, key string, noSync bool) error {
	args := append(etcdctlPrefixArgs(epc, noSync), "ls")
	return spawnWithExpect(args, key)
}

func testProcessClusterV2CtlLs(t *testing.T, cfg *etcdProcessClusterConfig, noSync bool) {
	if fileutil.Exist("../bin/etcdctl") == false {
		t.Fatalf("could not find etcdctl binary")
	}

	epc, errC := newEtcdProcessCluster(cfg)
	if errC != nil {
		t.Fatalf("could not start etcd process cluster (%v)", errC)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	key, value := "foo", "bar"

	if err := etcdctlSet(epc, key, value, noSync); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	if err := etcdctlLs(epc, key, noSync); err != nil {
		t.Fatalf("failed set (%v)", err)
	}
}

func TestCtlV2WatchWithProxy(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlWatch(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		false,
	)
}

func TestCtlV2WatchWithProxyNoSync(t *testing.T) {
	defer testutil.AfterTest(t)
	testProcessClusterV2CtlWatch(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			proxySize:    1,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
		true,
	)
}

func etcdctlWatch(epc *etcdProcessCluster, key, value string, noSync bool, done chan struct{}, errChan chan error) {
	args := append(etcdctlPrefixArgs(epc, noSync), "watch", key)
	if err := spawnWithExpect(args, value); err != nil {
		errChan <- err
		return
	}
	done <- struct{}{}
}

func testProcessClusterV2CtlWatch(t *testing.T, cfg *etcdProcessClusterConfig, noSync bool) {
	if fileutil.Exist("../bin/etcdctl") == false {
		t.Fatalf("could not find etcdctl binary")
	}

	epc, errC := newEtcdProcessCluster(cfg)
	if errC != nil {
		t.Fatalf("could not start etcd process cluster (%v)", errC)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	key, value := "foo", "bar"
	done, errChan := make(chan struct{}), make(chan error)

	go etcdctlWatch(epc, key, value, noSync, done, errChan)

	if err := etcdctlSet(epc, key, value, noSync); err != nil {
		t.Fatalf("failed set (%v)", err)
	}

	select {
	case <-done:
		return
	case err := <-errChan:
		t.Fatalf("failed watch (%v)", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("watch timed out!")
	}
}
