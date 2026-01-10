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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/client/pkg/v3/srv"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/pkg/v3/flags"
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
	MaxCallSendMsgSize    int
	MaxCallRecvMsgSize    int
	DNSClusterServiceName string

	TLS transport.TLSInfo

	OutputFormat string
	IsHex        bool

	User     string
	Password string
	Token    string

	Debug bool
}

type discoveryCfg struct {
	domain      string
	insecure    bool
	serviceName string
}

var display printer = &simplePrinter{}

var globalFlags GlobalFlags

func RegisterGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringSliceVar(&globalFlags.Endpoints, "endpoints", []string{"127.0.0.1:2379"}, "gRPC endpoints")
	cmd.PersistentFlags().BoolVar(&globalFlags.Debug, "debug", false, "enable client-side debug logging")
	cmd.PersistentFlags().StringVarP(&globalFlags.OutputFormat, "write-out", "w", "simple", "set the output format (fields, json, protobuf, simple, table)")
	cmd.PersistentFlags().BoolVar(&globalFlags.IsHex, "hex", false, "print byte strings as hex encoded strings")
	cmd.PersistentFlags().DurationVar(&globalFlags.DialTimeout, "dial-timeout", 2*time.Second, "dial timeout for client connections")
	cmd.PersistentFlags().DurationVar(&globalFlags.CommandTimeOut, "command-timeout", 5*time.Second, "timeout for short running command (excluding dial timeout)")
	cmd.PersistentFlags().DurationVar(&globalFlags.KeepAliveTime, "keepalive-time", 2*time.Second, "keepalive time for client connections")
	cmd.PersistentFlags().DurationVar(&globalFlags.KeepAliveTimeout, "keepalive-timeout", 6*time.Second, "keepalive timeout for client connections")
	cmd.PersistentFlags().IntVar(&globalFlags.MaxCallSendMsgSize, "max-request-bytes", 0, "client-side request send limit in bytes (if 0, it defaults to 2.0 MiB (2 * 1024 * 1024).)")
	cmd.PersistentFlags().IntVar(&globalFlags.MaxCallRecvMsgSize, "max-recv-bytes", 0, "client-side response receive limit in bytes (if 0, it defaults to \"math.MaxInt32\")")
	cmd.PersistentFlags().BoolVar(&globalFlags.Insecure, "insecure-transport", true, "disable transport security for client connections")
	cmd.PersistentFlags().BoolVar(&globalFlags.InsecureDiscovery, "insecure-discovery", true, "accept insecure SRV records describing cluster endpoints")
	cmd.PersistentFlags().BoolVar(&globalFlags.InsecureSkipVerify, "insecure-skip-tls-verify", false, "skip server certificate verification (CAUTION: this option should be enabled only for testing purposes)")
	cmd.PersistentFlags().StringVar(&globalFlags.TLS.CertFile, "cert", "", "identify secure client using this TLS certificate file")
	cmd.PersistentFlags().StringVar(&globalFlags.TLS.KeyFile, "key", "", "identify secure client using this TLS key file")
	cmd.PersistentFlags().StringVar(&globalFlags.TLS.TrustedCAFile, "cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle")
	cmd.PersistentFlags().StringVar(&globalFlags.Token, "auth-jwt-token", "", "JWT token used for authentication (if this option is used, --user and --password should not be set)")
	cmd.PersistentFlags().StringVar(&globalFlags.User, "user", "", "username[:password] for authentication (prompt if password is not supplied)")
	cmd.PersistentFlags().StringVar(&globalFlags.Password, "password", "", "password for authentication (if this option is used, --user option shouldn't include password)")
	cmd.PersistentFlags().StringVarP(&globalFlags.TLS.ServerName, "discovery-srv", "d", "", "domain name to query for SRV records describing cluster endpoints")
	cmd.PersistentFlags().StringVar(&globalFlags.DNSClusterServiceName, "discovery-srv-name", "", "service name to query when using DNS discovery")
}

func initDisplayFromCmd() {
	if display = NewPrinter(globalFlags.OutputFormat, globalFlags.IsHex); display == nil {
		cobrautl.ExitWithError(cobrautl.ExitBadFeature, errors.New("unsupported output format"))
	}
}

type discardValue struct{}

func (*discardValue) String() string   { return "" }
func (*discardValue) Set(string) error { return nil }
func (*discardValue) Type() string     { return "" }

func clientConfigFromCmd(cmd *cobra.Command) *clientv3.ConfigSpec {
	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	fs := cmd.InheritedFlags()
	if strings.HasPrefix(cmd.Use, "watch") {
		// silence "pkg/flags: unrecognized environment variable ETCDCTL_WATCH_KEY=foo" warnings
		// silence "pkg/flags: unrecognized environment variable ETCDCTL_WATCH_RANGE_END=bar" warnings
		fs.AddFlag(&pflag.Flag{Name: "watch-key", Value: &discardValue{}})
		fs.AddFlag(&pflag.Flag{Name: "watch-range-end", Value: &discardValue{}})
	}
	flags.SetPflagsFromEnv(lg, "ETCDCTL", fs)

	if globalFlags.Debug {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 4))
		fs.VisitAll(func(f *pflag.Flag) {
			fmt.Fprintf(os.Stderr, "%s=%v\n", flags.FlagToEnv("ETCDCTL", f.Name), f.Value)
		})
	} else {
		// WARNING logs contain important information like TLS misconfirugation, but spams
		// too many routine connection disconnects to turn on by default.
		//
		// See https://github.com/etcd-io/etcd/pull/9623 for background
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, os.Stderr))
	}

	cfg := &clientv3.ConfigSpec{}
	cfg.Endpoints, err = endpointsFromCmd()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	cfg.DialTimeout = dialTimeoutFromCmd()
	cfg.KeepAliveTime = keepAliveTimeFromCmd()
	cfg.KeepAliveTimeout = keepAliveTimeoutFromCmd()
	cfg.MaxCallSendMsgSize = maxCallSendMsgSizeFromCmd()
	cfg.MaxCallRecvMsgSize = maxCallRecvMsgSizeFromCmd()

	cfg.Secure = secureCfgFromCmd(cmd)
	cfg.Auth = authCfgFromCmd()

	initDisplayFromCmd()
	return cfg
}

