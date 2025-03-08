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
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/flags"
	cconfig "go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
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

	deprecatedFlags = map[string]string{
		// TODO: remove in 3.7.
		"snapshot-count": "--snapshot-count is deprecated in 3.6 and will be decommissioned in 3.7.",
		"max-snapshots":  "--max-snapshots is deprecated in 3.6 and will be decommissioned in 3.7.",
		"v2-deprecation": "--v2-deprecation is deprecated and scheduled for removal in v3.8. The default value is enforced, ignoring user input.",
		"experimental-compact-hash-check-enabled":           "--experimental-compact-hash-check-enabled is deprecated in 3.6 and will be decommissioned in 3.7. Use '--feature-gates=CompactHashCheck=true' instead.",
		"experimental-compact-hash-check-time":              "--experimental-compact-hash-check-time is deprecated in 3.6 and will be decommissioned in 3.7. Use '--compact-hash-check-time' instead.",
		"experimental-txn-mode-write-with-shared-buffer":    "--experimental-txn-mode-write-with-shared-buffer is deprecated in v3.6 and will be decommissioned in v3.7. Use '--feature-gates=TxnModeWriteWithSharedBuffer=true' instead.",
		"experimental-corrupt-check-time":                   "--experimental-corrupt-check-time is deprecated in v3.6 and will be decommissioned in v3.7. Use '--corrupt-check-time' instead.",
		"experimental-compaction-batch-limit":               "--experimental-compaction-batch-limit is deprecated in v3.6 and will be decommissioned in v3.7. Use '--compaction-batch-limit' instead.",
		"experimental-watch-progress-notify-interval":       "--experimental-watch-progress-notify-interval is deprecated in v3.6 and will be decommissioned in v3.7. Use '--watch-progress-notify-interval' instead.",
		"experimental-warning-apply-duration":               "--experimental-warning-apply-duration is deprecated in v3.6 and will be decommissioned in v3.7. Use '--warning-apply-duration' instead.",
		"experimental-bootstrap-defrag-threshold-megabytes": "--experimental-bootstrap-defrag-threshold-megabytes is deprecated in v3.6 and will be decommissioned in v3.7. Use '--bootstrap-defrag-threshold-megabytes' instead.",
		"experimental-memory-mlock":                         "--experimental-memory-mlock is deprecated in v3.6 and will be decommissioned in v3.7. Use '--memory-mlock' instead.",
		"experimental-snapshot-catchup-entries":             "--experimental-snapshot-catchup-entries is deprecated in v3.6 and will be decommissioned in v3.7. Use '--snapshot-catchup-entries' instead.",
		"experimental-compaction-sleep-interval":            "--experimental-compaction-sleep-interval is deprecated in v3.6 and will be decommissioned in v3.7. Use 'compaction-sleep-interval' instead.",
		"experimental-downgrade-check-time":                 "--experimental-downgrade-check-time is deprecated in v3.6 and will be decommissioned in v3.7. Use '--downgrade-check-time' instead.",
		"experimental-enable-distributed-tracing":           "--experimental-enable-distributed-tracing is deprecated in 3.6 and will be decommissioned in 3.7. Use --enable-distributed-tracing instead.",
		"experimental-distributed-tracing-address":          "--experimental-distributed-tracing-address is deprecated in 3.6 and will be decommissioned in 3.7. Use --distributed-tracing-address instead.",
		"experimental-distributed-tracing-service-name":     "--experimental-distributed-tracing-service-name is deprecated in 3.6 and will be decommissioned in 3.7. Use --distributed-tracing-service-name instead.",
		"experimental-distributed-tracing-instance-id":      "--experimental-distributed-tracing-instance-id is deprecated in 3.6 and will be decommissioned in 3.7. Use --distributed-tracing-instance-id instead.",
		"experimental-distributed-tracing-sampling-rate":    "--experimental-distributed-tracing-sampling-rate is deprecated in 3.6 and will be decommissioned in 3.7. Use --distributed-tracing-sampling-rate instead.",
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
	flagSet      *flag.FlagSet
	clusterState *flags.SelectiveStringValue
	fallback     *flags.SelectiveStringValue
	// Deprecated and scheduled for removal in v3.8. The default value is enforced, ignoring user input.
	// TODO: remove in v3.8.
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
			string(cconfig.V2Depr1WriteOnly),
			string(cconfig.V2Depr1WriteOnlyDrop),
			string(cconfig.V2Depr2Gone)),
	}
	fs := cfg.cf.flagSet
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usageline)
	}
	cfg.ec.AddFlags(fs)
	fs.StringVar(&cfg.configFile, "config-file", "", "Path to the server configuration file. Note that if a configuration file is provided, other command line flags and environment variables will be ignored.")
	fs.Var(cfg.cf.fallback, "discovery-fallback", fmt.Sprintf("Valid values include %q", cfg.cf.fallback.Valids()))
	fs.Var(cfg.cf.clusterState, "initial-cluster-state", "Initial cluster state ('new' when bootstrapping a new cluster or 'existing' when adding new members to an existing cluster). After successful initialization (bootstrapping or adding), flag is ignored on restarts.")
	fs.Var(cfg.cf.v2deprecation, "v2-deprecation", fmt.Sprintf("v2store deprecation stage: %q. Deprecated and scheduled for removal in v3.8. The default value is enforced, ignoring user input.", cfg.cf.v2deprecation.Valids()))

	fs.BoolVar(&cfg.printVersion, "version", false, "Print the version and exit.")
	// ignored
	for _, f := range cfg.ignored {
		fs.Var(&flags.IgnoredFlag{Name: f}, f, "")
	}
	return cfg
}

