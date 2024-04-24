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
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/etcdserver"
	"go.etcd.io/etcd/pkg/proxy"
	"go.uber.org/zap"
)

const etcdProcessBasePort = 20000

var (
	configNoTLS = etcdProcessClusterConfig{
		clusterSize:  3,
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
		clientTLS:    clientTLS,
		isPeerTLS:    true,
		initialToken: "new",
	}
	configClientTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		clientTLS:    clientTLS,
		initialToken: "new",
	}
	configClientBoth = etcdProcessClusterConfig{
		clusterSize:  1,
		clientTLS:    clientTLSAndNonTLS,
		initialToken: "new",
	}
	configClientAutoTLS = etcdProcessClusterConfig{
		clusterSize:     1,
		isClientAutoTLS: true,
		clientTLS:       clientTLS,
		initialToken:    "new",
	}
	configPeerTLS = etcdProcessClusterConfig{
		clusterSize:  3,
		isPeerTLS:    true,
		initialToken: "new",
	}
	configClientTLSCertAuth = etcdProcessClusterConfig{
		clusterSize:           1,
		clientTLS:             clientTLS,
		initialToken:          "new",
		clientCertAuthEnabled: true,
	}
	configClientTLSCertAuthWithNoCN = etcdProcessClusterConfig{
		clusterSize:           1,
		clientTLS:             clientTLS,
		initialToken:          "new",
		clientCertAuthEnabled: true,
		noCN:                  true,
	}
	configJWT = etcdProcessClusterConfig{
		clusterSize:   1,
		initialToken:  "new",
		authTokenOpts: "jwt,pub-key=../../integration/fixtures/server.crt,priv-key=../../integration/fixtures/server.key.insecure,sign-method=RS256,ttl=1s",
	}
)

func configStandalone(cfg etcdProcessClusterConfig) *etcdProcessClusterConfig {
	ret := cfg
	ret.clusterSize = 1
	return &ret
}

type etcdProcessCluster struct {
	cfg   *etcdProcessClusterConfig
	procs []etcdProcess
}

type etcdProcessClusterConfig struct {
	execPath            string
	dataDirPath         string
	keepDataDir         bool
	goFailEnabled       bool
	goFailClientTimeout time.Duration
	peerProxy           bool
	envVars             map[string]string

	clusterSize int

	baseScheme string
	basePort   int

	metricsURLScheme string

	snapshotCount int // default is 10000

	clientTLS             clientConnType
	clientCertAuthEnabled bool
	clientHttpSeparate    bool
	isPeerTLS             bool
	isPeerAutoTLS         bool
	isClientAutoTLS       bool
	isClientCRL           bool
	noCN                  bool

	cipherSuites []string

	forceNewCluster     bool
	initialToken        string
	quotaBackendBytes   int64
	noStrictReconfig    bool
	enableV2            bool
	initialCorruptCheck bool
	corruptCheckTime    time.Duration
	authTokenOpts       string

	MaxConcurrentStreams       uint32 // default is math.MaxUint32
	WatchProcessNotifyInterval time.Duration
	CompactionBatchLimit       int

	debug bool

	stopSignal os.Signal
}

// newEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new etcdProcessCluster once all nodes are ready to accept client requests.
func newEtcdProcessCluster(t testing.TB, cfg *etcdProcessClusterConfig) (*etcdProcessCluster, error) {
	epc, err := initEtcdProcessCluster(cfg)
	if err != nil {
		return nil, err
	}

	return startEtcdProcessCluster(t, epc, cfg)
}

