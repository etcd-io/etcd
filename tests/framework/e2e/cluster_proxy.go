// Copyright 2017 The etcd Authors
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

//go:build cluster_proxy

package e2e

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

type proxyEtcdProcess struct {
	*EtcdServerProcess
	// TODO(ahrtr): We need to remove `proxyV2` and v2discovery when the v2client is removed.
	proxyV2 *proxyV2Proc
	proxyV3 *proxyV3Proc
}

func NewEtcdProcess(t testing.TB, cfg *EtcdServerProcessConfig) (EtcdProcess, error) {
	return NewProxyEtcdProcess(t, cfg)
}

func NewProxyEtcdProcess(t testing.TB, cfg *EtcdServerProcessConfig) (*proxyEtcdProcess, error) {
	ep, err := NewEtcdServerProcess(t, cfg)
	if err != nil {
		return nil, err
	}
	pep := &proxyEtcdProcess{
		EtcdServerProcess: ep,
		proxyV2:           newProxyV2Proc(cfg),
		proxyV3:           newProxyV3Proc(cfg),
	}
	return pep, nil
}

func (p *proxyEtcdProcess) EndpointsHTTP() []string { return p.proxyV2.endpoints() }
func (p *proxyEtcdProcess) EndpointsGRPC() []string { return p.proxyV3.endpoints() }
func (p *proxyEtcdProcess) EndpointsMetrics() []string {
	panic("not implemented; proxy doesn't provide health information")
}

func (p *proxyEtcdProcess) Start(ctx context.Context) error {
	if err := p.EtcdServerProcess.Start(ctx); err != nil {
		return err
	}
	return p.proxyV3.Start(ctx)
}

func (p *proxyEtcdProcess) Restart(ctx context.Context) error {
	if err := p.EtcdServerProcess.Restart(ctx); err != nil {
		return err
	}
	return p.proxyV3.Restart(ctx)
}

func (p *proxyEtcdProcess) Stop() error {
	err := p.proxyV3.Stop()
	if eerr := p.EtcdServerProcess.Stop(); eerr != nil && err == nil {
		// fails on go-grpc issue #1384
		if !strings.Contains(eerr.Error(), "exit status 2") {
			err = eerr
		}
	}
	return err
}

func (p *proxyEtcdProcess) Close() error {
	err := p.proxyV3.Close()
	if eerr := p.EtcdServerProcess.Close(); eerr != nil && err == nil {
		// fails on go-grpc issue #1384
		if !strings.Contains(eerr.Error(), "exit status 2") {
			err = eerr
		}
	}
	return err
}

func (p *proxyEtcdProcess) Etcdctl(opts ...config.ClientOption) *EtcdctlV3 {
	etcdctl, err := NewEtcdctl(p.EtcdServerProcess.Config().Client, p.EtcdServerProcess.EndpointsGRPC(), opts...)
	if err != nil {
		panic(err)
	}
	return etcdctl
}

type proxyProc struct {
	lg       *zap.Logger
	name     string
	execPath string
	args     []string
	ep       string
	murl     string
	donec    chan struct{}

	proc *expect.ExpectProcess
}

func (pp *proxyProc) endpoints() []string { return []string{pp.ep} }

func (pp *proxyProc) start() error {
	if pp.proc != nil {
		panic("already started")
	}
	proc, err := SpawnCmdWithLogger(pp.lg, append([]string{pp.execPath}, pp.args...), nil, pp.name)
	if err != nil {
		return err
	}
	pp.proc = proc
	return nil
}

func (pp *proxyProc) waitReady(ctx context.Context, readyStr string) error {
	defer close(pp.donec)
	return WaitReadyExpectProc(ctx, pp.proc, []string{readyStr})
}

func (pp *proxyProc) Stop() error {
	if pp.proc == nil {
		return nil
	}
	err := pp.proc.Stop()
	if err != nil {
		return err
	}

	err = pp.proc.Close()
	if err != nil {
		// proxy received SIGTERM signal
		if !(strings.Contains(err.Error(), "unexpected exit code") ||
			// v2proxy exits with status 1 on auto tls; not sure why
			strings.Contains(err.Error(), "exit status 1")) {

			return err
		}
	}
	pp.proc = nil
	<-pp.donec
	pp.donec = make(chan struct{})
	return nil
}

