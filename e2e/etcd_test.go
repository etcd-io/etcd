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
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/gexpect"
	"github.com/coreos/etcd/pkg/fileutil"
)

const (
	etcdProcessBasePort = 20000
	certPath            = "../integration/fixtures/server.crt"
	privateKeyPath      = "../integration/fixtures/server.key.insecure"
	caPath              = "../integration/fixtures/ca.crt"
)

func TestBasicOpsNoTLS(t *testing.T) {
	testProcessClusterPutGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			isClientTLS:  false,
			isPeerTLS:    false,
			initialToken: "new",
		},
	)
}

func TestBasicOpsAllTLS(t *testing.T) {
	testProcessClusterPutGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			isClientTLS:  true,
			isPeerTLS:    true,
			initialToken: "new",
		},
	)
}

func TestBasicOpsPeerTLS(t *testing.T) {
	testProcessClusterPutGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			isClientTLS:  false,
			isPeerTLS:    true,
			initialToken: "new",
		},
	)
}

func TestBasicOpsClientTLS(t *testing.T) {
	testProcessClusterPutGet(
		t,
		&etcdProcessClusterConfig{
			clusterSize:  3,
			isClientTLS:  true,
			isPeerTLS:    false,
			initialToken: "new",
		},
	)
}

func testProcessClusterPutGet(t *testing.T, cfg *etcdProcessClusterConfig) {
	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	expectPut := `{"action":"set","node":{"key":"/testKey","value":"foo","`
	if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}

	expectGet := `{"action":"get","node":{"key":"/testKey","value":"foo","`
	if err := cURLGet(epc, "testKey", expectGet); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}
}

// cURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func cURLPrefixArgs(clus *etcdProcessCluster, key string) []string {
	cmd := []string{"curl"}
	if clus.cfg.isClientTLS {
		cmd = append(cmd, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	}
	acurl := clus.procs[rand.Intn(clus.cfg.clusterSize)].cfg.acurl
	keyURL := acurl.String() + "/v2/keys/testKey"
	cmd = append(cmd, "-L", keyURL)
	return cmd
}

func cURLPut(clus *etcdProcessCluster, key, val, expected string) error {
	args := append(cURLPrefixArgs(clus, key), "-XPUT", "-d", "value="+val)
	return spawnWithExpect(args, expected)
}

func cURLGet(clus *etcdProcessCluster, key, expected string) error {
	return spawnWithExpect(cURLPrefixArgs(clus, key), expected)
}

type etcdProcessCluster struct {
	cfg   *etcdProcessClusterConfig
	procs []*etcdProcess
}

type etcdProcess struct {
	cfg   *etcdProcessConfig
	proc  *gexpect.ExpectSubprocess
	donec chan struct{} // closed when Interact() terminates
}

type etcdProcessConfig struct {
	args        []string
	dataDirPath string
	acurl       url.URL
}

type etcdProcessClusterConfig struct {
	clusterSize  int
	isClientTLS  bool
	isPeerTLS    bool
	initialToken string
}

// newEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new etcdProcessCluster once all nodes are ready to accept client requests.
func newEtcdProcessCluster(cfg *etcdProcessClusterConfig) (*etcdProcessCluster, error) {
	etcdCfgs := cfg.etcdProcessConfigs()
	epc := &etcdProcessCluster{
		cfg:   cfg,
		procs: make([]*etcdProcess, cfg.clusterSize),
	}

	// launch etcd processes
	for i := range etcdCfgs {
		proc, err := newEtcdProcess(etcdCfgs[i])
		if err != nil {
			epc.Close()
			return nil, err
		}
		epc.procs[i] = proc
	}

	// wait for cluster to start
	readyC := make(chan error, cfg.clusterSize)
	readyStr := "set the initial cluster version"
	for i := range etcdCfgs {
		go func(etcdp *etcdProcess) {
			_, err := etcdp.proc.ExpectRegex(readyStr)
			readyC <- err
			etcdp.proc.ReadUntil('\n') // don't display rest of line
			etcdp.proc.Interact()
			close(etcdp.donec)
		}(epc.procs[i])
	}
	for range etcdCfgs {
		if err := <-readyC; err != nil {
			epc.Close()
			return nil, err
		}
	}
	return epc, nil
}

