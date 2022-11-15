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
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/config"
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
	return DefaultConfig()
}

func NewConfigAutoTLS() *EtcdProcessClusterConfig {
	return NewConfig(
		WithIsPeerTLS(true),
		WithIsPeerAutoTLS(true),
	)
}

func NewConfigTLS() *EtcdProcessClusterConfig {
	return NewConfig(
		WithClientTLS(ClientTLS),
		WithIsPeerTLS(true),
	)
}

func NewConfigClientTLS() *EtcdProcessClusterConfig {
	return NewConfig(WithClientTLS(ClientTLS))
}

func NewConfigClientAutoTLS() *EtcdProcessClusterConfig {
	return NewConfig(
		WithClusterSize(1),
		WithIsClientAutoTLS(true),
		WithClientTLS(ClientTLS),
	)
}

func NewConfigPeerTLS() *EtcdProcessClusterConfig {
	return NewConfig(
		WithIsPeerTLS(true),
	)
}

func NewConfigClientTLSCertAuth() *EtcdProcessClusterConfig {
	return NewConfig(
		WithClusterSize(1),
		WithClientTLS(ClientTLS),
		WithClientCertAuthEnabled(true),
	)
}

func NewConfigClientTLSCertAuthWithNoCN() *EtcdProcessClusterConfig {
	return NewConfig(
		WithClusterSize(1),
		WithClientTLS(ClientTLS),
		WithClientCertAuthEnabled(true),
		WithNoCN(true),
	)
}

func NewConfigJWT() *EtcdProcessClusterConfig {
	return NewConfig(
		WithClusterSize(1),
		WithAuthTokenOpts("jwt,pub-key="+path.Join(FixturesDir, "server.crt")+
			",priv-key="+path.Join(FixturesDir, "server.key.insecure")+",sign-method=RS256,ttl=1s"),
	)
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
	Logger  *zap.Logger
	Version ClusterVersion
	// DataDirPath specifies the data-dir for the members. If test cases
	// do not specify `DataDirPath`, then e2e framework creates a
	// temporary directory for each member; otherwise, it creates a
	// subdirectory (e.g. member-0, member-1 and member-2) under the given
	// `DataDirPath` for each member.
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
	GoFailEnabled           bool
	CompactionBatchLimit    int
}

func DefaultConfig() *EtcdProcessClusterConfig {
	return &EtcdProcessClusterConfig{
		ClusterSize:  3,
		InitialToken: "new",
	}
}

func NewConfig(opts ...EPClusterOption) *EtcdProcessClusterConfig {
	c := DefaultConfig()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type EPClusterOption func(*EtcdProcessClusterConfig)

func WithConfig(cfg *EtcdProcessClusterConfig) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { *c = *cfg }
}

func WithVersion(version ClusterVersion) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.Version = version }
}

func WithDataDirPath(path string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.DataDirPath = path }
}

func WithKeepDataDir(keep bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.KeepDataDir = keep }
}

func WithSnapshotCount(count int) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.SnapshotCount = count }
}

func WithClusterSize(size int) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.ClusterSize = size }
}

func WithBaseScheme(scheme string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.BaseScheme = scheme }
}

func WithBasePort(port int) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.BasePort = port }
}

func WithClientTLS(clientTLS ClientConnType) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.ClientTLS = clientTLS }
}

func WithClientCertAuthEnabled(enabled bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.ClientCertAuthEnabled = enabled }
}

func WithIsPeerTLS(isPeerTLS bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.IsPeerTLS = isPeerTLS }
}

func WithIsPeerAutoTLS(isPeerAutoTLS bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.IsPeerAutoTLS = isPeerAutoTLS }
}

func WithIsClientAutoTLS(isClientAutoTLS bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.IsClientAutoTLS = isClientAutoTLS }
}

func WithIsClientCRL(isClientCRL bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.IsClientCRL = isClientCRL }
}

func WithNoCN(noCN bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.NoCN = noCN }
}

func WithQuotaBackendBytes(bytes int64) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.QuotaBackendBytes = bytes }
}

func WithDisableStrictReconfigCheck(disable bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.DisableStrictReconfigCheck = disable }
}

func WithEnableV2(enable bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.EnableV2 = enable }
}

func WithAuthTokenOpts(token string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.AuthTokenOpts = token }
}

func WithRollingStart(rolling bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.RollingStart = rolling }
}

func WithDiscovery(discovery string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.Discovery = discovery }
}

func WithDiscoveryEndpoints(endpoints []string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.DiscoveryEndpoints = endpoints }
}

func WithDiscoveryToken(token string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.DiscoveryToken = token }
}

func WithLogLevel(level string) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.LogLevel = level }
}

func WithCorruptCheckTime(time time.Duration) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.CorruptCheckTime = time }
}

func WithCompactHashCheckEnabled(enabled bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.CompactHashCheckEnabled = enabled }
}

func WithCompactHashCheckTime(time time.Duration) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.CompactHashCheckTime = time }
}

func WithGoFailEnabled(enabled bool) EPClusterOption {
	return func(c *EtcdProcessClusterConfig) { c.GoFailEnabled = enabled }
}

