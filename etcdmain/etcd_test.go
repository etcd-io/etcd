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

package etcdmain

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/transport"
)

var (
	testClusterSize = 3
	electionPause   = 3 * time.Second
	stopTimeout     = 2 * time.Second

	certPath       = "testcerts/cert.pem"
	privateKeyPath = "testcerts/key.pem"
	caPath         = "testcerts/ca.perm"
	testTLSInfo    = transport.TLSInfo{
		CertFile: certPath,
		KeyFile:  privateKeyPath,
		CAFile:   caPath,
	}
)

func Test_startEtcd_v2(t *testing.T) {
	isClientTLS, isPeerTLS := false, false
	v2test(t, isClientTLS, isPeerTLS)
}

func Test_startEtcd_v2_TLS(t *testing.T) {
	isClientTLS, isPeerTLS := true, true
	v2test(t, isClientTLS, isPeerTLS)
}

func Test_startEtcd_v2_TLS_client_to_server(t *testing.T) {
	isClientTLS, isPeerTLS := true, false
	v2test(t, isClientTLS, isPeerTLS)
}

func Test_startEtcd_v2_TLS_server_to_server(t *testing.T) {
	isClientTLS, isPeerTLS := false, true
	v2test(t, isClientTLS, isPeerTLS)
}

// v2test sets up N number of members in a cluster, and tests simple curl client
// operations and EtcdServer.Stop methods.
func v2test(t *testing.T, isClientTLS, isPeerTLS bool) {
	configs, errE := createTestConfigs(testClusterSize, isClientTLS, isPeerTLS)
	if errE != nil {
		t.Error(errE)
	}

	// start the cluster
	stops := make([]<-chan struct{}, testClusterSize)
	for i := range configs {
		cfg := configs[i]
		defer os.RemoveAll(cfg.dir)
		stopped, err := startEtcd(cfg)
		if err != nil {
			t.Error(err)
		}
		stops[i] = stopped
	}

	time.Sleep(electionPause)

	// curl client endpoint: PUT, GET
	puts, gets := []string{}, []string{}
	if !isClientTLS {
		puts = []string{"curl", "-L", configs[0].acurls[0].String() + "/v2/keys/testKey", "-XPUT", "-d", `value="this is awesome"`}
		gets = []string{"curl", "-L", configs[0].acurls[0].String() + "/v2/keys/testKey"}
	} else {
		// give the CA signed client certificate to the server
		puts = []string{"curl", "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath, "-L", configs[0].acurls[0].String() + "/v2/keys/testKey", "-XPUT", "-d", `value="this is awesome"`}
		gets = []string{"curl", "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath, "-L", configs[0].acurls[0].String() + "/v2/keys/testKey"}
	}
	if _, err := exec.Command(puts[0], puts[1:]...).Output(); err != nil {
		t.Error(err)
	}
	outGet, err := exec.Command(gets[0], gets[1:]...).Output()
	if err != nil {
		t.Error(err)
	}
	if !strings.HasPrefix(string(outGet), `{"action":"get","node":{"key":"/testKey","value":"\"this is awesome\"","`) {
		t.Errorf("unexpected GET output: %s", outGet)
	}

	for i := range configs {
		cfg := configs[i]
		stopped := stops[i]

		log.Println("stopping", cfg.name)
		cfg.etcdServer.Stop()

		select {
		case <-time.After(stopTimeout):
			t.Errorf("timeout %s", cfg.name)
		case <-stopped:
			log.Println("stopped", cfg.name)
		}
	}
}

func getFreePorts(num int) ([]string, error) {
	pmap := make(map[string]struct{})
	for len(pmap) != num {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		pd := l.Addr().(*net.TCPAddr).Port
		l.Close()
		pmap[":"+strconv.Itoa(pd)] = struct{}{}
	}
	ps := []string{}
	for p := range pmap {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	return ps, nil
}

// createTestConfigs creates test configs with minimum configurations.
// This does not set up TLS.
func createTestConfigs(clusterSize int, isClientTLS, isPeerTLS bool) ([]*config, error) {
	ports, err := getFreePorts(2 * testClusterSize)
	if err != nil {
		return nil, err
	}
	clientScheme := "http"
	if isClientTLS {
		clientScheme = "https"
	}
	peerScheme := "http"
	if isPeerTLS {
		peerScheme = "https"
	}

	configs := make([]*config, clusterSize)
	initialClusterPairs := make([][]string, clusterSize)

	for i := 0; i < clusterSize; i++ {
		curls := []url.URL{{Scheme: clientScheme, Host: "localhost" + ports[2*i]}}
		purls := []url.URL{{Scheme: peerScheme, Host: "localhost" + ports[2*i+1]}}
		name := fmt.Sprintf("testname%d", i)

		cfg := NewConfig()
		cfg.name = name
		cfg.lcurls = curls
		cfg.acurls = curls
		cfg.lpurls = purls
		cfg.apurls = purls
		cfg.initialClusterToken = "new"
		cfg.dir = name + ".etcd"

		configs[i] = cfg
		initialClusterPairs[i] = []string{name, peerScheme + "://localhost" + ports[2*i+1]}
	}
	initialClusterStr := ""
	for _, p := range initialClusterPairs {
		prefix := ""
		if initialClusterStr != "" {
			prefix = ","
		}
		initialClusterStr += fmt.Sprintf("%s%s=%s", prefix, p[0], p[1])
	}
	for i := range configs {
		configs[i].initialCluster = initialClusterStr
	}

	if isClientTLS {
		for i := range configs {
			configs[i].clientTLSInfo = testTLSInfo
		}
	}
	if isPeerTLS {
		for i := range configs {
			configs[i].peerTLSInfo = testTLSInfo
		}
	}
	return configs, nil
}
