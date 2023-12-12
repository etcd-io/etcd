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

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/pkg/v3/proxy"
	"go.uber.org/zap"
)

var (
	EtcdServerReadyLines = []string{"ready to serve client requests"}
	BinPath              string
	CtlBinPath           string
	UtlBinPath           string
)

// EtcdProcess is a process that serves etcd requests.
type EtcdProcess interface {
	EndpointsV2() []string
	EndpointsV3() []string
	EndpointsGRPC() []string
	EndpointsHTTP() []string
	EndpointsMetrics() []string

	Start() error
	Restart() error
	Stop() error
	Close() error
	WithStopSignal(sig os.Signal) os.Signal
	Config() *EtcdServerProcessConfig
	Logs() LogsExpect

	PeerProxy() proxy.Server
	Failpoints() *BinaryFailpoints
	IsRunning() bool
}

type LogsExpect interface {
	Expect(string) (string, error)
	Lines() []string
	LineCount() int
}

type EtcdServerProcess struct {
	cfg        *EtcdServerProcessConfig
	proc       *expect.ExpectProcess
	proxy      proxy.Server
	failpoints *BinaryFailpoints
	donec      chan struct{} // closed when Interact() terminates
}

type EtcdServerProcessConfig struct {
	lg       *zap.Logger
	ExecPath string
	Args     []string
	TlsArgs  []string
	EnvVars  map[string]string

	DataDirPath string
	KeepDataDir bool

	Name string

	Purl url.URL

	Acurl         string
	Murl          string
	ClientHttpUrl string

	InitialToken   string
	InitialCluster string
	GoFailPort     int
	Proxy          *proxy.ServerConfig
}

func NewEtcdServerProcess(cfg *EtcdServerProcessConfig) (*EtcdServerProcess, error) {
	if !fileutil.Exist(cfg.ExecPath) {
		return nil, fmt.Errorf("could not find etcd binary: %s", cfg.ExecPath)
	}
	if !cfg.KeepDataDir {
		if err := os.RemoveAll(cfg.DataDirPath); err != nil {
			return nil, err
		}
	}
	ep := &EtcdServerProcess{cfg: cfg, donec: make(chan struct{})}
	if cfg.GoFailPort != 0 {
		ep.failpoints = &BinaryFailpoints{member: ep}
	}
	return ep, nil
}

func (ep *EtcdServerProcess) EndpointsV2() []string   { return ep.EndpointsHTTP() }
func (ep *EtcdServerProcess) EndpointsV3() []string   { return ep.EndpointsGRPC() }
func (ep *EtcdServerProcess) EndpointsGRPC() []string { return []string{ep.cfg.Acurl} }
func (ep *EtcdServerProcess) EndpointsHTTP() []string {
	if ep.cfg.ClientHttpUrl == "" {
		return []string{ep.cfg.Acurl}
	}
	return []string{ep.cfg.ClientHttpUrl}
}
func (ep *EtcdServerProcess) EndpointsMetrics() []string { return []string{ep.cfg.Murl} }

func (ep *EtcdServerProcess) Start() error {
	if ep.proc != nil {
		panic("already started")
	}
	if ep.cfg.Proxy != nil && ep.proxy == nil {
		ep.cfg.lg.Info("starting proxy...", zap.String("name", ep.cfg.Name), zap.String("from", ep.cfg.Proxy.From.String()), zap.String("to", ep.cfg.Proxy.To.String()))
		ep.proxy = proxy.NewServer(*ep.cfg.Proxy)
		select {
		case <-ep.proxy.Ready():
		case err := <-ep.proxy.Error():
			return err
		}
	}
	ep.cfg.lg.Info("starting server...", zap.String("name", ep.cfg.Name))
	proc, err := SpawnCmdWithLogger(ep.cfg.lg, append([]string{ep.cfg.ExecPath}, ep.cfg.Args...), ep.cfg.EnvVars)
	if err != nil {
		return err
	}
	ep.proc = proc
	err = ep.waitReady()
	if err == nil {
		ep.cfg.lg.Info("started server.", zap.String("name", ep.cfg.Name))
	}
	return err
}

func (ep *EtcdServerProcess) Restart() error {
	ep.cfg.lg.Info("restaring server...", zap.String("name", ep.cfg.Name))
	if err := ep.Stop(); err != nil {
		return err
	}
	ep.donec = make(chan struct{})
	err := ep.Start()
	if err == nil {
		ep.cfg.lg.Info("restared server", zap.String("name", ep.cfg.Name))
	}
	return err
}

