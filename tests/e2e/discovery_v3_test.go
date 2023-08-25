// Copyright 2022 The etcd Authors
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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestClusterOf1UsingV3Discovery_1endpoint(t *testing.T) {
	testClusterUsingV3Discovery(t, 1, 1, e2e.ClientNonTLS, false)
}
func TestClusterOf3UsingV3Discovery_1endpoint(t *testing.T) {
	testClusterUsingV3Discovery(t, 1, 3, e2e.ClientTLS, true)
}
func TestTLSClusterOf5UsingV3Discovery_1endpoint(t *testing.T) {
	testClusterUsingV3Discovery(t, 1, 5, e2e.ClientTLS, false)
}

func TestClusterOf1UsingV3Discovery_3endpoints(t *testing.T) {
	testClusterUsingV3Discovery(t, 3, 1, e2e.ClientNonTLS, false)
}
func TestClusterOf3UsingV3Discovery_3endpoints(t *testing.T) {
	testClusterUsingV3Discovery(t, 3, 3, e2e.ClientTLS, true)
}
func TestTLSClusterOf5UsingV3Discovery_3endpoints(t *testing.T) {
	testClusterUsingV3Discovery(t, 3, 5, e2e.ClientTLS, false)
}

func testClusterUsingV3Discovery(t *testing.T, discoveryClusterSize, targetClusterSize int, clientTlsType e2e.ClientConnType, isClientAutoTls bool) {
	e2e.BeforeTest(t)

	// step 1: start the discovery service
	ds, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithBasePort(2000),
		e2e.WithClusterSize(discoveryClusterSize),
		e2e.WithClientConnType(clientTlsType),
		e2e.WithClientAutoTLS(isClientAutoTls),
	)
	if err != nil {
		t.Fatalf("could not start discovery etcd cluster (%v)", err)
	}
	defer ds.Close()

	// step 2: configure the cluster size
	discoveryToken := "8A591FAB-1D72-41FA-BDF2-A27162FDA1E0"
	configSizeKey := fmt.Sprintf("/_etcd/registry/%s/_config/size", discoveryToken)
	configSizeValStr := strconv.Itoa(targetClusterSize)
	if err := ctlV3Put(ctlCtx{epc: ds}, configSizeKey, configSizeValStr, ""); err != nil {
		t.Errorf("failed to configure cluster size to discovery serivce, error: %v", err)
	}

	// step 3: start the etcd cluster
	epc, err := bootstrapEtcdClusterUsingV3Discovery(t, ds.EndpointsGRPC(), discoveryToken, targetClusterSize, clientTlsType, isClientAutoTls)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer epc.Close()

	// step 4: sanity test on the etcd cluster
	etcdctl := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ",")}
	if err := e2e.SpawnWithExpect(append(etcdctl, "put", "key", "value"), expect.ExpectedResponse{Value: "OK"}); err != nil {
		t.Fatal(err)
	}
	if err := e2e.SpawnWithExpect(append(etcdctl, "get", "key"), expect.ExpectedResponse{Value: "value"}); err != nil {
		t.Fatal(err)
	}
}

func bootstrapEtcdClusterUsingV3Discovery(t *testing.T, discoveryEndpoints []string, discoveryToken string, clusterSize int, clientTlsType e2e.ClientConnType, isClientAutoTls bool) (*e2e.EtcdProcessCluster, error) {
	// cluster configuration
	cfg := e2e.NewConfig(
		e2e.WithBasePort(3000),
		e2e.WithClusterSize(clusterSize),
		e2e.WithIsPeerTLS(true),
		e2e.WithIsPeerAutoTLS(true),
		e2e.WithDiscoveryToken(discoveryToken),
		e2e.WithDiscoveryEndpoints(discoveryEndpoints),
	)

	// initialize the cluster
	epc, err := e2e.InitEtcdProcessCluster(t, cfg)
	if err != nil {
		t.Fatalf("could not initialize etcd cluster (%v)", err)
		return epc, err
	}

	// populate discovery related security configuration
	for _, ep := range epc.Procs {
		epCfg := ep.Config()

		if clientTlsType == e2e.ClientTLS {
			if isClientAutoTls {
				epCfg.Args = append(epCfg.Args, "--discovery-insecure-transport=false")
				epCfg.Args = append(epCfg.Args, "--discovery-insecure-skip-tls-verify=true")
			} else {
				epCfg.Args = append(epCfg.Args, "--discovery-cacert="+e2e.CaPath)
				epCfg.Args = append(epCfg.Args, "--discovery-cert="+e2e.CertPath)
				epCfg.Args = append(epCfg.Args, "--discovery-key="+e2e.PrivateKeyPath)
			}
		}
	}

	// start the cluster
	return e2e.StartEtcdProcessCluster(context.TODO(), epc, cfg)
}
