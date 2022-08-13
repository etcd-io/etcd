// Copyright 2015 The etcd Authors
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

// Every change should be reflected on help.go as well.

package etcdmain

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/flags"
	cconfig "go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"

	"go.uber.org/zap"
)

var (
	fallbackFlagExit  = "exit"
	fallbackFlagProxy = "proxy"

	ignored = []string{
		"cluster-active-size",
		"cluster-remove-delay",
		"cluster-sync-interval",
		"config",
		"force",
		"max-result-buffer",
		"max-retry-attempts",
		"peer-heartbeat-interval",
		"peer-election-timeout",
		"retry-interval",
		"snapshot",
		"v",
		"vv",
		// for coverage testing
		"test.coverprofile",
		"test.outputdir",
	}
)

// config holds the config for a command line invocation of etcd
type config struct {
	ec           embed.Config
	cf           configFlags
	configFile   string
	printVersion bool
	ignored      []string
}

// configFlags has the set of flags used for command line parsing a Config
type configFlags struct {
	flagSet       *flag.FlagSet
	clusterState  *flags.SelectiveStringValue
	fallback      *flags.SelectiveStringValue
	v2deprecation *flags.SelectiveStringsValue
}

func newConfig() *config {
	cfg := &config{
		ec:      *embed.NewConfig(),
		ignored: ignored,
	}
	cfg.cf = configFlags{
		flagSet: flag.NewFlagSet("etcd", flag.ContinueOnError),
		clusterState: flags.NewSelectiveStringValue(
			embed.ClusterStateFlagNew,
			embed.ClusterStateFlagExisting,
		),
		fallback: flags.NewSelectiveStringValue(
			fallbackFlagExit,
			fallbackFlagProxy,
		),
		v2deprecation: flags.NewSelectiveStringsValue(
			string(cconfig.V2_DEPR_1_WRITE_ONLY),
			string(cconfig.V2_DEPR_1_WRITE_ONLY_DROP),
			string(cconfig.V2_DEPR_2_GONE)),
	}

	fs := cfg.cf.flagSet
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usageline)
	}

	fs.StringVar(&cfg.configFile, "config-file", "", "Path to the server configuration file. Note that if a configuration file is provided, other command line flags and environment variables will be ignored.")

	// member
	fs.StringVar(&cfg.ec.Dir, "data-dir", cfg.ec.Dir, "Path to the data directory.")
	fs.StringVar(&cfg.ec.WalDir, "wal-dir", cfg.ec.WalDir, "Path to the dedicated wal directory.")
	fs.Var(
		flags.NewUniqueURLsWithExceptions(embed.DefaultListenPeerURLs, ""),
		"listen-peer-urls",
		"List of URLs to listen on for peer traffic.",
	)
	fs.Var(
		flags.NewUniqueURLsWithExceptions(embed.DefaultListenClientURLs, ""), "listen-client-urls",
		"List of URLs to listen on for client traffic.",
	)
	fs.Var(
		flags.NewUniqueURLsWithExceptions("", ""),
		"listen-metrics-urls",
		"List of URLs to listen on for the metrics and health endpoints.",
	)
	fs.UintVar(&cfg.ec.MaxSnapFiles, "max-snapshots", cfg.ec.MaxSnapFiles, "Maximum number of snapshot files to retain (0 is unlimited).")
	fs.UintVar(&cfg.ec.MaxWalFiles, "max-wals", cfg.ec.MaxWalFiles, "Maximum number of wal files to retain (0 is unlimited).")
	fs.StringVar(&cfg.ec.Name, "name", cfg.ec.Name, "Human-readable name for this member.")
	fs.Uint64Var(&cfg.ec.SnapshotCount, "snapshot-count", cfg.ec.SnapshotCount, "Number of committed transactions to trigger a snapshot to disk.")
	fs.UintVar(&cfg.ec.TickMs, "heartbeat-interval", cfg.ec.TickMs, "Time (in milliseconds) of a heartbeat interval.")
	fs.UintVar(&cfg.ec.ElectionMs, "election-timeout", cfg.ec.ElectionMs, "Time (in milliseconds) for an election to timeout.")
	fs.BoolVar(&cfg.ec.InitialElectionTickAdvance, "initial-election-tick-advance", cfg.ec.InitialElectionTickAdvance, "Whether to fast-forward initial election ticks on boot for faster election.")
	fs.Int64Var(&cfg.ec.QuotaBackendBytes, "quota-backend-bytes", cfg.ec.QuotaBackendBytes, "Raise alarms when backend size exceeds the given quota. 0 means use the default quota.")
	fs.StringVar(&cfg.ec.BackendFreelistType, "backend-bbolt-freelist-type", cfg.ec.BackendFreelistType, "BackendFreelistType specifies the type of freelist that boltdb backend uses(array and map are supported types)")
	fs.DurationVar(&cfg.ec.BackendBatchInterval, "backend-batch-interval", cfg.ec.BackendBatchInterval, "BackendBatchInterval is the maximum time before commit the backend transaction.")
	fs.IntVar(&cfg.ec.BackendBatchLimit, "backend-batch-limit", cfg.ec.BackendBatchLimit, "BackendBatchLimit is the maximum operations before commit the backend transaction.")
	fs.UintVar(&cfg.ec.MaxTxnOps, "max-txn-ops", cfg.ec.MaxTxnOps, "Maximum number of operations permitted in a transaction.")
	fs.UintVar(&cfg.ec.MaxRequestBytes, "max-request-bytes", cfg.ec.MaxRequestBytes, "Maximum client request size in bytes the server will accept.")
	fs.DurationVar(&cfg.ec.GRPCKeepAliveMinTime, "grpc-keepalive-min-time", cfg.ec.GRPCKeepAliveMinTime, "Minimum interval duration that a client should wait before pinging server.")
	fs.DurationVar(&cfg.ec.GRPCKeepAliveInterval, "grpc-keepalive-interval", cfg.ec.GRPCKeepAliveInterval, "Frequency duration of server-to-client ping to check if a connection is alive (0 to disable).")
	fs.DurationVar(&cfg.ec.GRPCKeepAliveTimeout, "grpc-keepalive-timeout", cfg.ec.GRPCKeepAliveTimeout, "Additional duration of wait before closing a non-responsive connection (0 to disable).")
	fs.BoolVar(&cfg.ec.SocketOpts.ReusePort, "socket-reuse-port", cfg.ec.SocketOpts.ReusePort, "Enable to set socket option SO_REUSEPORT on listeners allowing rebinding of a port already in use.")
	fs.BoolVar(&cfg.ec.SocketOpts.ReuseAddress, "socket-reuse-address", cfg.ec.SocketOpts.ReuseAddress, "Enable to set socket option SO_REUSEADDR on listeners allowing binding to an address in `TIME_WAIT` state.")

	fs.Var(flags.NewUint32Value(cfg.ec.MaxConcurrentStreams), "max-concurrent-streams", "Maximum concurrent streams that each client can open at a time.")

	// raft connection timeouts
	fs.DurationVar(&rafthttp.ConnReadTimeout, "raft-read-timeout", rafthttp.DefaultConnReadTimeout, "Read timeout set on each rafthttp connection")
	fs.DurationVar(&rafthttp.ConnWriteTimeout, "raft-write-timeout", rafthttp.DefaultConnWriteTimeout, "Write timeout set on each rafthttp connection")

	// clustering
	fs.Var(
		flags.NewUniqueURLsWithExceptions(embed.DefaultInitialAdvertisePeerURLs, ""),
		"initial-advertise-peer-urls",
		"List of this member's peer URLs to advertise to the rest of the cluster.",
	)
	fs.Var(
		flags.NewUniqueURLsWithExceptions(embed.DefaultAdvertiseClientURLs, ""),
		"advertise-client-urls",
		"List of this member's client URLs to advertise to the public.",
	)

	fs.StringVar(&cfg.ec.Durl, "discovery", cfg.ec.Durl, "Discovery URL used to bootstrap the cluster for v2 discovery. Will be deprecated in v3.7, and be decommissioned in v3.8.")
	fs.Var(cfg.cf.fallback, "discovery-fallback", fmt.Sprintf("Valid values include %q", cfg.cf.fallback.Valids()))

	fs.Var(
		flags.NewUniqueStringsValue(""),
		"discovery-endpoints",
		"V3 discovery: List of gRPC endpoints of the discovery service.",
	)
	fs.StringVar(&cfg.ec.DiscoveryCfg.Token, "discovery-token", "", "V3 discovery: discovery token for the etcd cluster to be bootstrapped.")
	fs.DurationVar(&cfg.ec.DiscoveryCfg.DialTimeout, "discovery-dial-timeout", cfg.ec.DiscoveryCfg.DialTimeout, "V3 discovery: dial timeout for client connections.")
	fs.DurationVar(&cfg.ec.DiscoveryCfg.RequestTimeout, "discovery-request-timeout", cfg.ec.DiscoveryCfg.RequestTimeout, "V3 discovery: timeout for discovery requests (excluding dial timeout).")
	fs.DurationVar(&cfg.ec.DiscoveryCfg.KeepAliveTime, "discovery-keepalive-time", cfg.ec.DiscoveryCfg.KeepAliveTime, "V3 discovery: keepalive time for client connections.")
	fs.DurationVar(&cfg.ec.DiscoveryCfg.KeepAliveTimeout, "discovery-keepalive-timeout", cfg.ec.DiscoveryCfg.KeepAliveTimeout, "V3 discovery: keepalive timeout for client connections.")
	fs.BoolVar(&cfg.ec.DiscoveryCfg.Secure.InsecureTransport, "discovery-insecure-transport", true, "V3 discovery: disable transport security for client connections.")
	fs.BoolVar(&cfg.ec.DiscoveryCfg.Secure.InsecureSkipVerify, "discovery-insecure-skip-tls-verify", false, "V3 discovery: skip server certificate verification (CAUTION: this option should be enabled only for testing purposes).")
	fs.StringVar(&cfg.ec.DiscoveryCfg.Secure.Cert, "discovery-cert", "", "V3 discovery: identify secure client using this TLS certificate file.")
	fs.StringVar(&cfg.ec.DiscoveryCfg.Secure.Key, "discovery-key", "", "V3 discovery: identify secure client using this TLS key file.")
	fs.StringVar(&cfg.ec.DiscoveryCfg.Secure.Cacert, "discovery-cacert", "", "V3 discovery: verify certificates of TLS-enabled secure servers using this CA bundle.")
	fs.StringVar(&cfg.ec.DiscoveryCfg.Auth.Username, "discovery-user", "", "V3 discovery: username[:password] for authentication (prompt if password is not supplied).")
	fs.StringVar(&cfg.ec.DiscoveryCfg.Auth.Password, "discovery-password", "", "V3 discovery: password for authentication (if this option is used, --user option shouldn't include password).")

	fs.StringVar(&cfg.ec.Dproxy, "discovery-proxy", cfg.ec.Dproxy, "HTTP proxy to use for traffic to discovery service. Will be deprecated in v3.7, and be decommissioned in v3.8.")
	fs.StringVar(&cfg.ec.DNSCluster, "discovery-srv", cfg.ec.DNSCluster, "DNS domain used to bootstrap initial cluster.")
	fs.StringVar(&cfg.ec.DNSClusterServiceName, "discovery-srv-name", cfg.ec.DNSClusterServiceName, "Service name to query when using DNS discovery.")
	fs.StringVar(&cfg.ec.InitialCluster, "initial-cluster", cfg.ec.InitialCluster, "Initial cluster configuration for bootstrapping.")
	fs.StringVar(&cfg.ec.InitialClusterToken, "initial-cluster-token", cfg.ec.InitialClusterToken, "Initial cluster token for the etcd cluster during bootstrap.")
	fs.Var(cfg.cf.clusterState, "initial-cluster-state", "Initial cluster state ('new' or 'existing').")

	fs.BoolVar(&cfg.ec.StrictReconfigCheck, "strict-reconfig-check", cfg.ec.StrictReconfigCheck, "Reject reconfiguration requests that would cause quorum loss.")

	fs.BoolVar(&cfg.ec.PreVote, "pre-vote", cfg.ec.PreVote, "Enable to run an additional Raft election phase.")

	fs.Var(cfg.cf.v2deprecation, "v2-deprecation", fmt.Sprintf("v2store deprecation stage: %q. ", cfg.cf.v2deprecation.Valids()))

	// security
	fs.StringVar(&cfg.ec.ClientTLSInfo.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.ClientCertFile, "client-cert-file", "", "Path to an explicit peer client TLS cert file otherwise cert file will be used when client auth is required.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.ClientKeyFile, "client-key-file", "", "Path to an explicit peer client TLS key file otherwise key file will be used when client auth is required.")
	fs.BoolVar(&cfg.ec.ClientTLSInfo.ClientCertAuth, "client-cert-auth", false, "Enable client cert authentication.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.CRLFile, "client-crl-file", "", "Path to the client certificate revocation list file.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.AllowedHostname, "client-cert-allowed-hostname", "", "Allowed TLS hostname for client cert authentication.")
	fs.StringVar(&cfg.ec.ClientTLSInfo.TrustedCAFile, "trusted-ca-file", "", "Path to the client server TLS trusted CA cert file.")
	fs.BoolVar(&cfg.ec.ClientAutoTLS, "auto-tls", false, "Client TLS using generated certificates")
	fs.StringVar(&cfg.ec.PeerTLSInfo.CertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.KeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.ClientCertFile, "peer-client-cert-file", "", "Path to an explicit peer client TLS cert file otherwise peer cert file will be used when client auth is required.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.ClientKeyFile, "peer-client-key-file", "", "Path to an explicit peer client TLS key file otherwise peer key file will be used when client auth is required.")
	fs.BoolVar(&cfg.ec.PeerTLSInfo.ClientCertAuth, "peer-client-cert-auth", false, "Enable peer client cert authentication.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.TrustedCAFile, "peer-trusted-ca-file", "", "Path to the peer server TLS trusted CA file.")
	fs.BoolVar(&cfg.ec.PeerAutoTLS, "peer-auto-tls", false, "Peer TLS using generated certificates")
	fs.UintVar(&cfg.ec.SelfSignedCertValidity, "self-signed-cert-validity", 1, "The validity period of the client and peer certificates, unit is year")
	fs.StringVar(&cfg.ec.PeerTLSInfo.CRLFile, "peer-crl-file", "", "Path to the peer certificate revocation list file.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.AllowedCN, "peer-cert-allowed-cn", "", "Allowed CN for inter peer authentication.")
	fs.StringVar(&cfg.ec.PeerTLSInfo.AllowedHostname, "peer-cert-allowed-hostname", "", "Allowed TLS hostname for inter peer authentication.")
	fs.Var(flags.NewStringsValue(""), "cipher-suites", "Comma-separated list of supported TLS cipher suites between client/server and peers (empty will be auto-populated by Go).")
	fs.BoolVar(&cfg.ec.PeerTLSInfo.SkipClientSANVerify, "experimental-peer-skip-client-san-verification", false, "Skip verification of SAN field in client certificate for peer connections.")

	fs.Var(
		flags.NewUniqueURLsWithExceptions("*", "*"),
		"cors",
		"Comma-separated white list of origins for CORS, or cross-origin resource sharing, (empty or * means allow all)",
	)
	fs.Var(flags.NewUniqueStringsValue("*"), "host-whitelist", "Comma-separated acceptable hostnames from HTTP client requests, if server is not secure (empty means allow all).")

	// logging
	fs.StringVar(&cfg.ec.Logger, "logger", "zap", "Currently only supports 'zap' for structured logging.")
	fs.Var(flags.NewUniqueStringsValue(embed.DefaultLogOutput), "log-outputs", "Specify 'stdout' or 'stderr' to skip journald logging even when running under systemd, or list of comma separated output targets.")
	fs.StringVar(&cfg.ec.LogLevel, "log-level", logutil.DefaultLogLevel, "Configures log level. Only supports debug, info, warn, error, panic, or fatal. Default 'info'.")
	fs.StringVar(&cfg.ec.LogFormat, "log-format", logutil.DefaultLogFormat, "Configures log format. Only supports json, console. Default is 'json'.")
	fs.BoolVar(&cfg.ec.EnableLogRotation, "enable-log-rotation", false, "Enable log rotation of a single log-outputs file target.")
	fs.StringVar(&cfg.ec.LogRotationConfigJSON, "log-rotation-config-json", embed.DefaultLogRotationConfig, "Configures log rotation if enabled with a JSON logger config. Default: MaxSize=100(MB), MaxAge=0(days,no limit), MaxBackups=0(no limit), LocalTime=false(UTC), Compress=false(gzip)")

	// version
	fs.BoolVar(&cfg.printVersion, "version", false, "Print the version and exit.")

	fs.StringVar(&cfg.ec.AutoCompactionRetention, "auto-compaction-retention", "0", "Auto compaction retention for mvcc key value store. 0 means disable auto compaction.")
	fs.StringVar(&cfg.ec.AutoCompactionMode, "auto-compaction-mode", "periodic", "interpret 'auto-compaction-retention' one of: periodic|revision. 'periodic' for duration based retention, defaulting to hours if no time unit is provided (e.g. '5m'). 'revision' for revision number based retention.")

	// pprof profiler via HTTP
	fs.BoolVar(&cfg.ec.EnablePprof, "enable-pprof", false, "Enable runtime profiling data via HTTP server. Address is at client URL + \"/debug/pprof/\"")

	// additional metrics
	fs.StringVar(&cfg.ec.Metrics, "metrics", cfg.ec.Metrics, "Set level of detail for exported metrics, specify 'extensive' to include server side grpc histogram metrics")

	// experimental distributed tracing
	fs.BoolVar(&cfg.ec.ExperimentalEnableDistributedTracing, "experimental-enable-distributed-tracing", false, "Enable experimental distributed  tracing using OpenTelemetry Tracing.")
	fs.StringVar(&cfg.ec.ExperimentalDistributedTracingAddress, "experimental-distributed-tracing-address", embed.ExperimentalDistributedTracingAddress, "Address for distributed tracing used for OpenTelemetry Tracing (if enabled with experimental-enable-distributed-tracing flag).")
	fs.StringVar(&cfg.ec.ExperimentalDistributedTracingServiceName, "experimental-distributed-tracing-service-name", embed.ExperimentalDistributedTracingServiceName, "Configures service name for distributed tracing to be used to define service name for OpenTelemetry Tracing (if enabled with experimental-enable-distributed-tracing flag). 'etcd' is the default service name. Use the same service name for all instances of etcd.")
	fs.StringVar(&cfg.ec.ExperimentalDistributedTracingServiceInstanceID, "experimental-distributed-tracing-instance-id", "", "Configures service instance ID for distributed tracing to be used to define service instance ID key for OpenTelemetry Tracing (if enabled with experimental-enable-distributed-tracing flag). There is no default value set. This ID must be unique per etcd instance.")
	fs.IntVar(&cfg.ec.ExperimentalDistributedTracingSamplingRatePerMillion, "experimental-distributed-tracing-sampling-rate", 0, "Number of samples to collect per million spans for OpenTelemetry Tracing (if enabled with experimental-enable-distributed-tracing flag).")

	// auth
	fs.StringVar(&cfg.ec.AuthToken, "auth-token", cfg.ec.AuthToken, "Specify auth token specific options.")
	fs.UintVar(&cfg.ec.BcryptCost, "bcrypt-cost", cfg.ec.BcryptCost, "Specify bcrypt algorithm cost factor for auth password hashing.")
	fs.UintVar(&cfg.ec.AuthTokenTTL, "auth-token-ttl", cfg.ec.AuthTokenTTL, "The lifetime in seconds of the auth token.")

	// gateway
	fs.BoolVar(&cfg.ec.EnableGRPCGateway, "enable-grpc-gateway", cfg.ec.EnableGRPCGateway, "Enable GRPC gateway.")

	// experimental
	fs.BoolVar(&cfg.ec.ExperimentalInitialCorruptCheck, "experimental-initial-corrupt-check", cfg.ec.ExperimentalInitialCorruptCheck, "Enable to check data corruption before serving any client/peer traffic.")
	fs.DurationVar(&cfg.ec.ExperimentalCorruptCheckTime, "experimental-corrupt-check-time", cfg.ec.ExperimentalCorruptCheckTime, "Duration of time between cluster corruption check passes.")
	fs.BoolVar(&cfg.ec.ExperimentalCompactHashCheckEnabled, "experimental-compact-hash-check-enabled", cfg.ec.ExperimentalCompactHashCheckEnabled, "Enable leader to periodically check followers compaction hashes.")
	fs.DurationVar(&cfg.ec.ExperimentalCompactHashCheckTime, "experimental-compact-hash-check-time", cfg.ec.ExperimentalCompactHashCheckTime, "Duration of time between leader checks followers compaction hashes.")

	fs.BoolVar(&cfg.ec.ExperimentalEnableLeaseCheckpoint, "experimental-enable-lease-checkpoint", false, "Enable leader to send regular checkpoints to other members to prevent reset of remaining TTL on leader change.")
	// TODO: delete in v3.7
	fs.BoolVar(&cfg.ec.ExperimentalEnableLeaseCheckpointPersist, "experimental-enable-lease-checkpoint-persist", false, "Enable persisting remainingTTL to prevent indefinite auto-renewal of long lived leases. Always enabled in v3.6. Should be used to ensure smooth upgrade from v3.5 clusters with this feature enabled. Requires experimental-enable-lease-checkpoint to be enabled.")
	fs.IntVar(&cfg.ec.ExperimentalCompactionBatchLimit, "experimental-compaction-batch-limit", cfg.ec.ExperimentalCompactionBatchLimit, "Sets the maximum revisions deleted in each compaction batch.")
	fs.DurationVar(&cfg.ec.ExperimentalCompactionSleepInterval, "experimental-compaction-sleep-interval", cfg.ec.ExperimentalCompactionSleepInterval, "Sets the sleep interval between each compaction batch.")
	fs.DurationVar(&cfg.ec.ExperimentalWatchProgressNotifyInterval, "experimental-watch-progress-notify-interval", cfg.ec.ExperimentalWatchProgressNotifyInterval, "Duration of periodic watch progress notifications.")
	fs.DurationVar(&cfg.ec.ExperimentalDowngradeCheckTime, "experimental-downgrade-check-time", cfg.ec.ExperimentalDowngradeCheckTime, "Duration of time between two downgrade status check.")
	fs.DurationVar(&cfg.ec.ExperimentalWarningApplyDuration, "experimental-warning-apply-duration", cfg.ec.ExperimentalWarningApplyDuration, "Time duration after which a warning is generated if request takes more time.")
	fs.DurationVar(&cfg.ec.ExperimentalWarningUnaryRequestDuration, "experimental-warning-unary-request-duration", cfg.ec.ExperimentalWarningUnaryRequestDuration, "Time duration after which a warning is generated if a unary request takes more time.")
	fs.BoolVar(&cfg.ec.ExperimentalMemoryMlock, "experimental-memory-mlock", cfg.ec.ExperimentalMemoryMlock, "Enable to enforce etcd pages (in particular bbolt) to stay in RAM.")
	fs.BoolVar(&cfg.ec.ExperimentalTxnModeWriteWithSharedBuffer, "experimental-txn-mode-write-with-shared-buffer", true, "Enable the write transaction to use a shared buffer in its readonly check operations.")
	fs.UintVar(&cfg.ec.ExperimentalBootstrapDefragThresholdMegabytes, "experimental-bootstrap-defrag-threshold-megabytes", 0, "Enable the defrag during etcd server bootstrap on condition that it will free at least the provided threshold of disk space. Needs to be set to non-zero value to take effect.")
	fs.IntVar(&cfg.ec.ExperimentalMaxLearners, "experimental-max-learners", membership.DefaultMaxLearners, "Sets the maximum number of learners that can be available in the cluster membership.")
	fs.DurationVar(&cfg.ec.ExperimentalWaitClusterReadyTimeout, "experimental-wait-cluster-ready-timeout", cfg.ec.ExperimentalWaitClusterReadyTimeout, "Maximum duration to wait for the cluster to be ready.")

	// unsafe
	fs.BoolVar(&cfg.ec.UnsafeNoFsync, "unsafe-no-fsync", false, "Disables fsync, unsafe, will cause data loss.")
	fs.BoolVar(&cfg.ec.ForceNewCluster, "force-new-cluster", false, "Force to create a new one member cluster.")

	// ignored
	for _, f := range cfg.ignored {
		fs.Var(&flags.IgnoredFlag{Name: f}, f, "")
	}
	return cfg
}

func (cfg *config) parse(arguments []string) error {
	perr := cfg.cf.flagSet.Parse(arguments)
	switch perr {
	case nil:
	case flag.ErrHelp:
		fmt.Println(flagsline)
		os.Exit(0)
	default:
		os.Exit(2)
	}
	if len(cfg.cf.flagSet.Args()) != 0 {
		return fmt.Errorf("'%s' is not a valid flag", cfg.cf.flagSet.Arg(0))
	}

	if cfg.printVersion {
		fmt.Printf("etcd Version: %s\n", version.Version)
		fmt.Printf("Git SHA: %s\n", version.GitSHA)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	var err error

	// This env variable must be parsed separately
	// because we need to determine whether to use or
	// ignore the env variables based on if the config file is set.
	if cfg.configFile == "" {
		cfg.configFile = os.Getenv(flags.FlagToEnv("ETCD", "config-file"))
	}

	if cfg.configFile != "" {
		err = cfg.configFromFile(cfg.configFile)
		if lg := cfg.ec.GetLogger(); lg != nil {
			lg.Info(
				"loaded server configuration, other configuration command line flags and environment variables will be ignored if provided",
				zap.String("path", cfg.configFile),
			)
		}
	} else {
		err = cfg.configFromCmdLine()
	}

	if cfg.ec.V2Deprecation == "" {
		cfg.ec.V2Deprecation = cconfig.V2_DEPR_DEFAULT
	}

	// now logger is set up
	return err
}

func (cfg *config) configFromCmdLine() error {
	// user-specified logger is not setup yet, use this logger during flag parsing
	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	if err != nil {
		return err
	}
	verKey := "ETCD_VERSION"
	if verVal := os.Getenv(verKey); verVal != "" {
		// unset to avoid any possible side-effect.
		os.Unsetenv(verKey)

		lg.Warn(
			"cannot set special environment variable",
			zap.String("key", verKey),
			zap.String("value", verVal),
		)
	}

	err = flags.SetFlagsFromEnv(lg, "ETCD", cfg.cf.flagSet)
	if err != nil {
		return err
	}

	if rafthttp.ConnReadTimeout < rafthttp.DefaultConnReadTimeout {
		rafthttp.ConnReadTimeout = rafthttp.DefaultConnReadTimeout
		lg.Info(fmt.Sprintf("raft-read-timeout increased to minimum value: %v", rafthttp.DefaultConnReadTimeout))
	}
	if rafthttp.ConnWriteTimeout < rafthttp.DefaultConnWriteTimeout {
		rafthttp.ConnWriteTimeout = rafthttp.DefaultConnWriteTimeout
		lg.Info(fmt.Sprintf("raft-write-timeout increased to minimum value: %v", rafthttp.DefaultConnWriteTimeout))
	}

	cfg.ec.LPUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-peer-urls")
	cfg.ec.APUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "initial-advertise-peer-urls")
	cfg.ec.LCUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-client-urls")
	cfg.ec.ACUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "advertise-client-urls")
	cfg.ec.ListenMetricsUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-metrics-urls")

	cfg.ec.DiscoveryCfg.Endpoints = flags.UniqueStringsFromFlag(cfg.cf.flagSet, "discovery-endpoints")

	cfg.ec.CORS = flags.UniqueURLsMapFromFlag(cfg.cf.flagSet, "cors")
	cfg.ec.HostWhitelist = flags.UniqueStringsMapFromFlag(cfg.cf.flagSet, "host-whitelist")

	cfg.ec.CipherSuites = flags.StringsFromFlag(cfg.cf.flagSet, "cipher-suites")

	cfg.ec.MaxConcurrentStreams = flags.Uint32FromFlag(cfg.cf.flagSet, "max-concurrent-streams")

	cfg.ec.LogOutputs = flags.UniqueStringsFromFlag(cfg.cf.flagSet, "log-outputs")

	cfg.ec.ClusterState = cfg.cf.clusterState.String()

	cfg.ec.V2Deprecation = cconfig.V2DeprecationEnum(cfg.cf.v2deprecation.String())

	// disable default advertise-client-urls if lcurls is set
	missingAC := flags.IsSet(cfg.cf.flagSet, "listen-client-urls") && !flags.IsSet(cfg.cf.flagSet, "advertise-client-urls")
	if missingAC {
		cfg.ec.ACUrls = nil
	}

	// disable default initial-cluster if discovery is set
	if (cfg.ec.Durl != "" || cfg.ec.DNSCluster != "" || cfg.ec.DNSClusterServiceName != "" || len(cfg.ec.DiscoveryCfg.Endpoints) > 0) && !flags.IsSet(cfg.cf.flagSet, "initial-cluster") {
		cfg.ec.InitialCluster = ""
	}

	return cfg.validate()
}

func (cfg *config) configFromFile(path string) error {
	eCfg, err := embed.ConfigFromFile(path)
	if err != nil {
		return err
	}
	cfg.ec = *eCfg

	return nil
}

func (cfg *config) validate() error {
	if cfg.cf.fallback.String() == fallbackFlagProxy {
		return fmt.Errorf("v2 proxy is deprecated, and --discovery-fallback can't be configured as %q", fallbackFlagProxy)
	}
	return cfg.ec.Validate()
}