// `initEtcdProcessCluster` initializes a new cluster based on the given config.
// It doesn't start the cluster.
func initEtcdProcessCluster(cfg *etcdProcessClusterConfig) (*etcdProcessCluster, error) {
	etcdCfgs := cfg.etcdServerProcessConfigs()
	epc := &etcdProcessCluster{
		cfg:   cfg,
		procs: make([]etcdProcess, cfg.clusterSize),
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

	return epc, nil
}

// `startEtcdProcessCluster` launches a new cluster from etcd processes.
func startEtcdProcessCluster(t testing.TB, epc *etcdProcessCluster, cfg *etcdProcessClusterConfig) (*etcdProcessCluster, error) {
	if err := epc.Start(); err != nil {
		return nil, err
	}

	// overwrite the default signal
	if cfg.stopSignal != nil {
		for _, proc := range epc.procs {
			proc.WithStopSignal(cfg.stopSignal)
		}
	}
	for _, proc := range epc.procs {
		if cfg.goFailEnabled && !proc.Failpoints().Enabled() {
			epc.Close()
			t.Skip("please run test with 'FAILPOINTS=true'")
		}
	}
	return epc, nil
}

func (cfg *etcdProcessClusterConfig) clientScheme() string {
	if cfg.clientTLS == clientTLS {
		return "https"
	}
	return "http"
}

func (cfg *etcdProcessClusterConfig) peerScheme() string {
	peerScheme := cfg.baseScheme
	if peerScheme == "" {
		peerScheme = "http"
	}
	if cfg.isPeerTLS {
		peerScheme += "s"
	}
	return peerScheme
}

func (cfg *etcdProcessClusterConfig) etcdServerProcessConfigs() []*etcdServerProcessConfig {
	if cfg.basePort == 0 {
		cfg.basePort = etcdProcessBasePort
	}
	if cfg.execPath == "" {
		cfg.execPath = binPath
	}
	if cfg.snapshotCount == 0 {
		cfg.snapshotCount = etcdserver.DefaultSnapshotCount
	}

	etcdCfgs := make([]*etcdServerProcessConfig, cfg.clusterSize)
	initialCluster := make([]string, cfg.clusterSize)
	for i := 0; i < cfg.clusterSize; i++ {
		var curls []string
		var curl string
		port := cfg.basePort + 5*i
		peerPort := port + 1
		peer2Port := port + 3
		clientPort := port
		clientHttpPort := port + 4

		if cfg.clientTLS == clientTLSAndNonTLS {
			curl = clientURL(clientPort, clientNonTLS)
			curls = []string{curl, clientURL(clientPort, clientTLS)}
		} else {
			curl = clientURL(clientPort, cfg.clientTLS)
			curls = []string{curl}
		}

		purl := url.URL{Scheme: cfg.peerScheme(), Host: fmt.Sprintf("localhost:%d", port+1)}
		peerAdvertiseUrl := url.URL{Scheme: cfg.peerScheme(), Host: fmt.Sprintf("localhost:%d", peerPort)}
		var proxyCfg *proxy.ServerConfig
		if cfg.peerProxy {
			if !cfg.isPeerTLS {
				panic("Can't use peer proxy without peer TLS as it can result in malformed packets")
			}
			peerAdvertiseUrl.Host = fmt.Sprintf("localhost:%d", peer2Port)
			proxyCfg = &proxy.ServerConfig{
				Logger: zap.NewNop(),
				To:     purl,
				From:   peerAdvertiseUrl,
			}
		}

		name := fmt.Sprintf("testname%d", i)
		dataDirPath := cfg.dataDirPath
		if cfg.dataDirPath == "" {
			var derr error
			dataDirPath, derr = ioutil.TempDir("", name+".etcd")
			if derr != nil {
				panic(fmt.Sprintf("could not get tempdir for datadir: %s", derr))
			}
		}
		initialCluster[i] = fmt.Sprintf("%s=%s", name, peerAdvertiseUrl.String())

		args := []string{
			"--name", name,
			"--listen-client-urls", strings.Join(curls, ","),
			"--advertise-client-urls", strings.Join(curls, ","),
			"--listen-peer-urls", purl.String(),
			"--initial-advertise-peer-urls", peerAdvertiseUrl.String(),
			"--initial-cluster-token", cfg.initialToken,
			"--data-dir", dataDirPath,
			"--snapshot-count", fmt.Sprintf("%d", cfg.snapshotCount),
		}
		var clientHttpUrl string
		if cfg.clientHttpSeparate {
			clientHttpUrl = clientURL(clientHttpPort, cfg.clientTLS)
			args = append(args, "--listen-client-http-urls", clientHttpUrl)
		}
		args = addV2Args(args)
		if cfg.forceNewCluster {
			args = append(args, "--force-new-cluster")
		}
		if cfg.quotaBackendBytes > 0 {
			args = append(args,
				"--quota-backend-bytes", fmt.Sprintf("%d", cfg.quotaBackendBytes),
			)
		}
		if cfg.noStrictReconfig {
			args = append(args, "--strict-reconfig-check=false")
		}
		if cfg.enableV2 {
			args = append(args, "--enable-v2")
		}
		if cfg.initialCorruptCheck {
			args = append(args, "--experimental-initial-corrupt-check")
		}
		if cfg.corruptCheckTime != 0 {
			args = append(args, "--experimental-corrupt-check-time", cfg.corruptCheckTime.String())
		}
		var murl string
		if cfg.metricsURLScheme != "" {
			murl = (&url.URL{
				Scheme: cfg.metricsURLScheme,
				Host:   fmt.Sprintf("localhost:%d", port+2),
			}).String()
			args = append(args, "--listen-metrics-urls", murl)
		}

		args = append(args, cfg.tlsArgs()...)

		if cfg.authTokenOpts != "" {
			args = append(args, "--auth-token", cfg.authTokenOpts)
		}

		if cfg.MaxConcurrentStreams != 0 {
			args = append(args, "--max-concurrent-streams", fmt.Sprintf("%d", cfg.MaxConcurrentStreams))
		}

		if cfg.WatchProcessNotifyInterval != 0 {
			args = append(args, "--experimental-watch-progress-notify-interval", cfg.WatchProcessNotifyInterval.String())
		}
		if cfg.CompactionBatchLimit != 0 {
			args = append(args, "--experimental-compaction-batch-limit", fmt.Sprintf("%d", cfg.CompactionBatchLimit))
		}

		if cfg.debug {
			args = append(args, "--debug")
		}

		envVars := map[string]string{}
		for key, value := range cfg.envVars {
			envVars[key] = value
		}
		var gofailPort int
		if cfg.goFailEnabled {
			gofailPort = (i+1)*10000 + 2381
			envVars["GOFAIL_HTTP"] = fmt.Sprintf("127.0.0.1:%d", gofailPort)
		}

		etcdCfgs[i] = &etcdServerProcessConfig{
			execPath:            cfg.execPath,
			args:                args,
			envVars:             envVars,
			tlsArgs:             cfg.tlsArgs(),
			dataDirPath:         dataDirPath,
			keepDataDir:         cfg.keepDataDir,
			name:                name,
			purl:                peerAdvertiseUrl,
			acurl:               curl,
			murl:                murl,
			initialToken:        cfg.initialToken,
			clientHttpUrl:       clientHttpUrl,
			goFailPort:          gofailPort,
			goFailClientTimeout: cfg.goFailClientTimeout,
			proxy:               proxyCfg,
		}
	}

	initialClusterArgs := []string{"--initial-cluster", strings.Join(initialCluster, ",")}
	for i := range etcdCfgs {
		etcdCfgs[i].initialCluster = strings.Join(initialCluster, ",")
		etcdCfgs[i].args = append(etcdCfgs[i].args, initialClusterArgs...)
	}

	return etcdCfgs
}

func clientURL(port int, connType clientConnType) string {
	curlHost := fmt.Sprintf("localhost:%d", port)
	switch connType {
	case clientNonTLS:
		return (&url.URL{Scheme: "http", Host: curlHost}).String()
	case clientTLS:
		return (&url.URL{Scheme: "https", Host: curlHost}).String()
	default:
		panic(fmt.Sprintf("Unsupported connection type %v", connType))
	}
}

func (cfg *etcdProcessClusterConfig) tlsArgs() (args []string) {
	if cfg.clientTLS != clientNonTLS {
		if cfg.isClientAutoTLS {
			args = append(args, "--auto-tls")
		} else {
			tlsClientArgs := []string{
				"--cert-file", certPath,
				"--key-file", privateKeyPath,
				"--trusted-ca-file", caPath,
			}
			args = append(args, tlsClientArgs...)

			if cfg.clientCertAuthEnabled {
				args = append(args, "--client-cert-auth")
			}
		}
	}

	if cfg.isPeerTLS {
		if cfg.isPeerAutoTLS {
			args = append(args, "--peer-auto-tls")
		} else {
			tlsPeerArgs := []string{
				"--peer-cert-file", certPath,
				"--peer-key-file", privateKeyPath,
				"--peer-trusted-ca-file", caPath,
			}
			args = append(args, tlsPeerArgs...)
		}
	}

	if cfg.isClientCRL {
		args = append(args, "--client-crl-file", crlPath, "--client-cert-auth")
	}

	if len(cfg.cipherSuites) > 0 {
		args = append(args, "--cipher-suites", strings.Join(cfg.cipherSuites, ","))
	}

	return args
}

func (epc *etcdProcessCluster) EndpointsV2() []string {
	return epc.endpoints(func(ep etcdProcess) []string { return ep.EndpointsV2() })
}

func (epc *etcdProcessCluster) EndpointsV3() []string {
	return epc.endpoints(func(ep etcdProcess) []string { return ep.EndpointsV3() })
}

func (epc *etcdProcessCluster) EndpointsGRPC() []string {
	return epc.endpoints(func(ep etcdProcess) []string { return ep.EndpointsGRPC() })
}

func (epc *etcdProcessCluster) EndpointsHTTP() []string {
	return epc.endpoints(func(ep etcdProcess) []string { return ep.EndpointsHTTP() })
}

func (epc *etcdProcessCluster) endpoints(f func(ep etcdProcess) []string) (ret []string) {
	for _, p := range epc.procs {
		ret = append(ret, f(p)...)
	}
	return ret
}

func (epc *etcdProcessCluster) Start() error {
	return epc.start(func(ep etcdProcess) error { return ep.Start() })
}

func (epc *etcdProcessCluster) Restart() error {
	return epc.start(func(ep etcdProcess) error { return ep.Restart() })
}

func (epc *etcdProcessCluster) start(f func(ep etcdProcess) error) error {
	readyC := make(chan error, len(epc.procs))
	for i := range epc.procs {
		go func(n int) { readyC <- f(epc.procs[n]) }(i)
	}
	for range epc.procs {
		if err := <-readyC; err != nil {
			epc.Close()
			return err
		}
	}
	return nil
}

func (epc *etcdProcessCluster) Stop() (err error) {
	for _, p := range epc.procs {
		if p == nil {
			continue
		}
		if curErr := p.Stop(); curErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; %v", err, curErr)
			} else {
				err = curErr
			}
		}
	}
	return err
}

