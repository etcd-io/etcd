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
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/gexpect"
)

const (
	etcdProcessBasePort = 20000
	certPath            = "../integration/fixtures/server.crt"
	privateKeyPath      = "../integration/fixtures/server.key.insecure"
	caPath              = "../integration/fixtures/ca.crt"
)

type clientConnType int

const (
	clientNonTLS clientConnType = iota
	clientTLS
	clientTLSAndNonTLS
)

var (
	configNoTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    0,
		isPeerTLS:    false,
		initialToken: "new",
	}
	configAutoTLS = etcdProcessClusterConfig{
		clusterSize:   3,
		isPeerTLS:     true,
		isPeerAutoTLS: true,
		initialToken:  "new",
	}
	configTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    0,
		clientTLS:    clientTLS,
		isPeerTLS:    true,
		initialToken: "new",
	}
	configClientTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    0,
		clientTLS:    clientTLS,
		isPeerTLS:    false,
		initialToken: "new",
	}
	configClientBoth = etcdProcessClusterConfig{
		clusterSize:  1,
		proxySize:    0,
		clientTLS:    clientTLSAndNonTLS,
		isPeerTLS:    false,
		initialToken: "new",
	}
	configPeerTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    0,
		isPeerTLS:    true,
		initialToken: "new",
	}
	configWithProxy = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    1,
		isPeerTLS:    false,
		initialToken: "new",
	}
	configWithProxyTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    1,
		clientTLS:    clientTLS,
		isPeerTLS:    true,
		initialToken: "new",
	}
	configWithProxyPeerTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		proxySize:    1,
		isPeerTLS:    true,
		initialToken: "new",
	}
)

func configStandalone(cfg etcdProcessClusterConfig) *etcdProcessClusterConfig {
	ret := cfg
	ret.clusterSize = 1
	return &ret
}

func TestBasicOpsNoTLS(t *testing.T)        { testBasicOpsPutGet(t, &configNoTLS) }
func TestBasicOpsAutoTLS(t *testing.T)      { testBasicOpsPutGet(t, &configAutoTLS) }
func TestBasicOpsAllTLS(t *testing.T)       { testBasicOpsPutGet(t, &configTLS) }
func TestBasicOpsPeerTLS(t *testing.T)      { testBasicOpsPutGet(t, &configPeerTLS) }
func TestBasicOpsClientTLS(t *testing.T)    { testBasicOpsPutGet(t, &configClientTLS) }
func TestBasicOpsProxyNoTLS(t *testing.T)   { testBasicOpsPutGet(t, &configWithProxy) }
func TestBasicOpsProxyTLS(t *testing.T)     { testBasicOpsPutGet(t, &configWithProxyTLS) }
func TestBasicOpsProxyPeerTLS(t *testing.T) { testBasicOpsPutGet(t, &configWithProxyPeerTLS) }
func TestBasicOpsClientBoth(t *testing.T)   { testBasicOpsPutGet(t, &configClientBoth) }