func (pp *proxyProc) Close() error { return pp.Stop() }

type proxyV2Proc struct {
	proxyProc
	dataDir string
}

func proxyListenURL(cfg *EtcdServerProcessConfig, portOffset int) string {
	u, err := url.Parse(cfg.ClientURL)
	if err != nil {
		panic(err)
	}
	host, port, _ := net.SplitHostPort(u.Host)
	p, _ := strconv.ParseInt(port, 10, 16)
	u.Host = fmt.Sprintf("%s:%d", host, int(p)+portOffset)
	return u.String()
}

func newProxyV2Proc(cfg *EtcdServerProcessConfig) *proxyV2Proc {
	listenAddr := proxyListenURL(cfg, 2)
	name := fmt.Sprintf("testname-proxy-%p", cfg)
	dataDir := path.Join(cfg.DataDirPath, name+".etcd")
	args := []string{
		"--name", name,
		"--proxy", "on",
		"--listen-client-urls", listenAddr,
		"--initial-cluster", cfg.Name + "=" + cfg.PeerURL.String(),
		"--data-dir", dataDir,
	}
	return &proxyV2Proc{
		proxyProc: proxyProc{
			name:     cfg.Name,
			lg:       cfg.lg,
			execPath: cfg.ExecPath,
			args:     append(args, cfg.TlsArgs...),
			ep:       listenAddr,
			donec:    make(chan struct{}),
		},
		dataDir: dataDir,
	}
}

type proxyV3Proc struct {
	proxyProc
}

func newProxyV3Proc(cfg *EtcdServerProcessConfig) *proxyV3Proc {
	listenAddr := proxyListenURL(cfg, 3)
	args := []string{
		"grpc-proxy",
		"start",
		"--listen-addr", strings.Split(listenAddr, "/")[2],
		"--endpoints", cfg.ClientURL,
		// pass-through member RPCs
		"--advertise-client-url", "",
		"--data-dir", cfg.DataDirPath,
	}
	murl := ""
	if cfg.MetricsURL != "" {
		murl = proxyListenURL(cfg, 4)
		args = append(args, "--metrics-addr", murl)
	}
	tlsArgs := []string{}
	for i := 0; i < len(cfg.TlsArgs); i++ {
		switch cfg.TlsArgs[i] {
		case "--cert-file":
			tlsArgs = append(tlsArgs, "--cert-file", cfg.TlsArgs[i+1])
			i++
		case "--key-file":
			tlsArgs = append(tlsArgs, "--key-file", cfg.TlsArgs[i+1])
			i++
		case "--trusted-ca-file":
			tlsArgs = append(tlsArgs, "--trusted-ca-file", cfg.TlsArgs[i+1])
			i++
		case "--auto-tls":
			tlsArgs = append(tlsArgs, "--auto-tls", "--insecure-skip-tls-verify")
		case "--peer-trusted-ca-file", "--peer-cert-file", "--peer-key-file":
			i++ // skip arg
		case "--client-cert-auth", "--peer-auto-tls":
		default:
			tlsArgs = append(tlsArgs, cfg.TlsArgs[i])
		}
	}
	if len(cfg.TlsArgs) > 0 {
		// Configure certificates for connection proxy ---> server.
		// This certificate must NOT have CN set.
		tlsArgs = append(tlsArgs,
			"--cert", path.Join(FixturesDir, "client-nocn.crt"),
			"--key", path.Join(FixturesDir, "client-nocn.key.insecure"),
			"--cacert", path.Join(FixturesDir, "ca.crt"),
			"--client-crl-file", path.Join(FixturesDir, "revoke.crl"))
	}

	return &proxyV3Proc{
		proxyProc{
			name:     cfg.Name,
			lg:       cfg.lg,
			execPath: cfg.ExecPath,
			args:     append(args, tlsArgs...),
			ep:       listenAddr,
			murl:     murl,
			donec:    make(chan struct{}),
		},
	}
}

func (v3p *proxyV3Proc) Restart(ctx context.Context) error {
	if err := v3p.Stop(); err != nil {
		return err
	}
	return v3p.Start(ctx)
}

func (v3p *proxyV3Proc) Start(ctx context.Context) error {
	if err := v3p.start(); err != nil {
		return err
	}
	return v3p.waitReady(ctx, "started gRPC proxy")
}