func (cfg *config) parse(arguments []string) error {
	perr := cfg.cf.flagSet.Parse(arguments)
	switch {
	case perr == nil:
	case errors.Is(perr, flag.ErrHelp):
		fmt.Println(flagsline)
		os.Exit(0)
	default:
		os.Exit(2)
	}
	if len(cfg.cf.flagSet.Args()) != 0 {
		return fmt.Errorf("%q is not a valid flag", cfg.cf.flagSet.Arg(0))
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

	// params related to experimental flag deprecation
	// TODO: delete in v3.7
	if cfg.ec.FlagsExplicitlySet["experimental-compact-hash-check-time"] {
		cfg.ec.CompactHashCheckTime = cfg.ec.ExperimentalCompactHashCheckTime
	}

	if cfg.ec.FlagsExplicitlySet["experimental-corrupt-check-time"] {
		cfg.ec.CorruptCheckTime = cfg.ec.ExperimentalCorruptCheckTime
	}

	if cfg.ec.FlagsExplicitlySet["experimental-compaction-batch-limit"] {
		cfg.ec.CompactionBatchLimit = cfg.ec.ExperimentalCompactionBatchLimit
	}

	if cfg.ec.FlagsExplicitlySet["experimental-watch-progress-notify-interval"] {
		cfg.ec.WatchProgressNotifyInterval = cfg.ec.ExperimentalWatchProgressNotifyInterval
	}

	if cfg.ec.FlagsExplicitlySet["experimental-warning-apply-duration"] {
		cfg.ec.WarningApplyDuration = cfg.ec.ExperimentalWarningApplyDuration
	}

	if cfg.ec.FlagsExplicitlySet["experimental-bootstrap-defrag-threshold-megabytes"] {
		cfg.ec.BootstrapDefragThresholdMegabytes = cfg.ec.ExperimentalBootstrapDefragThresholdMegabytes
	}
	if cfg.ec.FlagsExplicitlySet["experimental-peer-skip-client-san-verification"] {
		cfg.ec.PeerTLSInfo.SkipClientSANVerify = cfg.ec.ExperimentalPeerSkipClientSanVerification
	}

	if cfg.ec.FlagsExplicitlySet["experimental-memory-mlock"] {
		cfg.ec.MemoryMlock = cfg.ec.ExperimentalMemoryMlock
	}

	if cfg.ec.FlagsExplicitlySet["experimental-snapshot-catchup-entries"] {
		cfg.ec.SnapshotCatchUpEntries = cfg.ec.ExperimentalSnapshotCatchUpEntries
	}

	if cfg.ec.FlagsExplicitlySet["experimental-compaction-sleep-interval"] {
		cfg.ec.CompactionSleepInterval = cfg.ec.ExperimentalCompactionSleepInterval
	}

	if cfg.ec.FlagsExplicitlySet["experimental-downgrade-check-time"] {
		cfg.ec.DowngradeCheckTime = cfg.ec.ExperimentalDowngradeCheckTime
	}

	if cfg.ec.FlagsExplicitlySet["experimental-enable-distributed-tracing"] {
		cfg.ec.EnableDistributedTracing = cfg.ec.ExperimentalEnableDistributedTracing
	}

	if cfg.ec.FlagsExplicitlySet["experimental-distributed-tracing-address"] {
		cfg.ec.DistributedTracingAddress = cfg.ec.ExperimentalDistributedTracingAddress
	}

	if cfg.ec.FlagsExplicitlySet["experimental-distributed-tracing-service-name"] {
		cfg.ec.DistributedTracingServiceName = cfg.ec.ExperimentalDistributedTracingServiceName
	}

	if cfg.ec.FlagsExplicitlySet["experimental-distributed-tracing-instance-id"] {
		cfg.ec.DistributedTracingServiceInstanceID = cfg.ec.ExperimentalDistributedTracingServiceInstanceID
	}

	if cfg.ec.FlagsExplicitlySet["experimental-distributed-tracing-sampling-rate"] {
		cfg.ec.DistributedTracingSamplingRatePerMillion = cfg.ec.ExperimentalDistributedTracingSamplingRatePerMillion
	}

	// `V2Deprecation` (--v2-deprecation) is deprecated and scheduled for removal in v3.8. The default value is enforced, ignoring user input.
	cfg.ec.V2Deprecation = cconfig.V2DeprDefault

	cfg.ec.WarningUnaryRequestDuration, perr = cfg.parseWarningUnaryRequestDuration()
	if perr != nil {
		return perr
	}

	// Check for deprecated options from both command line and config file
	var warningsForDeprecatedOpts []string
	for flagName := range cfg.ec.FlagsExplicitlySet {
		if msg, ok := deprecatedFlags[flagName]; ok {
			warningsForDeprecatedOpts = append(warningsForDeprecatedOpts, msg)
		}
	}

	// Log warnings if any deprecated options were found
	if len(warningsForDeprecatedOpts) > 0 {
		if lg := cfg.ec.GetLogger(); lg != nil {
			for _, msg := range warningsForDeprecatedOpts {
				lg.Warn(msg)
			}
		}
	}

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

	cfg.ec.ListenPeerUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-peer-urls")
	cfg.ec.AdvertisePeerUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "initial-advertise-peer-urls")
	cfg.ec.ListenClientUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-client-urls")
	cfg.ec.ListenClientHttpUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-client-http-urls")
	cfg.ec.AdvertiseClientUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "advertise-client-urls")
	cfg.ec.ListenMetricsUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-metrics-urls")

	cfg.ec.DiscoveryCfg.Endpoints = flags.UniqueStringsFromFlag(cfg.cf.flagSet, "discovery-endpoints")

	cfg.ec.CORS = flags.UniqueURLsMapFromFlag(cfg.cf.flagSet, "cors")
	cfg.ec.HostWhitelist = flags.UniqueStringsMapFromFlag(cfg.cf.flagSet, "host-whitelist")

	cfg.ec.ClientTLSInfo.AllowedHostnames = flags.StringsFromFlag(cfg.cf.flagSet, "client-cert-allowed-hostname")
	cfg.ec.PeerTLSInfo.AllowedCNs = flags.StringsFromFlag(cfg.cf.flagSet, "peer-cert-allowed-cn")
	cfg.ec.PeerTLSInfo.AllowedHostnames = flags.StringsFromFlag(cfg.cf.flagSet, "peer-cert-allowed-hostname")

	cfg.ec.CipherSuites = flags.StringsFromFlag(cfg.cf.flagSet, "cipher-suites")

	cfg.ec.MaxConcurrentStreams = flags.Uint32FromFlag(cfg.cf.flagSet, "max-concurrent-streams")

	cfg.ec.LogOutputs = flags.UniqueStringsFromFlag(cfg.cf.flagSet, "log-outputs")

	cfg.ec.ClusterState = cfg.cf.clusterState.String()

	cfg.ec.V2Deprecation = cconfig.V2DeprecationEnum(cfg.cf.v2deprecation.String())

	// disable default advertise-client-urls if lcurls is set
	missingAC := flags.IsSet(cfg.cf.flagSet, "listen-client-urls") && !flags.IsSet(cfg.cf.flagSet, "advertise-client-urls")
	if missingAC {
		cfg.ec.AdvertiseClientUrls = nil
	}

	// disable default initial-cluster if discovery is set
	if (cfg.ec.Durl != "" || cfg.ec.DNSCluster != "" || cfg.ec.DNSClusterServiceName != "" || len(cfg.ec.DiscoveryCfg.Endpoints) > 0) && !flags.IsSet(cfg.cf.flagSet, "initial-cluster") {
		cfg.ec.InitialCluster = ""
	}

	cfg.cf.flagSet.Visit(func(f *flag.Flag) {
		cfg.ec.FlagsExplicitlySet[f.Name] = true
	})

	getBoolFlagVal := func(flagName string) *bool {
		boolVal, parseErr := flags.GetBoolFlagVal(cfg.cf.flagSet, flagName)
		if parseErr != nil {
			panic(parseErr)
		}
		return boolVal
	}

	// SetFeatureGatesFromExperimentalFlags validates that cmd line flags for experimental feature and their feature gates are not explicitly set simultaneously,
	// and passes the values of cmd line flags for experimental feature to the server feature gate.
	err = embed.SetFeatureGatesFromExperimentalFlags(cfg.ec.ServerFeatureGate, getBoolFlagVal, cfg.cf.flagSet.Lookup(embed.ServerFeatureGateFlagName).Value.String())
	if err != nil {
		return err
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

func (cfg *config) parseWarningUnaryRequestDuration() (time.Duration, error) {
	if cfg.ec.ExperimentalWarningUnaryRequestDuration != 0 && cfg.ec.WarningUnaryRequestDuration != 0 {
		return 0, errors.New(
			"both --experimental-warning-unary-request-duration and --warning-unary-request-duration flags are set. " +
				"Use only --warning-unary-request-duration")
	}

	if cfg.ec.WarningUnaryRequestDuration != 0 {
		return cfg.ec.WarningUnaryRequestDuration, nil
	}

	if cfg.ec.ExperimentalWarningUnaryRequestDuration != 0 {
		cfg.ec.GetLogger().Warn(
			"--experimental-warning-unary-request-duration is deprecated, and will be decommissioned in v3.7. " +
				"Use --warning-unary-request-duration instead.")
		return cfg.ec.ExperimentalWarningUnaryRequestDuration, nil
	}

	return embed.DefaultWarningUnaryRequestDuration, nil
}
