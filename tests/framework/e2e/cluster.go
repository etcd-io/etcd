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
	"net/url"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/server/v3/etcdserver"
)

const EtcdProcessBasePort = 20000

type ClientConnType int

const (
	ClientNonTLS ClientConnType = iota
	ClientTLS
	ClientTLSAndNonTLS
)

// allow alphanumerics, underscores and dashes
var testNameCleanRegex = regexp.MustCompile(`[^a-zA-Z0-9 \-_]+`)

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
	lg      *zap.Logger
	Cfg     *EtcdProcessClusterConfig
	Procs   []EtcdProcess
	nextSeq int // sequence number of the next etcd process (if it will be required)
}

type EtcdProcessClusterConfig struct {
	Logger      *zap.Logger
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

	ForceNewCluster            bool
	InitialToken               string
	QuotaBackendBytes          int64
	DisableStrictReconfigCheck bool
	EnableV2                   bool
	InitialCorruptCheck        bool
	AuthTokenOpts              string
	V2deprecation              string

	RollingStart bool

	Discovery string // v2 discovery

	DiscoveryEndpoints []string // v3 discovery
	DiscoveryToken     string
	LogLevel           string

	MaxConcurrentStreams    uint32 // default is math.MaxUint32
	CorruptCheckTime        time.Duration
	CompactHashCheckEnabled bool
	CompactHashCheckTime    time.Duration
}

// NewEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new EtcdProcessCluster once all nodes are ready to accept client requests.
func NewEtcdProcessCluster(ctx context.Context, t testing.TB, cfg *EtcdProcessClusterConfig) (*EtcdProcessCluster, error) {
	epc, err := InitEtcdProcessCluster(t, cfg)
	if err != nil {
		return nil, err
	}

	return StartEtcdProcessCluster(ctx, epc, cfg)
}

