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

// +build cluster_proxy

package e2e

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"go.etcd.io/etcd/v3/pkg/expect"
)

type proxyEtcdProcess struct {
	etcdProc etcdProcess
	proxyV2  *proxyV2Proc
	proxyV3  *proxyV3Proc
}

func newEtcdProcess(cfg *etcdServerProcessConfig) (etcdProcess, error) {
	return newProxyEtcdProcess(cfg)
}

func newProxyEtcdProcess(cfg *etcdServerProcessConfig) (*proxyEtcdProcess, error) {
	ep, err := newEtcdServerProcess(cfg)
	if err != nil {
		return nil, err
	}
	pep := &proxyEtcdProcess{
		etcdProc: ep,
		proxyV2:  newProxyV2Proc(cfg),
		proxyV3:  newProxyV3Proc(cfg),
	}
	return pep, nil
}

func (p *proxyEtcdProcess) Config() *etcdServerProcessConfig { return p.etcdProc.Config() }

func (p *proxyEtcdProcess) EndpointsV2() []string { return p.proxyV2.endpoints() }
func (p *proxyEtcdProcess) EndpointsV3() []string { return p.proxyV3.endpoints() }
func (p *proxyEtcdProcess) EndpointsMetrics() []string {
	panic("not implemented; proxy doesn't provide health information")
}

func (p *proxyEtcdProcess) Start() error {
	if err := p.etcdProc.Start(); err != nil {
		return err
	}
	if err := p.proxyV2.Start(); err != nil {
		return err
	}
	return p.proxyV3.Start()
}

func (p *proxyEtcdProcess) Restart() error {
	if err := p.etcdProc.Restart(); err != nil {
		return err
	}
	if err := p.proxyV2.Restart(); err != nil {
		return err
	}
	return p.proxyV3.Restart()
}

func (p *proxyEtcdProcess) Stop() error {
	err := p.proxyV2.Stop()
	if v3err := p.proxyV3.Stop(); err == nil {
		err = v3err
	}
	if eerr := p.etcdProc.Stop(); eerr != nil && err == nil {
		// fails on go-grpc issue #1384
		if !strings.Contains(eerr.Error(), "exit status 2") {
			err = eerr
		}
	}
	return err
}

func (p *proxyEtcdProcess) Close() error {
	err := p.proxyV2.Close()
	if v3err := p.proxyV3.Close(); err == nil {
		err = v3err
	}
	if eerr := p.etcdProc.Close(); eerr != nil && err == nil {
		// fails on go-grpc issue #1384
		if !strings.Contains(eerr.Error(), "exit status 2") {
			err = eerr
		}
	}
	return err
}

func (p *proxyEtcdProcess) WithStopSignal(sig os.Signal) os.Signal {
	p.proxyV3.WithStopSignal(sig)
	p.proxyV3.WithStopSignal(sig)
	return p.etcdProc.WithStopSignal(sig)
}

type proxyProc struct {
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
	proc, err := spawnCmd(append([]string{pp.execPath}, pp.args...))
	if err != nil {
		return err
	}
	pp.proc = proc
	return nil
}

func (pp *proxyProc) waitReady(readyStr string) error {
	defer close(pp.donec)
	return waitReadyExpectProc(pp.proc, []string{readyStr})
}

func (pp *proxyProc) Stop() error {
	if pp.proc == nil {
		return nil
	}
	if err := pp.proc.Stop(); err != nil && !strings.Contains(err.Error(), "exit status 1") {
		// v2proxy exits with status 1 on auto tls; not sure why
		return err
	}
	pp.proc = nil
	<-pp.donec
	pp.donec = make(chan struct{})
	return nil
}

func (pp *proxyProc) WithStopSignal(sig os.Signal) os.Signal {
	ret := pp.proc.StopSignal
	pp.proc.StopSignal = sig
	return ret
}

func (pp *proxyProc) Close() error { return pp.Stop() }

type proxyV2Proc struct {
	proxyProc
	dataDir string
}

func proxyListenURL(cfg *etcdServerProcessConfig, portOffset int) string {
	u, err := url.Parse(cfg.acurl)
	if err != nil {
		panic(err)
	}
	host, port, _ := net.SplitHostPort(u.Host)
	p, _ := strconv.ParseInt(port, 10, 16)
	u.Host = fmt.Sprintf("%s:%d", host, int(p)+portOffset)
	return u.String()
}

