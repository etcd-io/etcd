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
	cfg.ec.AddFlags(fs)
	fs.StringVar(&cfg.configFile, "config-file", "", "Path to the server configuration file. Note that if a configuration file is provided, other command line flags and environment variables will be ignored.")
	fs.Var(cfg.cf.fallback, "discovery-fallback", fmt.Sprintf("Valid values include %q", cfg.cf.fallback.Valids()))
	fs.Var(cfg.cf.clusterState, "initial-cluster-state", "Initial cluster state ('new' when bootstrapping a new cluster or 'existing' when adding new members to an existing cluster). After successful initialization (bootstrapping or adding), flag is ignored on restarts.")
	fs.Var(cfg.cf.v2deprecation, "v2-deprecation", fmt.Sprintf("v2store deprecation stage: %q. ", cfg.cf.v2deprecation.Valids()))

	fs.BoolVar(&cfg.printVersion, "version", false, "Print the version and exit.")
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

	if cfg.ec.V2Deprecation == "" {
		cfg.ec.V2Deprecation = cconfig.V2_DEPR_DEFAULT
	}

	cfg.ec.WarningUnaryRequestDuration, perr = cfg.parseWarningUnaryRequestDuration()
	if perr != nil {
		return perr
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

	cfg.ec.ListenPeerUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-peer-urls")
	cfg.ec.AdvertisePeerUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "initial-advertise-peer-urls")
	cfg.ec.ListenClientUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-client-urls")
	cfg.ec.ListenClientHttpUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "listen-client-http-urls")
	cfg.ec.AdvertiseClientUrls = flags.UniqueURLsFromFlag(cfg.cf.flagSet, "advertise-client-urls")
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
		cfg.ec.AdvertiseClientUrls = nil
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
