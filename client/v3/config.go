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

package clientv3

import (
	"context"
	"crypto/tls"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/cobrautl"
	"go.etcd.io/etcd/client/pkg/v3/transport"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Config struct {
	// Endpoints is a list of URLs.
	Endpoints []string `json:"endpoints"`

	// AutoSyncInterval is the interval to update endpoints with its latest members.
	// 0 disables auto-sync. By default auto-sync is disabled.
	AutoSyncInterval time.Duration `json:"auto-sync-interval"`

	// DialTimeout is the timeout for failing to establish a connection.
	DialTimeout time.Duration `json:"dial-timeout"`

	// DialKeepAliveTime is the time after which client pings the server to see if
	// transport is alive.
	DialKeepAliveTime time.Duration `json:"dial-keep-alive-time"`

	// DialKeepAliveTimeout is the time that the client waits for a response for the
	// keep-alive probe. If the response is not received in this time, the connection is closed.
	DialKeepAliveTimeout time.Duration `json:"dial-keep-alive-timeout"`

	// MaxCallSendMsgSize is the client-side request send limit in bytes.
	// If 0, it defaults to 2.0 MiB (2 * 1024 * 1024).
	// Make sure that "MaxCallSendMsgSize" < server-side default send/recv limit.
	// ("--max-request-bytes" flag to etcd or "embed.Config.MaxRequestBytes").
	MaxCallSendMsgSize int

	// MaxCallRecvMsgSize is the client-side response receive limit.
	// If 0, it defaults to "math.MaxInt32", because range response can
	// easily exceed request send limits.
	// Make sure that "MaxCallRecvMsgSize" >= server-side default send/recv limit.
	// ("--max-request-bytes" flag to etcd or "embed.Config.MaxRequestBytes").
	MaxCallRecvMsgSize int

	// TLS holds the client secure credentials, if any.
	TLS *tls.Config

	// Username is a user name for authentication.
	Username string `json:"username"`

	// Password is a password for authentication.
	Password string `json:"password"`

	// RejectOldCluster when set will refuse to create a client against an outdated cluster.
	RejectOldCluster bool `json:"reject-old-cluster"`

	// DialOptions is a list of dial options for the grpc client (e.g., for interceptors).
	// For example, pass "grpc.WithBlock()" to block until the underlying connection is up.
	// Without this, Dial returns immediately and connecting the server happens in background.
	DialOptions []grpc.DialOption

	// Context is the default client context; it can be used to cancel grpc dial out and
	// other operations that do not have an explicit context.
	Context context.Context

	// Logger sets client-side logger.
	// If nil, fallback to building LogConfig.
	Logger *zap.Logger

	// LogConfig configures client-side logger.
	// If nil, use the default logger.
	// TODO: configure gRPC logger
	LogConfig *zap.Config

	// PermitWithoutStream when set will allow client to send keepalive pings to server without any active streams(RPCs).
	PermitWithoutStream bool `json:"permit-without-stream"`

	// TODO: support custom balancer picker
}

type ClientConfig struct {
	Endpoints        []string      `json:"endpoints"`
	RequestTimeOut   time.Duration `json:"request-timeout"`
	DialTimeout      time.Duration `json:"dial-timeout"`
	KeepAliveTime    time.Duration `json:"keepalive-time"`
	KeepAliveTimeout time.Duration `json:"keepalive-timeout"`
	Secure           *SecureConfig `json:"secure"`
	Auth             *AuthConfig   `json:"auth"`
}

type SecureConfig struct {
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Cacert     string `json:"cacert"`
	ServerName string `json:"server-name"`

	InsecureTransport  bool `json:"insecure-transport"`
	InsecureSkipVerify bool `json:"insecure-skip-tls-verify"`
}

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (cc *ClientConfig) MustClient() *Client {
	cfg, err := NewClientCfg(cc.Endpoints, cc.DialTimeout, cc.KeepAliveTime, cc.KeepAliveTimeout, cc.Secure, cc.Auth)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}

	client, err := New(*cfg)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadConnection, err)
	}

	return client
}

func NewClientCfg(endpoints []string, dialTimeout, keepAliveTime, keepAliveTimeout time.Duration, scfg *SecureConfig, acfg *AuthConfig) (*Config, error) {
	// set tls if any one tls option set
	var cfgtls *transport.TLSInfo
	tlsinfo := transport.TLSInfo{}
	tlsinfo.Logger, _ = zap.NewProduction()
	if scfg.Cert != "" {
		tlsinfo.CertFile = scfg.Cert
		cfgtls = &tlsinfo
	}

	if scfg.Key != "" {
		tlsinfo.KeyFile = scfg.Key
		cfgtls = &tlsinfo
	}

	if scfg.Cacert != "" {
		tlsinfo.TrustedCAFile = scfg.Cacert
		cfgtls = &tlsinfo
	}

	if scfg.ServerName != "" {
		tlsinfo.ServerName = scfg.ServerName
		cfgtls = &tlsinfo
	}

	cfg := &Config{
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
	if cfg.TLS == nil && !scfg.InsecureTransport {
		cfg.TLS = &tls.Config{}
	}

	// If the user wants to skip TLS verification then we should set
	// the InsecureSkipVerify flag in tls configuration.
	if scfg.InsecureSkipVerify && cfg.TLS != nil {
		cfg.TLS.InsecureSkipVerify = true
	}

	if acfg != nil {
		cfg.Username = acfg.Username
		cfg.Password = acfg.Password
	}

	return cfg, nil
}