func (ep *EtcdServerProcess) Stop() (err error) {
	ep.cfg.lg.Info("stoping server...", zap.String("name", ep.cfg.Name))
	if ep == nil || ep.proc == nil {
		return nil
	}
	err = ep.proc.Stop()
	if err != nil {
		return err
	}
	ep.proc = nil
	<-ep.donec
	ep.donec = make(chan struct{})
	if ep.cfg.Purl.Scheme == "unix" || ep.cfg.Purl.Scheme == "unixs" {
		err = os.Remove(ep.cfg.Purl.Host + ep.cfg.Purl.Path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	ep.cfg.lg.Info("stopped server.", zap.String("name", ep.cfg.Name))
	if ep.proxy != nil {
		ep.cfg.lg.Info("stopping proxy...", zap.String("name", ep.cfg.Name))
		err = ep.proxy.Close()
		ep.proxy = nil
		if err != nil {
			return err
		}
	}
	return nil
}

func (ep *EtcdServerProcess) Close() error {
	ep.cfg.lg.Info("closing server...", zap.String("name", ep.cfg.Name))
	if err := ep.Stop(); err != nil {
		return err
	}
	if !ep.cfg.KeepDataDir {
		ep.cfg.lg.Info("removing directory", zap.String("data-dir", ep.cfg.DataDirPath))
		return os.RemoveAll(ep.cfg.DataDirPath)
	}
	return nil
}

func (ep *EtcdServerProcess) WithStopSignal(sig os.Signal) os.Signal {
	ret := ep.proc.StopSignal
	ep.proc.StopSignal = sig
	return ret
}

func (ep *EtcdServerProcess) waitReady() error {
	defer close(ep.donec)
	return WaitReadyExpectProc(ep.proc, EtcdServerReadyLines)
}

func (ep *EtcdServerProcess) Config() *EtcdServerProcessConfig { return ep.cfg }

func (ep *EtcdServerProcess) Logs() LogsExpect {
	if ep.proc == nil {
		ep.cfg.lg.Panic("Please grap logs before process is stopped")
	}
	return ep.proc
}

func (ep *EtcdServerProcess) PeerProxy() proxy.Server {
	return ep.proxy
}

func (ep *EtcdServerProcess) Failpoints() *BinaryFailpoints {
	return ep.failpoints
}

func (ep *EtcdServerProcess) IsRunning() bool {
	if ep.proc == nil {
		return false
	}

	if ep.proc.IsRunning() {
		return true
	}

	ep.cfg.lg.Info("server exited",
		zap.String("name", ep.cfg.Name))
	ep.proc = nil
	return false
}

type BinaryFailpoints struct {
	member         EtcdProcess
	availableCache map[string]string
}

func (f *BinaryFailpoints) SetupEnv(failpoint, payload string) error {
	if f.member.IsRunning() {
		return errors.New("cannot setup environment variable while process is running")
	}
	f.member.Config().EnvVars["GOFAIL_FAILPOINTS"] = fmt.Sprintf("%s=%s", failpoint, payload)
	return nil
}

func (f *BinaryFailpoints) SetupHTTP(ctx context.Context, failpoint, payload string) error {
	host := fmt.Sprintf("127.0.0.1:%d", f.member.Config().GoFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequestWithContext(ctx, "PUT", failpointUrl.String(), bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

func (f *BinaryFailpoints) DeactivateHTTP(ctx context.Context, failpoint string) error {
	host := fmt.Sprintf("127.0.0.1:%d", f.member.Config().GoFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequestWithContext(ctx, "DELETE", failpointUrl.String(), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

var httpClient = http.Client{
	Timeout: 1 * time.Second,
}

func (f *BinaryFailpoints) Enabled() bool {
	_, err := failpoints(f.member)
	if err != nil {
		return false
	}
	return true
}

func (f *BinaryFailpoints) Available(failpoint string) bool {
	if f.availableCache == nil {
		fs, err := failpoints(f.member)
		if err != nil {
			panic(err)
		}
		f.availableCache = fs
	}
	_, found := f.availableCache[failpoint]
	return found
}

func failpoints(member EtcdProcess) (map[string]string, error) {
	body, err := fetchFailpointsBody(member)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return parseFailpointsBody(body)
}

func fetchFailpointsBody(member EtcdProcess) (io.ReadCloser, error) {
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().GoFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   address,
	}
	resp, err := http.Get(failpointUrl.String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("invalid status code, %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func parseFailpointsBody(body io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	failpoints := map[string]string{}
	for _, line := range lines {
		// Format:
		// failpoint=value
		parts := strings.SplitN(line, "=", 2)
		failpoint := parts[0]
		var value string
		if len(parts) == 2 {
			value = parts[1]
		}
		failpoints[failpoint] = value
	}
	return failpoints, nil
}