func (epc *etcdProcessCluster) Close() error {
	err := epc.Stop()
	for _, p := range epc.procs {
		// p is nil when newEtcdProcess fails in the middle
		// Close still gets called to clean up test data
		if p == nil {
			continue
		}
		if cerr := p.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}

func (epc *etcdProcessCluster) WithStopSignal(sig os.Signal) (ret os.Signal) {
	for _, p := range epc.procs {
		ret = p.WithStopSignal(sig)
	}
	return ret
}

// WaitLeader returns index of the member in c.Members() that is leader
// or fails the test (if not established in 30s).
func (epc *etcdProcessCluster) WaitLeader(t testing.TB) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return epc.WaitMembersForLeader(ctx, t, epc.procs)
}

// WaitMembersForLeader waits until given members agree on the same leader,
// and returns its 'index' in the 'membs' list
func (epc *etcdProcessCluster) WaitMembersForLeader(ctx context.Context, t testing.TB, membs []etcdProcess) int {
	cc := NewEtcdctl(epc.EndpointsV3(), epc.cfg.clientTLS, epc.cfg.isClientAutoTLS, epc.cfg.enableV2)

	// ensure leader is up via linearizable get
	for {
		select {
		case <-ctx.Done():
			t.Fatal("WaitMembersForLeader timeout")
		default:
		}
		_, err := cc.Get("0")
		if err == nil || strings.Contains(err.Error(), "Key not found") {
			break
		}
		t.Logf("WaitMembersForLeader Get err: %v", err)
	}

	leaders := make(map[uint64]struct{})
	members := make(map[uint64]int)
	for {
		select {
		case <-ctx.Done():
			t.Fatal("WaitMembersForLeader timeout")
		default:
		}
		for i := range membs {
			resp, err := membs[i].Etcdctl(epc.cfg.clientTLS, epc.cfg.isClientAutoTLS, epc.cfg.enableV2).Status()
			if err != nil {
				if strings.Contains(err.Error(), "connection refused") {
					// if member[i] has stopped
					continue
				} else {
					t.Fatal(err)
				}
			}
			members[resp[0].Header.MemberId] = i
			leaders[resp[0].Leader] = struct{}{}
		}
		// members agree on the same leader
		if len(leaders) == 1 {
			break
		}
		leaders = make(map[uint64]struct{})
		members = make(map[uint64]int)
		// From main branch 10 * config.TickDuration (10 * time.Millisecond)
		time.Sleep(100 * time.Millisecond)
	}
	for l := range leaders {
		if index, ok := members[l]; ok {
			t.Logf("members agree on a leader, members:%v , leader:%v", members, l)
			return index
		}
		t.Fatalf("members agree on a leader which is not one of members, members:%v , leader:%v", members, l)
	}
	t.Fatal("impossible path of execution")
	return -1
}
