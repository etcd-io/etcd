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

package command

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/bgentry/speakeasy"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/flags"
	"go.etcd.io/etcd/pkg/srv"
	"go.etcd.io/etcd/pkg/transport"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"
)

// GlobalFlags are flags that defined globally
// and are inherited to all sub-commands.
type GlobalFlags struct {
	Insecure              bool
	InsecureSkipVerify    bool
	InsecureDiscovery     bool
	Endpoints             []string
	DialTimeout           time.Duration
	CommandTimeOut        time.Duration
	KeepAliveTime         time.Duration
	KeepAliveTimeout      time.Duration
	DNSClusterServiceName string

	TLS transport.TLSInfo

	OutputFormat string
	IsHex        bool

	User     string
	Password string

	Debug bool
}

type secureCfg struct {
	cert       string
	key        string
	cacert     string
	serverName string

	insecureTransport  bool
	insecureSkipVerify bool
}

type authCfg struct {
	username string
	password string
}

type discoveryCfg struct {
	domain      string
	insecure    bool
	serviceName string
}

type clientConfig struct {
	endpoints        []string
	dialTimeout      time.Duration
	keepAliveTime    time.Duration
	keepAliveTimeout time.Duration
	scfg             *secureCfg
	acfg             *authCfg
}

type discardValue struct{}

var display printer = &simplePrinter{}

var (
	viperHandler *viper.Viper
)

func SetViperHandler(v *viper.Viper) {
	viperHandler = v
}

func getViperBasedParam(cmd *cobra.Command, f *pflag.Flag) (interface{}, error) {
	switch f.Value.Type() {
	case "stringSlice":
		return getStringSliceParamBasedOnConfig(cmd, f.Name)
	case "bool":
		return getBooleanParamBasedOnConfig(cmd, f.Name)
	case "duration":
		return getDurationParamBasedOnConfig(cmd, f.Name)
	case "string", "":
		return getStringParamBasedOnConfig(cmd, f.Name)
	default:
		return getStringParamBasedOnConfig(cmd, f.Name)
	}
}

func getBooleanParamBasedOnConfig(cmd *cobra.Command, arg string) (bool, error) {
	var val bool
	var err error
	flagSet := cmd.Flags().Lookup(arg)
	val, err = cmd.Flags().GetBool(arg)
	if viperHandler.IsSet(arg) && !flagSet.Changed {
		val = viperHandler.GetBool(arg)
	}
	return val, err
}

func getStringParamBasedOnConfig(cmd *cobra.Command, arg string) (string, error) {
	var val string
	var err error
	flagSet := cmd.Flags().Lookup(arg)
	val, err = cmd.Flags().GetString(arg)
	if viperHandler.IsSet(arg) && !flagSet.Changed {
		val = viperHandler.GetString(arg)
	}
	return val, err
}

func getStringParamBasedOnConfigWithHook(cmd *cobra.Command, arg string, hook func(string) (string, error)) (string, error) {
	val, err := getStringParamBasedOnConfig(cmd, arg)
	if err == nil && (viperHandler.IsSet(arg) && !cmd.Flags().Lookup(arg).Changed) {
		val, err = hook(val)
	}
	return val, err
}

func getStringSliceParamBasedOnConfig(cmd *cobra.Command, arg string) ([]string, error) {
	var val []string
	var err error
	flagSet := cmd.Flags().Lookup(arg)
	val, err = cmd.Flags().GetStringSlice(arg)
	if viperHandler.IsSet(arg) && !flagSet.Changed {
		val = viperHandler.GetStringSlice(arg)
	}
	return val, err
}

func getDurationParamBasedOnConfig(cmd *cobra.Command, arg string) (time.Duration, error) {
	var val time.Duration
	var err error
	flagSet := cmd.Flags().Lookup(arg)
	val, err = cmd.Flags().GetDuration(arg)
	if viperHandler.IsSet(arg) && !flagSet.Changed {
		val = viperHandler.GetDuration(arg)
	}
	return val, err
}

func initDisplayFromCmd(cmd *cobra.Command) {
	isHex, err := getBooleanParamBasedOnConfig(cmd, "hex")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	outputType, err := getStringParamBasedOnConfig(cmd, "write-out")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if display = NewPrinter(outputType, isHex); display == nil {
		ExitWithError(ExitBadFeature, errors.New("unsupported output format"))
	}
}

func (*discardValue) String() string   { return "" }
func (*discardValue) Set(string) error { return nil }
func (*discardValue) Type() string     { return "" }

