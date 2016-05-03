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
	"net/url"
	"os"
	"strings"

	"github.com/coreos/etcd/pkg/expect"
	"github.com/coreos/etcd/pkg/fileutil"
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
		initialToken: "new",
	}
	configClientBoth = etcdProcessClusterConfig{
		clusterSize:  1,
		proxySize:    0,
		clientTLS:    clientTLSAndNonTLS,
		initialToken: "new",
	}
	configClientAutoTLS = etcdProcessClusterConfig{
		clusterSize:     1,
		proxySize:       0,
		isClientAuthTLS: true,
		clientTLS:       clientTLS,
		initialToken:    "new",
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

type etcdProcessCluster struct {
	cfg   *etcdProcessClusterConfig
	procs []*etcdProcess
}

type etcdProcess struct {
	cfg   *etcdProcessConfig
	proc  *expect.ExpectProcess
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
	clusterSize       int
	basePort          int
	proxySize         int
	clientTLS         clientConnType
	isPeerTLS         bool
	isPeerAutoTLS     bool
	isClientAuthTLS   bool
	initialToken      string
	quotaBackendBytes int64
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
	readyStr := "enabled capabilities for version"
	for i := range etcdCfgs {
		go func(etcdp *etcdProcess) {
			rs := readyStr
			if etcdp.cfg.isProxy {
				// rs = "proxy: listening for client requests on"
				rs = "proxy: endpoints found"
			}
			_, err := etcdp.proc.Expect(rs)
			readyC <- err
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
	if !fileutil.Exist("../bin/etcd") {
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
	if cfg.basePort == 0 {
		cfg.basePort = etcdProcessBasePort
	}

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
		port := cfg.basePort + 2*i

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
		if cfg.quotaBackendBytes > 0 {
			args = append(args,
				"--quota-backend-bytes", fmt.Sprintf("%d", cfg.quotaBackendBytes),
			)
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
		port := cfg.basePort + 2*cfg.clusterSize + i + 1
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
		if cfg.isClientAuthTLS {
			args = append(args, "--auto-tls=true")
		} else {
			tlsClientArgs := []string{
				"--cert-file", certPath,
				"--key-file", privateKeyPath,
				"--ca-file", caPath,
			}
			args = append(args, tlsClientArgs...)
		}
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
		if curErr := p.proc.Stop(); curErr != nil {
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

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	return expect.NewExpect(args[0], args[1:]...)
}

func spawnWithExpect(args []string, expected string) error {
	return spawnWithExpects(args, []string{expected}...)
}

func spawnWithExpects(args []string, xs ...string) error {
	proc, err := spawnCmd(args)
	if err != nil {
		return err
	}
	// process until either stdout or stderr contains
	// the expected string
	var (
		lines    []string
		lineFunc = func(txt string) bool { return true }
	)
	for _, txt := range xs {
		for {
			l, err := proc.ExpectFunc(lineFunc)
			if err != nil {
				return fmt.Errorf("%v (expected %q, got %q)", err, txt, lines)
			}
			lines = append(lines, l)
			if strings.Contains(l, txt) {
				break
			}
		}
	}
	perr := proc.Close()
	if err != nil {
		return err
	}
	if len(xs) == 0 && proc.LineCount() != 0 { // expect no output
		return fmt.Errorf("unexpected output (got lines %q, line count %d)", lines, proc.LineCount())
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

func (epc *etcdProcessCluster) endpoints() []string {
	eps := make([]string, epc.cfg.clusterSize)
	for i, ep := range epc.backends() {
		eps[i] = ep.cfg.acurl
	}
	return eps
}
