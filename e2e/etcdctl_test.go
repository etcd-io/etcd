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

func TestBasicOpsV2CtlWatchWithProxy(t *testing.T) {
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

func TestBasicOpsV2CtlWatchWithProxyNoSync(t *testing.T) {
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

func etcdctlSet(epc *etcdProcessCluster, key, value string, noSync bool) error {
	endpoint := ""
	if proxies := epc.proxies(); len(proxies) != 0 {
		endpoint = proxies[0].cfg.acurl.String()
	} else if backends := epc.backends(); len(backends) != 0 {
		endpoint = backends[0].cfg.acurl.String()
	}

	putArgs := []string{"../bin/etcdctl", "--endpoint", endpoint}
	if noSync {
		putArgs = append(putArgs, "--no-sync")
	}
	putArgs = append(putArgs, "set", key, value)

	return spawnWithExpect(putArgs, value)
}

func etcdctlWatch(epc *etcdProcessCluster, key, value string, noSync bool, done chan struct{}, errChan chan error) {
	endpoint := ""
	if proxies := epc.proxies(); len(proxies) != 0 {
		endpoint = proxies[0].cfg.acurl.String()
	} else if backends := epc.backends(); len(backends) != 0 {
		endpoint = backends[0].cfg.acurl.String()
	}

	watchArgs := []string{"../bin/etcdctl", "--endpoint", endpoint}
	if noSync {
		watchArgs = append(watchArgs, "--no-sync")
	}
	watchArgs = append(watchArgs, "watch", key)

	if err := spawnWithExpect(watchArgs, value); err != nil {
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
	case <-time.After(2 * time.Second):
		t.Fatalf("watch timed out!")
	}
}