func newProxyV2Proc(cfg *etcdServerProcessConfig) *proxyV2Proc {
	listenAddr := proxyListenURL(cfg, 2)
	name := fmt.Sprintf("testname-proxy-%p", cfg)
	args := []string{
		"--name", name,
		"--proxy", "on",
		"--listen-client-urls", listenAddr,
		"--initial-cluster", cfg.name + "=" + cfg.purl.String(),
	}
	return &proxyV2Proc{
		proxyProc{
			execPath: cfg.execPath,
			args:     append(args, cfg.tlsArgs...),
			ep:       listenAddr,
			donec:    make(chan struct{}),
		},
		name + ".etcd",
	}
}

func (v2p *proxyV2Proc) Start() error {
	os.RemoveAll(v2p.dataDir)
	if err := v2p.start(); err != nil {
		return err
	}
	// The full line we are expecting in the logs:
	// "caller":"httpproxy/director.go:65","msg":"endpoints found","endpoints":["http://localhost:20000"]}
	return v2p.waitReady("endpoints found")
}

func (v2p *proxyV2Proc) Restart() error {
	if err := v2p.Stop(); err != nil {
		return err
	}
	return v2p.Start()
}

func (v2p *proxyV2Proc) Stop() error {
	if err := v2p.proxyProc.Stop(); err != nil {
		return err
	}
	// v2 proxy caches members; avoid reuse of directory
	return os.RemoveAll(v2p.dataDir)
}

type proxyV3Proc struct {
	proxyProc
}

func newProxyV3Proc(cfg *etcdServerProcessConfig) *proxyV3Proc {
	listenAddr := proxyListenURL(cfg, 3)
	args := []string{
		"grpc-proxy",
		"start",
		"--listen-addr", strings.Split(listenAddr, "/")[2],
		"--endpoints", cfg.acurl,
		// pass-through member RPCs
		"--advertise-client-url", "",
	}
	murl := ""
	if cfg.murl != "" {
		murl = proxyListenURL(cfg, 4)
		args = append(args, "--metrics-addr", murl)
	}
	tlsArgs := []string{}
	for i := 0; i < len(cfg.tlsArgs); i++ {
		switch cfg.tlsArgs[i] {
		case "--cert-file":
			tlsArgs = append(tlsArgs, "--cert-file", cfg.tlsArgs[i+1])
			i++
		case "--key-file":
			tlsArgs = append(tlsArgs, "--key-file", cfg.tlsArgs[i+1])
			i++
		case "--trusted-ca-file":
			tlsArgs = append(tlsArgs, "--trusted-ca-file", cfg.tlsArgs[i+1])
			i++
		case "--auto-tls":
			tlsArgs = append(tlsArgs, "--auto-tls", "--insecure-skip-tls-verify")
		case "--peer-trusted-ca-file", "--peer-cert-file", "--peer-key-file":
			i++ // skip arg
		case "--client-cert-auth", "--peer-auto-tls":
		default:
			tlsArgs = append(tlsArgs, cfg.tlsArgs[i])
		}

		// Configure certificates for connection proxy ---> server.
		// This certificate must NOT have CN set.
		tlsArgs = append(tlsArgs,
			"--cert", "../fixtures/client-nocn.crt",
			"--key", "../fixtures/client-nocn.key.insecure",
			"--cacert", "../fixtures/ca.crt",
			"--client-crl-file", "../fixtures/revoke.crl")
	}
	return &proxyV3Proc{
		proxyProc{
			execPath: cfg.execPath,
			args:     append(args, tlsArgs...),
			ep:       listenAddr,
			murl:     murl,
			donec:    make(chan struct{}),
		},
	}
}

func (v3p *proxyV3Proc) Restart() error {
	if err := v3p.Stop(); err != nil {
		return err
	}
	return v3p.Start()
}

func (v3p *proxyV3Proc) Start() error {
	if err := v3p.start(); err != nil {
		return err
	}
	return v3p.waitReady("started gRPC proxy")
}
