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
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const EtcdProcessBasePort = 20000

type ClientConnType int

const (
	ClientNonTLS ClientConnType = iota
	ClientTLS
	ClientTLSAndNonTLS
)

func NewConfigNoTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{ClusterSize: 3,
		InitialToken: "new",
	}
}

func NewConfigAutoTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:   3,
		IsPeerTLS:     true,
		IsPeerAutoTLS: true,
		InitialToken:  "new",
	}
}

func NewConfigTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  3,
		ClientTLS:    ClientTLS,
		IsPeerTLS:    true,
		InitialToken: "new",
	}
}

func NewConfigClientTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  3,
		ClientTLS:    ClientTLS,
		InitialToken: "new",
	}
}

func NewConfigClientBoth() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  1,
		ClientTLS:    ClientTLSAndNonTLS,
		InitialToken: "new",
	}
}

func NewConfigClientAutoTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:     1,
		IsClientAutoTLS: true,
		ClientTLS:       ClientTLS,
		InitialToken:    "new",
	}
}

func NewConfigPeerTLS() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  3,
		IsPeerTLS:    true,
		InitialToken: "new",
	}
}

func NewConfigClientTLSCertAuth() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:           1,
		ClientTLS:             ClientTLS,
		InitialToken:          "new",
		ClientCertAuthEnabled: true,
	}
}

func NewConfigClientTLSCertAuthWithNoCN() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:           1,
		ClientTLS:             ClientTLS,
		InitialToken:          "new",
		ClientCertAuthEnabled: true,
		NoCN:                  true,
	}
}

func NewConfigJWT() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  1,
		InitialToken: "new",
		AuthTokenOpts: "jwt,pub-key=" + path.Join(FixturesDir, "server.crt") +
			",priv-key=" + path.Join(FixturesDir, "server.key.insecure") + ",sign-method=RS256,ttl=1s",
	}
}

func ConfigStandalone(cfg EtcdProcessClusterConfig) *EtcdProcessClusterConfig {
	ret := cfg
	ret.ClusterSize = 1
	return &ret
}

type EtcdProcessCluster struct {
	lg    *zap.Logger
	Cfg   *EtcdProcessClusterConfig
	Procs []EtcdProcess
}

type EtcdProcessClusterConfig struct {
	ExecPath    string
	DataDirPath string
	KeepDataDir bool
	EnvVars     map[string]string

	ClusterSize int

	BaseScheme string
	BasePort   int

	MetricsURLScheme string

	SnapshotCount int // default is 10000

	ClientTLS             ClientConnType
	ClientCertAuthEnabled bool
	IsPeerTLS             bool
	IsPeerAutoTLS         bool
	IsClientAutoTLS       bool
	IsClientCRL           bool
	NoCN                  bool

	CipherSuites []string

	ForceNewCluster     bool
	InitialToken        string
	QuotaBackendBytes   int64
	NoStrictReconfig    bool
	EnableV2            bool
	InitialCorruptCheck bool
	AuthTokenOpts       string
	V2deprecation       string

	RollingStart bool
	Discovery    string
}

// NewEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new EtcdProcessCluster once all nodes are ready to accept client requests.
func NewEtcdProcessCluster(t testing.TB, cfg *EtcdProcessClusterConfig) (*EtcdProcessCluster, error) {
	epc, err := InitEtcdProcessCluster(t, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.RollingStart {
		if err := epc.RollingStart(); err != nil {
			return nil, fmt.Errorf("Cannot rolling-start: %v", err)
		}
	} else {
		if err := epc.Start(); err != nil {
			return nil, fmt.Errorf("Cannot start: %v", err)
		}
	}
	return epc, nil
}

// InitEtcdProcessCluster initializes a new cluster based on the given config.
// It doesn't start the cluster.
func InitEtcdProcessCluster(t testing.TB, cfg *EtcdProcessClusterConfig) (*EtcdProcessCluster, error) {
	SkipInShortMode(t)

	etcdCfgs := cfg.EtcdServerProcessConfigs(t)
	epc := &EtcdProcessCluster{
		Cfg:   cfg,
		lg:    zaptest.NewLogger(t),
		Procs: make([]EtcdProcess, cfg.ClusterSize),
	}

	// launch etcd processes
	for i := range etcdCfgs {
		proc, err := NewEtcdProcess(etcdCfgs[i])
		if err != nil {
			epc.Close()
			return nil, fmt.Errorf("cannot configure: %v", err)
		}
		epc.Procs[i] = proc
	}

	return epc, nil
}

func (cfg *EtcdProcessClusterConfig) ClientScheme() string {
	if cfg.ClientTLS == ClientTLS {
		return "https"
	}
	return "http"
}

func (cfg *EtcdProcessClusterConfig) PeerScheme() string {
	peerScheme := cfg.BaseScheme
	if peerScheme == "" {
		peerScheme = "http"
	}
	if cfg.IsPeerTLS {
		peerScheme += "s"
	}
	return peerScheme
}

func (cfg *EtcdProcessClusterConfig) EtcdServerProcessConfigs(tb testing.TB) []*EtcdServerProcessConfig {
	lg := zaptest.NewLogger(tb)

	if cfg.BasePort == 0 {
		cfg.BasePort = EtcdProcessBasePort
	}
	if cfg.ExecPath == "" {
		cfg.ExecPath = BinPath
	}
	if cfg.SnapshotCount == 0 {
		cfg.SnapshotCount = etcdserver.DefaultSnapshotCount
	}

	etcdCfgs := make([]*EtcdServerProcessConfig, cfg.ClusterSize)
	initialCluster := make([]string, cfg.ClusterSize)
	for i := 0; i < cfg.ClusterSize; i++ {
		var curls []string
		var curl, curltls string
		port := cfg.BasePort + 5*i
		curlHost := fmt.Sprintf("localhost:%d", port)

		switch cfg.ClientTLS {
		case ClientNonTLS, ClientTLS:
			curl = (&url.URL{Scheme: cfg.ClientScheme(), Host: curlHost}).String()
			curls = []string{curl}
		case ClientTLSAndNonTLS:
			curl = (&url.URL{Scheme: "http", Host: curlHost}).String()
			curltls = (&url.URL{Scheme: "https", Host: curlHost}).String()
			curls = []string{curl, curltls}
		}

		purl := url.URL{Scheme: cfg.PeerScheme(), Host: fmt.Sprintf("localhost:%d", port+1)}
		name := fmt.Sprintf("test-%d", i)
		dataDirPath := cfg.DataDirPath
		if cfg.DataDirPath == "" {
			dataDirPath = tb.TempDir()
		}
		initialCluster[i] = fmt.Sprintf("%s=%s", name, purl.String())

		args := []string{
			"--name", name,
			"--listen-client-urls", strings.Join(curls, ","),
			"--advertise-client-urls", strings.Join(curls, ","),
			"--listen-peer-urls", purl.String(),
			"--initial-advertise-peer-urls", purl.String(),
			"--initial-cluster-token", cfg.InitialToken,
			"--data-dir", dataDirPath,
			"--snapshot-count", fmt.Sprintf("%d", cfg.SnapshotCount),
		}

		if cfg.ForceNewCluster {
			args = append(args, "--force-new-cluster")
		}
		if cfg.QuotaBackendBytes > 0 {
			args = append(args,
				"--quota-backend-bytes", fmt.Sprintf("%d", cfg.QuotaBackendBytes),
			)
		}
		if cfg.NoStrictReconfig {
			args = append(args, "--strict-reconfig-check=false")
		}
		if cfg.EnableV2 {
			args = append(args, "--enable-v2")
		}
		if cfg.InitialCorruptCheck {
			args = append(args, "--experimental-initial-corrupt-check")
		}
		var murl string
		if cfg.MetricsURLScheme != "" {
			murl = (&url.URL{
				Scheme: cfg.MetricsURLScheme,
				Host:   fmt.Sprintf("localhost:%d", port+2),
			}).String()
			args = append(args, "--listen-metrics-urls", murl)
		}

		args = append(args, cfg.TlsArgs()...)

		if cfg.AuthTokenOpts != "" {
			args = append(args, "--auth-token", cfg.AuthTokenOpts)
		}

		if cfg.V2deprecation != "" {
			args = append(args, "--v2-deprecation", cfg.V2deprecation)
		}

		if cfg.Discovery != "" {
			args = append(args, "--discovery", cfg.Discovery)
		}

		etcdCfgs[i] = &EtcdServerProcessConfig{
			lg:           lg,
			ExecPath:     cfg.ExecPath,
			Args:         args,
			EnvVars:      cfg.EnvVars,
			TlsArgs:      cfg.TlsArgs(),
			DataDirPath:  dataDirPath,
			KeepDataDir:  cfg.KeepDataDir,
			Name:         name,
			Purl:         purl,
			Acurl:        curl,
			Murl:         murl,
			InitialToken: cfg.InitialToken,
		}
	}

	if cfg.Discovery == "" {
		for i := range etcdCfgs {
			initialClusterArgs := []string{"--initial-cluster", strings.Join(initialCluster, ",")}
			etcdCfgs[i].InitialCluster = strings.Join(initialCluster, ",")
			etcdCfgs[i].Args = append(etcdCfgs[i].Args, initialClusterArgs...)
		}
	}

	return etcdCfgs
}

func (cfg *EtcdProcessClusterConfig) TlsArgs() (args []string) {
	if cfg.ClientTLS != ClientNonTLS {
		if cfg.IsClientAutoTLS {
			args = append(args, "--auto-tls")
		} else {
			tlsClientArgs := []string{
				"--cert-file", CertPath,
				"--key-file", PrivateKeyPath,
				"--trusted-ca-file", CaPath,
			}
			args = append(args, tlsClientArgs...)

			if cfg.ClientCertAuthEnabled {
				args = append(args, "--client-cert-auth")
			}
		}
	}

	if cfg.IsPeerTLS {
		if cfg.IsPeerAutoTLS {
			args = append(args, "--peer-auto-tls")
		} else {
			tlsPeerArgs := []string{
				"--peer-cert-file", CertPath,
				"--peer-key-file", PrivateKeyPath,
				"--peer-trusted-ca-file", CaPath,
			}
			args = append(args, tlsPeerArgs...)
		}
	}

	if cfg.IsClientCRL {
		args = append(args, "--client-crl-file", CrlPath, "--client-cert-auth")
	}

	if len(cfg.CipherSuites) > 0 {
		args = append(args, "--cipher-suites", strings.Join(cfg.CipherSuites, ","))
	}

	return args
}

func (epc *EtcdProcessCluster) EndpointsV2() []string {
	return epc.Endpoints(func(ep EtcdProcess) []string { return ep.EndpointsV2() })
}

func (epc *EtcdProcessCluster) EndpointsV3() []string {
	return epc.Endpoints(func(ep EtcdProcess) []string { return ep.EndpointsV3() })
}

func (epc *EtcdProcessCluster) Endpoints(f func(ep EtcdProcess) []string) (ret []string) {
	for _, p := range epc.Procs {
		ret = append(ret, f(p)...)
	}
	return ret
}

func (epc *EtcdProcessCluster) Start() error {
	return epc.start(func(ep EtcdProcess) error { return ep.Start() })
}

func (epc *EtcdProcessCluster) RollingStart() error {
	return epc.rollingStart(func(ep EtcdProcess) error { return ep.Start() })
}

func (epc *EtcdProcessCluster) Restart() error {
	return epc.start(func(ep EtcdProcess) error { return ep.Restart() })
}

func (epc *EtcdProcessCluster) start(f func(ep EtcdProcess) error) error {
	readyC := make(chan error, len(epc.Procs))
	for i := range epc.Procs {
		go func(n int) { readyC <- f(epc.Procs[n]) }(i)
	}
	for range epc.Procs {
		if err := <-readyC; err != nil {
			epc.Close()
			return err
		}
	}
	return nil
}

func (epc *EtcdProcessCluster) rollingStart(f func(ep EtcdProcess) error) error {
	readyC := make(chan error, len(epc.Procs))
	for i := range epc.Procs {
		go func(n int) { readyC <- f(epc.Procs[n]) }(i)
		// make sure the servers do not start at the same time
		time.Sleep(time.Second)
	}
	for range epc.Procs {
		if err := <-readyC; err != nil {
			epc.Close()
			return err
		}
	}
	return nil
}

func (epc *EtcdProcessCluster) Stop() (err error) {
	for _, p := range epc.Procs {
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

func (epc *EtcdProcessCluster) Close() error {
	epc.lg.Info("closing test cluster...")
	err := epc.Stop()
	for _, p := range epc.Procs {
		// p is nil when NewEtcdProcess fails in the middle
		// Close still gets called to clean up test data
		if p == nil {
			continue
		}
		if cerr := p.Close(); cerr != nil {
			err = cerr
		}
	}
	epc.lg.Info("closed test cluster.")
	return err
}

func (epc *EtcdProcessCluster) WithStopSignal(sig os.Signal) (ret os.Signal) {
	for _, p := range epc.Procs {
		ret = p.WithStopSignal(sig)
	}
	return ret
}