func clientConfigFromCmd(cmd *cobra.Command) *clientConfig {
	lg, err := zap.NewProduction()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	fs := cmd.InheritedFlags()
	if strings.HasPrefix(cmd.Use, "watch") {
		// silence "pkg/flags: unrecognized environment variable ETCDCTL_WATCH_KEY=foo" warnings
		// silence "pkg/flags: unrecognized environment variable ETCDCTL_WATCH_RANGE_END=bar" warnings
		fs.AddFlag(&pflag.Flag{Name: "watch-key", Value: &discardValue{}})
		fs.AddFlag(&pflag.Flag{Name: "watch-range-end", Value: &discardValue{}})
	}
	flags.SetPflagsFromEnv(lg, "ETCDCTL", fs)

	debug, err := getBooleanParamBasedOnConfig(cmd, "debug")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	if debug {
		clientv3.SetLogger(grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 4))
		fs.VisitAll(func(f *pflag.Flag) {
			var val interface{}
			if val, err = getViperBasedParam(cmd, f); err != nil {
				val = f.Value
			}
			fmt.Fprintf(os.Stderr, "%s=%v\n", flags.FlagToEnv("ETCDCTL", f.Name), val)
		})
	} else {
		// WARNING logs contain important information like TLS misconfirugation, but spams
		// too many routine connection disconnects to turn on by default.
		//
		// See https://github.com/etcd-io/etcd/pull/9623 for background
		clientv3.SetLogger(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, os.Stderr))
	}

	cfg := &clientConfig{}
	cfg.endpoints, err = endpointsFromCmd(cmd)
	if err != nil {
		ExitWithError(ExitError, err)
	}

	cfg.dialTimeout = dialTimeoutFromCmd(cmd)
	cfg.keepAliveTime = keepAliveTimeFromCmd(cmd)
	cfg.keepAliveTimeout = keepAliveTimeoutFromCmd(cmd)

	cfg.scfg = secureCfgFromCmd(cmd)
	cfg.acfg = authCfgFromCmd(cmd)

	initDisplayFromCmd(cmd)
	return cfg
}

func mustClientCfgFromCmd(cmd *cobra.Command) *clientv3.Config {
	cc := clientConfigFromCmd(cmd)
	cfg, err := newClientCfg(cc.endpoints, cc.dialTimeout, cc.keepAliveTime, cc.keepAliveTimeout, cc.scfg, cc.acfg)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}
	return cfg
}

func mustClientFromCmd(cmd *cobra.Command) *clientv3.Client {
	cfg := clientConfigFromCmd(cmd)
	return cfg.mustClient()
}

func (cc *clientConfig) mustClient() *clientv3.Client {
	cfg, err := newClientCfg(cc.endpoints, cc.dialTimeout, cc.keepAliveTime, cc.keepAliveTimeout, cc.scfg, cc.acfg)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	client, err := clientv3.New(*cfg)
	if err != nil {
		ExitWithError(ExitBadConnection, err)
	}

	return client
}

func newClientCfg(endpoints []string, dialTimeout, keepAliveTime, keepAliveTimeout time.Duration, scfg *secureCfg, acfg *authCfg) (*clientv3.Config, error) {
	// set tls if any one tls option set
	var cfgtls *transport.TLSInfo
	tlsinfo := transport.TLSInfo{}
	tlsinfo.Logger, _ = zap.NewProduction()
	if scfg.cert != "" {
		tlsinfo.CertFile = scfg.cert
		cfgtls = &tlsinfo
	}

	if scfg.key != "" {
		tlsinfo.KeyFile = scfg.key
		cfgtls = &tlsinfo
	}

	if scfg.cacert != "" {
		tlsinfo.TrustedCAFile = scfg.cacert
		cfgtls = &tlsinfo
	}

	if scfg.serverName != "" {
		tlsinfo.ServerName = scfg.serverName
		cfgtls = &tlsinfo
	}

	cfg := &clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    keepAliveTime,
		DialKeepAliveTimeout: keepAliveTimeout,
	}

	if cfgtls != nil {
		clientTLS, err := cfgtls.ClientConfig()
		if err != nil {
			return nil, err
		}
		cfg.TLS = clientTLS
	}

	// if key/cert is not given but user wants secure connection, we
	// should still setup an empty tls configuration for gRPC to setup
	// secure connection.
	if cfg.TLS == nil && !scfg.insecureTransport {
		cfg.TLS = &tls.Config{}
	}

	// If the user wants to skip TLS verification then we should set
	// the InsecureSkipVerify flag in tls configuration.
	if scfg.insecureSkipVerify && cfg.TLS != nil {
		cfg.TLS.InsecureSkipVerify = true
	}

	if acfg != nil {
		cfg.Username = acfg.username
		cfg.Password = acfg.password
	}

	return cfg, nil
}

func argOrStdin(args []string, stdin io.Reader, i int) (string, error) {
	if i < len(args) {
		return args[i], nil
	}
	bytes, err := ioutil.ReadAll(stdin)
	if string(bytes) == "" || err != nil {
		return "", errors.New("no available argument and stdin")
	}
	return string(bytes), nil
}

func dialTimeoutFromCmd(cmd *cobra.Command) time.Duration {
	dialTimeout, err := getDurationParamBasedOnConfig(cmd, "dial-timeout")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return dialTimeout
}

func keepAliveTimeFromCmd(cmd *cobra.Command) time.Duration {
	keepAliveTime, err := getDurationParamBasedOnConfig(cmd, "keepalive-time")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return keepAliveTime
}