func mustClientCfgFromCmd(cmd *cobra.Command) *clientv3.Config {
	cc := clientConfigFromCmd(cmd)
	lg, _ := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	cfg, err := clientv3.NewClientConfig(cc, lg)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}
	return cfg
}

func mustClientFromCmd(cmd *cobra.Command) *clientv3.Client {
	cfg := clientConfigFromCmd(cmd)
	return mustClient(cfg)
}

func mustClient(cc *clientv3.ConfigSpec) *clientv3.Client {
	lg, _ := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	cfg, err := clientv3.NewClientConfig(cc, lg)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}

	client, err := clientv3.New(*cfg)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadConnection, err)
	}

	return client
}

func argOrStdin(args []string, stdin io.Reader, i int) (string, error) {
	if i < len(args) {
		return args[i], nil
	}
	bytes, err := io.ReadAll(stdin)
	if string(bytes) == "" || err != nil {
		return "", errors.New("no available argument and stdin")
	}
	return string(bytes), nil
}

func dialTimeoutFromCmd() time.Duration {
	return globalFlags.DialTimeout
}

func keepAliveTimeFromCmd() time.Duration {
	return globalFlags.KeepAliveTime
}

func keepAliveTimeoutFromCmd() time.Duration {
	return globalFlags.KeepAliveTimeout
}

func maxCallSendMsgSizeFromCmd() int {
	return globalFlags.MaxCallSendMsgSize
}

func maxCallRecvMsgSizeFromCmd() int {
	return globalFlags.MaxCallRecvMsgSize
}

func secureCfgFromCmd(cmd *cobra.Command) *clientv3.SecureConfig {
	cert, key, cacert := keyAndCertFromCmd(cmd)
	insecureTr := insecureTransportFromCmd()
	skipVerify := insecureSkipVerifyFromCmd()
	discoveryCfg := discoveryCfgFromCmd()

	if discoveryCfg.insecure {
		discoveryCfg.domain = ""
	}

	return &clientv3.SecureConfig{
		Cert:       cert,
		Key:        key,
		Cacert:     cacert,
		ServerName: discoveryCfg.domain,

		InsecureTransport:  insecureTr,
		InsecureSkipVerify: skipVerify,
	}
}

func insecureTransportFromCmd() bool {
	return globalFlags.Insecure
}

func insecureSkipVerifyFromCmd() bool {
	return globalFlags.InsecureSkipVerify
}

func keyAndCertFromCmd(cmd *cobra.Command) (cert, key, cacert string) {
	cert = globalFlags.TLS.CertFile
	if cert == "" && cmd.Flags().Changed("cert") {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("empty string is passed to --cert option"))
	}

	key = globalFlags.TLS.KeyFile
	if key == "" && cmd.Flags().Changed("key") {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("empty string is passed to --key option"))
	}

	cacert = globalFlags.TLS.TrustedCAFile
	if cacert == "" && cmd.Flags().Changed("cacert") {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("empty string is passed to --cacert option"))
	}

	return cert, key, cacert
}

func authCfgFromCmd() *clientv3.AuthConfig {
	userFlag := globalFlags.User
	passwordFlag := globalFlags.Password
	tokenFlag := globalFlags.Token
	var err error

	if userFlag == "" && tokenFlag == "" {
		return nil
	}

	var cfg clientv3.AuthConfig

	if tokenFlag != "" {
		cfg.Token = tokenFlag
		return &cfg
	}

	if passwordFlag == "" {
		splitted := strings.SplitN(userFlag, ":", 2)
		if len(splitted) < 2 {
			cfg.Username = userFlag
			cfg.Password, err = speakeasy.Ask("Password: ")
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitError, err)
			}
		} else {
			cfg.Username = splitted[0]
			cfg.Password = splitted[1]
		}
	} else {
		cfg.Username = userFlag
		cfg.Password = passwordFlag
	}

	return &cfg
}

func insecureDiscoveryFromCmd() bool {
	return globalFlags.InsecureDiscovery
}

func discoverySrvFromCmd() string {
	return globalFlags.TLS.ServerName
}

func discoveryDNSClusterServiceNameFromCmd() string {
	return globalFlags.DNSClusterServiceName
}

func discoveryCfgFromCmd() *discoveryCfg {
	return &discoveryCfg{
		domain:      discoverySrvFromCmd(),
		insecure:    insecureDiscoveryFromCmd(),
		serviceName: discoveryDNSClusterServiceNameFromCmd(),
	}
}

func endpointsFromCmd() ([]string, error) {
	eps, err := endpointsFromFlagValue()
	if err != nil {
		return nil, err
	}
	// If domain discovery returns no endpoints, check endpoints flag
	if len(eps) == 0 {
		eps = globalFlags.Endpoints
		for i, ip := range eps {
			eps[i] = strings.TrimSpace(ip)
		}
	}
	return eps, err
}

func endpointsFromFlagValue() ([]string, error) {
	discoveryCfg := discoveryCfgFromCmd()

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
	var ret []string
	for _, ep := range eps {
		if strings.HasPrefix(ep, "http://") {
			fmt.Fprintf(os.Stderr, "ignoring discovered insecure endpoint %q\n", ep)
			continue
		}
		ret = append(ret, ep)
	}
	return ret, err
}