func newEtcdProcess(cfg *etcdProcessConfig) (*etcdProcess, error) {
	if fileutil.Exist("../bin/etcd") == false {
		return nil, fmt.Errorf("could not find etcd binary")
	}
	if err := os.RemoveAll(cfg.dataDirPath); err != nil {
		return nil, err
	}
	child, err := spawnCmd(append([]string{"../bin/etcd"}, cfg.args...))
	if err != nil {
		return nil, err
	}
	child.Capture()
	return &etcdProcess{cfg: cfg, proc: child, donec: make(chan struct{})}, nil
}

func (cfg *etcdProcessClusterConfig) etcdProcessConfigs() []*etcdProcessConfig {
	clientScheme := "http"
	if cfg.isClientTLS {
		clientScheme = "https"
	}
	peerScheme := "http"
	if cfg.isPeerTLS {
		peerScheme = "https"
	}

	etcdCfgs := make([]*etcdProcessConfig, cfg.clusterSize)
	initialCluster := make([]string, cfg.clusterSize)
	for i := 0; i < cfg.clusterSize; i++ {
		port := etcdProcessBasePort + 2*i
		curl := url.URL{Scheme: clientScheme, Host: fmt.Sprintf("localhost:%d", port)}
		purl := url.URL{Scheme: peerScheme, Host: fmt.Sprintf("localhost:%d", port+1)}
		name := fmt.Sprintf("testname%d", i)
		dataDirPath := name + ".etcd"
		initialCluster[i] = fmt.Sprintf("%s=%s", name, purl.String())

		args := []string{
			"--name", name,
			"--listen-client-urls", curl.String(),
			"--advertise-client-urls", curl.String(),
			"--listen-peer-urls", purl.String(),
			"--initial-advertise-peer-urls", purl.String(),
			"--initial-cluster-token", cfg.initialToken,
			"--data-dir", dataDirPath,
		}
		if cfg.isClientTLS {
			tlsClientArgs := []string{
				"--cert-file", certPath,
				"--key-file", privateKeyPath,
				"--ca-file", caPath,
			}
			args = append(args, tlsClientArgs...)
		}
		if cfg.isPeerTLS {
			tlsPeerArgs := []string{
				"--peer-cert-file", certPath,
				"--peer-key-file", privateKeyPath,
				"--peer-ca-file", caPath,
			}
			args = append(args, tlsPeerArgs...)
		}

		etcdCfgs[i] = &etcdProcessConfig{
			args:        args,
			dataDirPath: dataDirPath,
			acurl:       curl,
		}
	}

	initialClusterArgs := []string{"--initial-cluster", strings.Join(initialCluster, ",")}
	for i := range etcdCfgs {
		etcdCfgs[i].args = append(etcdCfgs[i].args, initialClusterArgs...)
	}

	return etcdCfgs
}

func (epc *etcdProcessCluster) Close() (err error) {
	for _, p := range epc.procs {
		if p == nil {
			continue
		}
		os.RemoveAll(p.cfg.dataDirPath)
		if curErr := p.proc.Close(); curErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; %v", err, curErr)
			} else {
				err = curErr
			}
		}
		<-p.donec
	}
	return err
}

func spawnCmd(args []string) (*gexpect.ExpectSubprocess, error) {
	// redirect stderr to stdout since gexpect only uses stdout
	cmd := `/bin/sh -c "` + strings.Join(args, " ") + ` 2>&1 "`
	return gexpect.Spawn(cmd)
}

func spawnWithExpect(args []string, expected string) error {
	proc, err := spawnCmd(args)
	if err != nil {
		return err
	}
	if _, err := proc.ExpectRegex(expected); err != nil {
		return err
	}
	return proc.Close()
}