// InitEtcdProcessCluster initializes a new cluster based on the given config.
// It doesn't start the cluster.
func InitEtcdProcessCluster(t testing.TB, cfg *EtcdProcessClusterConfig) (*EtcdProcessCluster, error) {
	SkipInShortMode(t)

	if cfg.Logger == nil {
		cfg.Logger = zaptest.NewLogger(t)
	}
	if cfg.BasePort == 0 {
		cfg.BasePort = EtcdProcessBasePort
	}
	if cfg.ExecPath == "" {
		cfg.ExecPath = BinPath.Etcd
	}
	if cfg.SnapshotCount == 0 {
		cfg.SnapshotCount = etcdserver.DefaultSnapshotCount
	}

	etcdCfgs := cfg.EtcdAllServerProcessConfigs(t)
	epc := &EtcdProcessCluster{
		Cfg:     cfg,
		lg:      zaptest.NewLogger(t),
		Procs:   make([]EtcdProcess, cfg.ClusterSize),
		nextSeq: cfg.ClusterSize,
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

// StartEtcdProcessCluster launches a new cluster from etcd processes.
func StartEtcdProcessCluster(ctx context.Context, epc *EtcdProcessCluster, cfg *EtcdProcessClusterConfig) (*EtcdProcessCluster, error) {
	if cfg.RollingStart {
		if err := epc.RollingStart(ctx); err != nil {
			return nil, fmt.Errorf("cannot rolling-start: %v", err)
		}
	} else {
		if err := epc.Start(ctx); err != nil {
			return nil, fmt.Errorf("cannot start: %v", err)
		}
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

func (cfg *EtcdProcessClusterConfig) EtcdAllServerProcessConfigs(tb testing.TB) []*EtcdServerProcessConfig {
	etcdCfgs := make([]*EtcdServerProcessConfig, cfg.ClusterSize)
	initialCluster := make([]string, cfg.ClusterSize)

	for i := 0; i < cfg.ClusterSize; i++ {
		etcdCfgs[i] = cfg.EtcdServerProcessConfig(tb, i)
		initialCluster[i] = fmt.Sprintf("%s=%s", etcdCfgs[i].Name, etcdCfgs[i].Purl.String())
	}

	for i := range etcdCfgs {
		cfg.SetInitialOrDiscovery(etcdCfgs[i], initialCluster, "new")
	}

	return etcdCfgs
}

func (cfg *EtcdProcessClusterConfig) SetInitialOrDiscovery(serverCfg *EtcdServerProcessConfig, initialCluster []string, initialClusterState string) {
	if cfg.Discovery == "" && len(cfg.DiscoveryEndpoints) == 0 {
		serverCfg.InitialCluster = strings.Join(initialCluster, ",")
		serverCfg.Args = append(serverCfg.Args, "--initial-cluster", serverCfg.InitialCluster)
		serverCfg.Args = append(serverCfg.Args, "--initial-cluster-state", initialClusterState)
	}

	if len(cfg.DiscoveryEndpoints) > 0 {
		serverCfg.Args = append(serverCfg.Args, fmt.Sprintf("--discovery-token=%s", cfg.DiscoveryToken))
		serverCfg.Args = append(serverCfg.Args, fmt.Sprintf("--discovery-endpoints=%s", strings.Join(cfg.DiscoveryEndpoints, ",")))
	}
}

func (cfg *EtcdProcessClusterConfig) EtcdServerProcessConfig(tb testing.TB, i int) *EtcdServerProcessConfig {
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

	name := fmt.Sprintf("%s-test-%d", testNameCleanRegex.ReplaceAllString(tb.Name(), ""), i)
	dataDirPath := cfg.DataDirPath
	if cfg.DataDirPath == "" {
		dataDirPath = tb.TempDir()
	}

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
	if cfg.DisableStrictReconfigCheck {
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

	if cfg.LogLevel != "" {
		args = append(args, "--log-level", cfg.LogLevel)
	}

	if cfg.MaxConcurrentStreams != 0 {
		args = append(args, "--max-concurrent-streams", fmt.Sprintf("%d", cfg.MaxConcurrentStreams))
	}

	if cfg.CorruptCheckTime != 0 {
		args = append(args, "--experimental-corrupt-check-time", fmt.Sprintf("%s", cfg.CorruptCheckTime))
	}
	if cfg.CompactHashCheckEnabled {
		args = append(args, "--experimental-compact-hash-check-enabled")
	}
	if cfg.CompactHashCheckTime != 0 {
		args = append(args, "--experimental-compact-hash-check-time", cfg.CompactHashCheckTime.String())
	}

	return &EtcdServerProcessConfig{
		lg:           cfg.Logger,
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

func (epc *EtcdProcessCluster) CloseProc(ctx context.Context, finder func(EtcdProcess) bool) error {
	procIndex := -1
	for i := range epc.Procs {
		if finder(epc.Procs[i]) {
			procIndex = i
			break
		}
	}

	if procIndex == -1 {
		return fmt.Errorf("no process found to stop")
	}

	proc := epc.Procs[procIndex]
	epc.Procs = append(epc.Procs[:procIndex], epc.Procs[procIndex+1:]...)

	if proc == nil {
		return nil
	}

	// First remove member from the cluster

	memberCtl := epc.Client()
	memberList, err := memberCtl.MemberList(ctx)
	if err != nil {
		return fmt.Errorf("failed to get member list: %w", err)
	}

	memberID, err := findMemberIDByEndpoint(memberList.Members, proc.Config().Acurl)
	if err != nil {
		return fmt.Errorf("failed to find member ID: %w", err)
	}

	for i := 0; i < 10; i++ {
		_, err = memberCtl.MemberRemove(ctx, memberID)
		if err != nil && strings.Contains(err.Error(), rpctypes.ErrGRPCUnhealthy.Error()) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	// Then stop process
	return proc.Close()
}

func (epc *EtcdProcessCluster) StartNewProc(ctx context.Context, tb testing.TB) error {
	serverCfg := epc.Cfg.EtcdServerProcessConfig(tb, epc.nextSeq)
	epc.nextSeq++

	initialCluster := []string{
		fmt.Sprintf("%s=%s", serverCfg.Name, serverCfg.Purl.String()),
	}
	for _, p := range epc.Procs {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=%s", p.Config().Name, p.Config().Purl.String()))
	}

	epc.Cfg.SetInitialOrDiscovery(serverCfg, initialCluster, "existing")

	// First add new member to cluster
	memberCtl := epc.Client()
	_, err := memberCtl.MemberAdd(ctx, serverCfg.Name, []string{serverCfg.Purl.String()})
	if err != nil {
		return fmt.Errorf("failed to add new member: %w", err)
	}

	// Then start process
	proc, err := NewEtcdProcess(serverCfg)
	if err != nil {
		epc.Close()
		return fmt.Errorf("cannot configure: %v", err)
	}

	epc.Procs = append(epc.Procs, proc)

	return proc.Start(ctx)
}

func (epc *EtcdProcessCluster) Start(ctx context.Context) error {
	return epc.start(func(ep EtcdProcess) error { return ep.Start(ctx) })
}

func (epc *EtcdProcessCluster) RollingStart(ctx context.Context) error {
	return epc.rollingStart(func(ep EtcdProcess) error { return ep.Start(ctx) })
}

func (epc *EtcdProcessCluster) Restart(ctx context.Context) error {
	return epc.start(func(ep EtcdProcess) error { return ep.Restart(ctx) })
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

func (epc *EtcdProcessCluster) Client() *EtcdctlV3 {
	return NewEtcdctl(epc.Cfg, epc.EndpointsV3())
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

func findMemberIDByEndpoint(members []*etcdserverpb.Member, endpoint string) (uint64, error) {
	for _, m := range members {
		if m.ClientURLs[0] == endpoint {
			return m.ID, nil
		}
	}

	return 0, fmt.Errorf("member not found")
}