func keepAliveTimeoutFromCmd(cmd *cobra.Command) time.Duration {
	keepAliveTimeout, err := getDurationParamBasedOnConfig(cmd, "keepalive-timeout")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return keepAliveTimeout
}

func secureCfgFromCmd(cmd *cobra.Command) *secureCfg {
	cert, key, cacert := keyAndCertFromCmd(cmd)
	insecureTr := insecureTransportFromCmd(cmd)
	skipVerify := insecureSkipVerifyFromCmd(cmd)
	discoveryCfg := discoveryCfgFromCmd(cmd)

	if discoveryCfg.insecure {
		discoveryCfg.domain = ""
	}

	return &secureCfg{
		cert:       cert,
		key:        key,
		cacert:     cacert,
		serverName: discoveryCfg.domain,

		insecureTransport:  insecureTr,
		insecureSkipVerify: skipVerify,
	}
}

func insecureTransportFromCmd(cmd *cobra.Command) bool {
	insecureTr, err := getBooleanParamBasedOnConfig(cmd, "insecure-transport")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return insecureTr
}

func insecureSkipVerifyFromCmd(cmd *cobra.Command) bool {
	skipVerify, err := getBooleanParamBasedOnConfig(cmd, "insecure-skip-tls-verify")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return skipVerify
}

func keyAndCertFromCmd(cmd *cobra.Command) (cert, key, cacert string) {
	var err error
	if cert, err = getStringParamBasedOnConfig(cmd, "cert"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if cert == "" && cmd.Flags().Changed("cert") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --cert option"))
	}

	if key, err = getStringParamBasedOnConfig(cmd, "key"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if key == "" && cmd.Flags().Changed("key") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --key option"))
	}

	if cacert, err = getStringParamBasedOnConfig(cmd, "cacert"); err != nil {
		ExitWithError(ExitBadArgs, err)
	} else if cacert == "" && cmd.Flags().Changed("cacert") {
		ExitWithError(ExitBadArgs, errors.New("empty string is passed to --cacert option"))
	}

	return cert, key, cacert
}

func authCfgFromCmd(cmd *cobra.Command) *authCfg {
	userFlag, err := getStringParamBasedOnConfig(cmd, "user")
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}
	passwordFlag, err := getStringParamBasedOnConfigWithHook(cmd, "password", decodePassword)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	if userFlag == "" {
		return nil
	}

	var cfg authCfg

	if passwordFlag == "" {
		splitted := strings.SplitN(userFlag, ":", 2)
		if len(splitted) < 2 {
			cfg.username = userFlag
			cfg.password, err = speakeasy.Ask("Password: ")
			if err != nil {
				ExitWithError(ExitError, err)
			}
		} else {
			cfg.username = splitted[0]
			cfg.password = splitted[1]
		}
	} else {
		cfg.username = userFlag
		cfg.password = passwordFlag
	}

	return &cfg
}

func insecureDiscoveryFromCmd(cmd *cobra.Command) bool {
	discovery, err := getBooleanParamBasedOnConfig(cmd, "insecure-discovery")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return discovery
}

func discoverySrvFromCmd(cmd *cobra.Command) string {
	domainStr, err := getStringParamBasedOnConfig(cmd, "discovery-srv")
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}
	return domainStr
}

func discoveryDNSClusterServiceNameFromCmd(cmd *cobra.Command) string {
	serviceNameStr, err := getStringParamBasedOnConfig(cmd, "discovery-srv-name")
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}
	return serviceNameStr
}

func discoveryCfgFromCmd(cmd *cobra.Command) *discoveryCfg {
	return &discoveryCfg{
		domain:      discoverySrvFromCmd(cmd),
		insecure:    insecureDiscoveryFromCmd(cmd),
		serviceName: discoveryDNSClusterServiceNameFromCmd(cmd),
	}
}

func endpointsFromCmd(cmd *cobra.Command) ([]string, error) {
	eps, err := endpointsFromFlagValue(cmd)
	if err != nil {
		return nil, err
	}
	// If domain discovery returns no endpoints, check endpoints flag
	if len(eps) == 0 {
		eps, err = getStringSliceParamBasedOnConfig(cmd, "endpoints")
		if err == nil {
			for i, ip := range eps {
				eps[i] = strings.TrimSpace(ip)
			}
		}
	}
	return eps, err
}

func endpointsFromFlagValue(cmd *cobra.Command) ([]string, error) {
	discoveryCfg := discoveryCfgFromCmd(cmd)

	// If we still don't have domain discovery, return nothing
	if discoveryCfg.domain == "" {
		return []string{}, nil
	}

	srvs, err := srv.GetClient("etcd-client", discoveryCfg.domain, discoveryCfg.serviceName)
	if err != nil {
		return nil, err
	}
	eps := srvs.Endpoints
	if discoveryCfg.insecure {
		return eps, err
	}
	// strip insecure connections
	ret := []string{}
	for _, ep := range eps {
		if strings.HasPrefix(ep, "http://") {
			fmt.Fprintf(os.Stderr, "ignoring discovered insecure endpoint %q\n", ep)
			continue
		}
		ret = append(ret, ep)
	}
	return ret, err
}