// NewEtcdProcessCluster launches a new cluster from etcd processes, returning
// a new EtcdProcessCluster once all nodes are ready to accept client requests.
func NewEtcdProcessCluster(ctx context.Context, t testing.TB, cfg *EtcdProcessClusterConfig, opts ...EPClusterOption) (*EtcdProcessCluster, error) {
	if cfg == nil {
		cfg = NewConfig(opts...)
	} else {
		for _, opt := range opts {
			opt(cfg)
		}
	}
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
	} else {
		// When test cases specify the DataDirPath and there are more than
		// one member in the cluster, we need to create a subdirectory for
		// each member to avoid conflict.
		// We also create a subdirectory for one-member cluster, because we
		// support dynamically adding new member.
		dataDirPath = filepath.Join(cfg.DataDirPath, fmt.Sprintf("member-%d", i))
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
	if cfg.CompactionBatchLimit != 0 {
		args = append(args, "--experimental-compaction-batch-limit", fmt.Sprintf("%d", cfg.CompactionBatchLimit))
	}
	envVars := map[string]string{}
	for key, value := range cfg.EnvVars {
		envVars[key] = value
	}
	var gofailPort int
	if cfg.GoFailEnabled {
		gofailPort = (i+1)*10000 + 2381
		envVars["GOFAIL_HTTP"] = fmt.Sprintf("127.0.0.1:%d", gofailPort)
	}

	var execPath string
	switch cfg.Version {
	case CurrentVersion:
		execPath = BinPath.Etcd
	case MinorityLastVersion:
		if i <= cfg.ClusterSize/2 {
			execPath = BinPath.Etcd
		} else {
			execPath = BinPath.EtcdLastRelease
		}
	case QuorumLastVersion:
		if i <= cfg.ClusterSize/2 {
			execPath = BinPath.EtcdLastRelease
		} else {
			execPath = BinPath.Etcd
		}
	case LastVersion:
		execPath = BinPath.EtcdLastRelease
	default:
		panic(fmt.Sprintf("Unknown cluster version %v", cfg.Version))
	}

	return &EtcdServerProcessConfig{
		lg:           cfg.Logger,
		ExecPath:     execPath,
		Args:         args,
		EnvVars:      envVars,
		TlsArgs:      cfg.TlsArgs(),
		DataDirPath:  dataDirPath,
		KeepDataDir:  cfg.KeepDataDir,
		Name:         name,
		Purl:         purl,
		Acurl:        curl,
		Murl:         murl,
		InitialToken: cfg.InitialToken,
		GoFailPort:   gofailPort,
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

func (epc *EtcdProcessCluster) CloseProc(ctx context.Context, finder func(EtcdProcess) bool, opts ...config.ClientOption) error {
	procIndex := -1
	if finder != nil {
		for i := range epc.Procs {
			if finder(epc.Procs[i]) {
				procIndex = i
				break
			}
		}
	} else {
		procIndex = len(epc.Procs) - 1
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

	memberCtl := epc.Client(opts...)
	memberList, err := memberCtl.MemberList(ctx)
	if err != nil {
		return fmt.Errorf("failed to get member list: %w", err)
	}

	memberID, err := findMemberIDByEndpoint(memberList.Members, proc.Config().Acurl)
	if err != nil {
		return fmt.Errorf("failed to find member ID: %w", err)
	}

	memberRemoved := false
	for i := 0; i < 10; i++ {
		_, err := memberCtl.MemberRemove(ctx, memberID)
		if err != nil && strings.Contains(err.Error(), "member not found") {
			memberRemoved = true
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	if !memberRemoved {
		return errors.New("failed to remove member after 10 tries")
	}

	epc.lg.Info("successfully removed member", zap.String("acurl", proc.Config().Acurl))

	// Then stop process
	return proc.Close()
}

func (epc *EtcdProcessCluster) StartNewProc(ctx context.Context, cfg *EtcdProcessClusterConfig, tb testing.TB, opts ...config.ClientOption) error {
	var serverCfg *EtcdServerProcessConfig
	if cfg != nil {
		serverCfg = cfg.EtcdServerProcessConfig(tb, epc.nextSeq)
	} else {
		serverCfg = epc.Cfg.EtcdServerProcessConfig(tb, epc.nextSeq)
	}

	epc.nextSeq++

	initialCluster := []string{
		fmt.Sprintf("%s=%s", serverCfg.Name, serverCfg.Purl.String()),
	}
	for _, p := range epc.Procs {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=%s", p.Config().Name, p.Config().Purl.String()))
	}

	epc.Cfg.SetInitialOrDiscovery(serverCfg, initialCluster, "existing")

	// First add new member to cluster
	memberCtl := epc.Client(opts...)
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

func (epc *EtcdProcessCluster) Client(opts ...config.ClientOption) *EtcdctlV3 {
	etcdctl, err := NewEtcdctl(epc.Cfg, epc.EndpointsV3(), opts...)
	if err != nil {
		panic(err)
	}
	return etcdctl
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