func testBasicOpsPutGet(t *testing.T, cfg *etcdProcessClusterConfig) {
	defer testutil.AfterTest(t)

	// test doesn't use quorum gets, so ensure there are no followers to avoid
	// stale reads that will break the test
	cfg = configStandalone(*cfg)

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
	expectGet := `{"action":"get","node":{"key":"/testKey","value":"foo","`

	if cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}

		if err := cURLGet(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
		if err := cURLGetUseTLS(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	} else {
		if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}

		if err := cURLGet(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

// cURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func cURLPrefixArgs(clus *etcdProcessCluster, key string) []string {
	cmdArgs := []string{"curl"}
	acurl := clus.procs[rand.Intn(clus.cfg.clusterSize)].cfg.acurl

	if clus.cfg.clientTLS == clientTLS {
		cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	}
	keyURL := acurl + "/v2/keys/testKey"
	cmdArgs = append(cmdArgs, "-L", keyURL)
	return cmdArgs
}

func cURLPrefixArgsUseTLS(clus *etcdProcessCluster, key string) []string {
	cmdArgs := []string{"curl"}
	if clus.cfg.clientTLS != clientTLSAndNonTLS {
		panic("should not use cURLPrefixArgsUseTLS when serving only TLS or non-TLS")
	}
	cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	acurl := clus.procs[rand.Intn(clus.cfg.clusterSize)].cfg.acurltls
	keyURL := acurl + "/v2/keys/testKey"
	cmdArgs = append(cmdArgs, "-L", keyURL)
	return cmdArgs
}

func cURLPut(clus *etcdProcessCluster, key, val, expected string) error {
	args := append(cURLPrefixArgs(clus, key), "-XPUT", "-d", "value="+val)
	return spawnWithExpectedString(args, expected)
}

func cURLGet(clus *etcdProcessCluster, key, expected string) error {
	return spawnWithExpectedString(cURLPrefixArgs(clus, key), expected)
}

func cURLGetUseTLS(clus *etcdProcessCluster, key, expected string) error {
	return spawnWithExpectedString(cURLPrefixArgsUseTLS(clus, key), expected)
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
	acurl       string
	// additional url for tls connection when the etcd process
	// serves both http and https
	acurltls string
	isProxy  bool
}

type etcdProcessClusterConfig struct {
	clusterSize   int
	proxySize     int
	clientTLS     clientConnType
	isPeerTLS     bool
	isPeerAutoTLS bool
	initialToken  string
}

// newEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new etcdProcessCluster once all nodes are ready to accept client requests.
func newEtcdProcessCluster(cfg *etcdProcessClusterConfig) (*etcdProcessCluster, error) {
	etcdCfgs := cfg.etcdProcessConfigs()
	epc := &etcdProcessCluster{
		cfg:   cfg,
		procs: make([]*etcdProcess, cfg.clusterSize+cfg.proxySize),
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
	readyC := make(chan error, cfg.clusterSize+cfg.proxySize)
	readyStr := "etcdserver: set the initial cluster version to"
	for i := range etcdCfgs {
		go func(etcdp *etcdProcess) {
			rs := readyStr
			if etcdp.cfg.isProxy {
				// rs = "proxy: listening for client requests on"
				rs = "proxy: endpoints found"
			}
			ok, err := etcdp.proc.ExpectRegex(rs)
			if err != nil {
				readyC <- err
			} else if !ok {
				readyC <- fmt.Errorf("couldn't get expected output: '%s'", rs)
			} else {
				readyC <- nil
			}
			etcdp.proc.ReadLine()
			etcdp.proc.Interact() // this blocks(leaks) if another goroutine is reading
			etcdp.proc.ReadLine() // wait for leaky goroutine to accept an EOF
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
	return &etcdProcess{cfg: cfg, proc: child, donec: make(chan struct{})}, nil
}

func (cfg *etcdProcessClusterConfig) etcdProcessConfigs() []*etcdProcessConfig {
	clientScheme := "http"
	if cfg.clientTLS == clientTLS {
		clientScheme = "https"
	}
	peerScheme := "http"
	if cfg.isPeerTLS {
		peerScheme = "https"
	}

	etcdCfgs := make([]*etcdProcessConfig, cfg.clusterSize+cfg.proxySize)
	initialCluster := make([]string, cfg.clusterSize)
	for i := 0; i < cfg.clusterSize; i++ {
		var curls []string
		var curl, curltls string
		port := etcdProcessBasePort + 2*i

		switch cfg.clientTLS {
		case clientNonTLS, clientTLS:
			curl = (&url.URL{Scheme: clientScheme, Host: fmt.Sprintf("localhost:%d", port)}).String()
			curls = []string{curl}
		case clientTLSAndNonTLS:
			curl = (&url.URL{Scheme: "http", Host: fmt.Sprintf("localhost:%d", port)}).String()
			curltls = (&url.URL{Scheme: "https", Host: fmt.Sprintf("localhost:%d", port)}).String()
			curls = []string{curl, curltls}
		}

		purl := url.URL{Scheme: peerScheme, Host: fmt.Sprintf("localhost:%d", port+1)}
		name := fmt.Sprintf("testname%d", i)
		dataDirPath, derr := ioutil.TempDir("", name+".etcd")
		if derr != nil {
			panic("could not get tempdir for datadir")
		}
		initialCluster[i] = fmt.Sprintf("%s=%s", name, purl.String())

		args := []string{
			"--name", name,
			"--listen-client-urls", strings.Join(curls, ","),
			"--advertise-client-urls", strings.Join(curls, ","),
			"--listen-peer-urls", purl.String(),
			"--initial-advertise-peer-urls", purl.String(),
			"--initial-cluster-token", cfg.initialToken,
			"--data-dir", dataDirPath,
		}

		args = append(args, cfg.tlsArgs()...)

		etcdCfgs[i] = &etcdProcessConfig{
			args:        args,
			dataDirPath: dataDirPath,
			acurl:       curl,
			acurltls:    curltls,
		}
	}
	for i := 0; i < cfg.proxySize; i++ {
		port := etcdProcessBasePort + 2*cfg.clusterSize + i + 1
		curl := url.URL{Scheme: clientScheme, Host: fmt.Sprintf("localhost:%d", port)}
		name := fmt.Sprintf("testname-proxy%d", i)
		dataDirPath, derr := ioutil.TempDir("", name+".etcd")
		if derr != nil {
			panic("could not get tempdir for datadir")
		}
		args := []string{
			"--name", name,
			"--proxy", "on",
			"--listen-client-urls", curl.String(),
			"--data-dir", dataDirPath,
		}
		args = append(args, cfg.tlsArgs()...)
		etcdCfgs[cfg.clusterSize+i] = &etcdProcessConfig{
			args:        args,
			dataDirPath: dataDirPath,
			acurl:       curl.String(),
			isProxy:     true,
		}
	}

	initialClusterArgs := []string{"--initial-cluster", strings.Join(initialCluster, ",")}
	for i := range etcdCfgs {
		etcdCfgs[i].args = append(etcdCfgs[i].args, initialClusterArgs...)
	}

	return etcdCfgs
}

func (cfg *etcdProcessClusterConfig) tlsArgs() (args []string) {
	if cfg.clientTLS != clientNonTLS {
		tlsClientArgs := []string{
			"--cert-file", certPath,
			"--key-file", privateKeyPath,
			"--ca-file", caPath,
		}
		args = append(args, tlsClientArgs...)
	}
	if cfg.isPeerTLS {
		if cfg.isPeerAutoTLS {
			args = append(args, "--peer-auto-tls=true")
		} else {
			tlsPeerArgs := []string{
				"--peer-cert-file", certPath,
				"--peer-key-file", privateKeyPath,
				"--peer-ca-file", caPath,
			}
			args = append(args, tlsPeerArgs...)
		}
	}
	return args
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
	ok, err := proc.ExpectRegex(expected)
	perr := proc.Close()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("couldn't get expected output: '%s'", expected)
	}
	return perr
}

// spawnWithExpectedString compares outputs in string format.
// This is useful when gexpect does not match regex correctly with
// some UTF-8 format characters.
func spawnWithExpectedString(args []string, expected string) error {
	proc, err := spawnCmd(args)
	if err != nil {
		return err
	}
	s, err := proc.ReadLine()
	perr := proc.Close()
	if err != nil {
		return err
	}
	if !strings.Contains(s, expected) {
		return fmt.Errorf("expected %q, got %q", expected, s)
	}
	return perr
}

// proxies returns only the proxy etcdProcess.
func (epc *etcdProcessCluster) proxies() []*etcdProcess {
	return epc.procs[epc.cfg.clusterSize:]
}

func (epc *etcdProcessCluster) backends() []*etcdProcess {
	return epc.procs[:epc.cfg.clusterSize]
}
