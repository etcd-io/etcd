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

	"go.etcd.io/etcd/pkg/expect"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/pkg/proxy"
)

var (
	etcdServerReadyLines = []string{"enabled capabilities for version", "published"}
	binPath              string
	ctlBinPath           string
)

// etcdProcess is a process that serves etcd requests.
type etcdProcess interface {
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
	Config() *etcdServerProcessConfig

	Logs() logsExpect
	PeerProxy() proxy.Server
	Failpoints() *BinaryFailpoints
	IsRunning() bool

	Etcdctl(connType clientConnType, isAutoTLS bool, v2 bool) *Etcdctl
}

type logsExpect interface {
	Expect(string) (string, error)
	Lines() []string
}

type etcdServerProcess struct {
	cfg        *etcdServerProcessConfig
	proc       *expect.ExpectProcess
	proxy      proxy.Server
	failpoints *BinaryFailpoints
	donec      chan struct{} // closed when Interact() terminates
}

type etcdServerProcessConfig struct {
	execPath string
	args     []string
	envVars  map[string]string
	tlsArgs  []string

	dataDirPath string
	keepDataDir bool

	name string

	purl url.URL

	acurl         string
	murl          string
	clientHttpUrl string

	initialToken   string
	initialCluster string

	proxy               *proxy.ServerConfig
	goFailPort          int
	goFailClientTimeout time.Duration
}

func newEtcdServerProcess(cfg *etcdServerProcessConfig) (*etcdServerProcess, error) {
	if !fileutil.Exist(cfg.execPath) {
		return nil, fmt.Errorf("could not find etcd binary")
	}
	if !cfg.keepDataDir {
		if err := os.RemoveAll(cfg.dataDirPath); err != nil {
			return nil, err
		}
	}
	ep := &etcdServerProcess{cfg: cfg, donec: make(chan struct{})}
	if cfg.goFailPort != 0 {
		ep.failpoints = &BinaryFailpoints{
			member:        ep,
			clientTimeout: cfg.goFailClientTimeout,
		}
	}

	return ep, nil
}

func (ep *etcdServerProcess) EndpointsV2() []string   { return ep.EndpointsHTTP() }
func (ep *etcdServerProcess) EndpointsV3() []string   { return ep.EndpointsGRPC() }
func (ep *etcdServerProcess) EndpointsGRPC() []string { return []string{ep.cfg.acurl} }
func (ep *etcdServerProcess) EndpointsHTTP() []string {
	if ep.cfg.clientHttpUrl == "" {
		return []string{ep.cfg.acurl}
	}
	return []string{ep.cfg.clientHttpUrl}
}
func (ep *etcdServerProcess) EndpointsMetrics() []string { return []string{ep.cfg.murl} }

func (ep *etcdServerProcess) Start() error {
	if ep.proc != nil {
		panic("already started")
	}
	if ep.cfg.proxy != nil && ep.proxy == nil {
		ep.proxy = proxy.NewServer(*ep.cfg.proxy)
		select {
		case <-ep.proxy.Ready():
		case err := <-ep.proxy.Error():
			return err
		}
	}
	proc, err := spawnCmdWithEnv(append([]string{ep.cfg.execPath}, ep.cfg.args...), ep.cfg.envVars)
	if err != nil {
		return err
	}
	ep.proc = proc
	return ep.waitReady()
}

func (ep *etcdServerProcess) Restart() error {
	if err := ep.Stop(); err != nil {
		return err
	}
	ep.donec = make(chan struct{})
	return ep.Start()
}

func (ep *etcdServerProcess) Stop() (err error) {
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
	if ep.cfg.purl.Scheme == "unix" || ep.cfg.purl.Scheme == "unixs" {
		err = os.Remove(ep.cfg.purl.Host + ep.cfg.purl.Path)
		if err != nil {
			return err
		}
	}
	if ep.proxy != nil {
		err = ep.proxy.Close()
		ep.proxy = nil
		if err != nil {
			return err
		}
	}
	return nil
}

func (ep *etcdServerProcess) Close() error {
	if err := ep.Stop(); err != nil {
		return err
	}
	return os.RemoveAll(ep.cfg.dataDirPath)
}

func (ep *etcdServerProcess) WithStopSignal(sig os.Signal) os.Signal {
	ret := ep.proc.StopSignal
	ep.proc.StopSignal = sig
	return ret
}

func (ep *etcdServerProcess) waitReady() error {
	defer close(ep.donec)
	return waitReadyExpectProc(ep.proc, etcdServerReadyLines)
}

func (ep *etcdServerProcess) Config() *etcdServerProcessConfig { return ep.cfg }

func (ep *etcdServerProcess) Logs() logsExpect {
	if ep.proc == nil {
		panic("please grab logs before process is stopped")
	}
	return ep.proc
}

func (ep *etcdServerProcess) PeerProxy() proxy.Server {
	return ep.proxy
}

func (ep *etcdServerProcess) Failpoints() *BinaryFailpoints {
	return ep.failpoints
}

func (ep *etcdServerProcess) IsRunning() bool {
	if ep.proc == nil {
		return false
	}

	if ep.proc.IsRunning() {
		return true
	}
	ep.proc = nil
	return false
}

func (ep *etcdServerProcess) Etcdctl(connType clientConnType, isAutoTLS, v2 bool) *Etcdctl {
	return NewEtcdctl(ep.EndpointsV3(), connType, isAutoTLS, v2)
}

type BinaryFailpoints struct {
	member         etcdProcess
	availableCache map[string]string
	clientTimeout  time.Duration
}

func (f *BinaryFailpoints) SetupEnv(failpoint, payload string) error {
	if f.member.IsRunning() {
		return errors.New("cannot setup environment variable while process is running")
	}
	f.member.Config().envVars["GOFAIL_FAILPOINTS"] = fmt.Sprintf("%s=%s", failpoint, payload)
	return nil
}

func (f *BinaryFailpoints) SetupHTTP(ctx context.Context, failpoint, payload string) error {
	host := fmt.Sprintf("127.0.0.1:%d", f.member.Config().goFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequestWithContext(ctx, "PUT", failpointUrl.String(), bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	httpClient := http.Client{
		Timeout: 1 * time.Second,
	}
	if f.clientTimeout != 0 {
		httpClient.Timeout = f.clientTimeout
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
	host := fmt.Sprintf("127.0.0.1:%d", f.member.Config().goFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequestWithContext(ctx, "DELETE", failpointUrl.String(), nil)
	if err != nil {
		return err
	}
	httpClient := http.Client{
		Timeout: 1 * time.Second,
	}
	if f.clientTimeout != 0 {
		httpClient.Timeout = f.clientTimeout
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

func failpoints(member etcdProcess) (map[string]string, error) {
	body, err := fetchFailpointsBody(member)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return parseFailpointsBody(body)
}

func fetchFailpointsBody(member etcdProcess) (io.ReadCloser, error) {
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().goFailPort)
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
