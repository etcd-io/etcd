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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/pkg/v3/proxy"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

var (
	EtcdServerReadyLines = []string{"ready to serve client requests"}
)

// EtcdProcess is a process that serves etcd requests.
type EtcdProcess interface {
	EndpointsGRPC() []string
	EndpointsHTTP() []string
	EndpointsMetrics() []string
	Etcdctl(opts ...config.ClientOption) *EtcdctlV3

	IsRunning() bool
	Wait(ctx context.Context) error
	Start(ctx context.Context) error
	Restart(ctx context.Context) error
	Stop() error
	Close() error
	Config() *EtcdServerProcessConfig
	PeerProxy() proxy.Server
	Failpoints() *BinaryFailpoints
	LazyFS() *LazyFS
	Logs() LogsExpect
	Kill() error
}

type LogsExpect interface {
	ExpectWithContext(context.Context, expect.ExpectedResponse) (string, error)
	Lines() []string
	LineCount() int
}

type EtcdServerProcess struct {
	cfg        *EtcdServerProcessConfig
	proc       *expect.ExpectProcess
	proxy      proxy.Server
	lazyfs     *LazyFS
	failpoints *BinaryFailpoints
	donec      chan struct{} // closed when Interact() terminates
}

type EtcdServerProcessConfig struct {
	lg       *zap.Logger
	ExecPath string
	Args     []string
	TlsArgs  []string
	EnvVars  map[string]string

	Client      ClientConfig
	DataDirPath string
	KeepDataDir bool

	Name string

	PeerURL       url.URL
	ClientURL     string
	ClientHTTPURL string
	MetricsURL    string

	InitialToken   string
	InitialCluster string
	GoFailPort     int

	LazyFSEnabled bool
	Proxy         *proxy.ServerConfig
}

func NewEtcdServerProcess(t testing.TB, cfg *EtcdServerProcessConfig) (*EtcdServerProcess, error) {
	if !fileutil.Exist(cfg.ExecPath) {
		return nil, fmt.Errorf("could not find etcd binary: %s", cfg.ExecPath)
	}
	if !cfg.KeepDataDir {
		if err := os.RemoveAll(cfg.DataDirPath); err != nil {
			return nil, err
		}
		if err := os.Mkdir(cfg.DataDirPath, 0700); err != nil {
			return nil, err
		}
	}
	ep := &EtcdServerProcess{cfg: cfg, donec: make(chan struct{})}
	if cfg.GoFailPort != 0 {
		ep.failpoints = &BinaryFailpoints{member: ep}
	}
	if cfg.LazyFSEnabled {
		ep.lazyfs = newLazyFS(cfg.lg, cfg.DataDirPath, t)
	}
	return ep, nil
}

func (ep *EtcdServerProcess) EndpointsGRPC() []string { return []string{ep.cfg.ClientURL} }
func (ep *EtcdServerProcess) EndpointsHTTP() []string {
	if ep.cfg.ClientHTTPURL == "" {
		return []string{ep.cfg.ClientURL}
	}
	return []string{ep.cfg.ClientHTTPURL}
}
func (ep *EtcdServerProcess) EndpointsMetrics() []string { return []string{ep.cfg.MetricsURL} }

func (epc *EtcdServerProcess) Etcdctl(opts ...config.ClientOption) *EtcdctlV3 {
	etcdctl, err := NewEtcdctl(epc.Config().Client, epc.EndpointsGRPC(), opts...)
	if err != nil {
		panic(err)
	}
	return etcdctl
}

func (ep *EtcdServerProcess) Start(ctx context.Context) error {
	ep.donec = make(chan struct{})
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
	if ep.lazyfs != nil {
		ep.cfg.lg.Info("starting lazyfs...", zap.String("name", ep.cfg.Name))
		err := ep.lazyfs.Start(ctx)
		if err != nil {
			return err
		}
	}

	ep.cfg.lg.Info("starting server...", zap.String("name", ep.cfg.Name))
	proc, err := SpawnCmdWithLogger(ep.cfg.lg, append([]string{ep.cfg.ExecPath}, ep.cfg.Args...), ep.cfg.EnvVars, ep.cfg.Name)
	if err != nil {
		return err
	}
	ep.proc = proc
	err = ep.waitReady(ctx)
	if err == nil {
		ep.cfg.lg.Info("started server.", zap.String("name", ep.cfg.Name), zap.Int("pid", ep.proc.Pid()))
	}
	return err
}

func (ep *EtcdServerProcess) Restart(ctx context.Context) error {
	ep.cfg.lg.Info("restarting server...", zap.String("name", ep.cfg.Name))
	if err := ep.Stop(); err != nil {
		return err
	}
	err := ep.Start(ctx)
	if err == nil {
		ep.cfg.lg.Info("restarted server", zap.String("name", ep.cfg.Name))
	}
	return err
}

func (ep *EtcdServerProcess) Stop() (err error) {
	ep.cfg.lg.Info("stopping server...", zap.String("name", ep.cfg.Name))
	if ep == nil || ep.proc == nil {
		return nil
	}
	defer func() {
		ep.proc = nil
	}()

	err = ep.proc.Stop()
	if err != nil {
		return err
	}
	err = ep.proc.Close()
	if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
		return err
	}
	<-ep.donec
	ep.donec = make(chan struct{})
	if ep.cfg.PeerURL.Scheme == "unix" || ep.cfg.PeerURL.Scheme == "unixs" {
		err = os.Remove(ep.cfg.PeerURL.Host + ep.cfg.PeerURL.Path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	ep.cfg.lg.Info("stopped server.", zap.String("name", ep.cfg.Name))
	if ep.proxy != nil {
		ep.cfg.lg.Info("stopping proxy...", zap.String("name", ep.cfg.Name))
		err := ep.proxy.Close()
		ep.proxy = nil
		if err != nil {
			return err
		}
	}
	if ep.lazyfs != nil {
		ep.cfg.lg.Info("stopping lazyfs...", zap.String("name", ep.cfg.Name))
		err = ep.lazyfs.Stop()
		ep.lazyfs = nil
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

func (ep *EtcdServerProcess) waitReady(ctx context.Context) error {
	defer close(ep.donec)
	return WaitReadyExpectProc(ctx, ep.proc, EtcdServerReadyLines)
}

func (ep *EtcdServerProcess) Config() *EtcdServerProcessConfig { return ep.cfg }

func (ep *EtcdServerProcess) Logs() LogsExpect {
	if ep.proc == nil {
		ep.cfg.lg.Panic("Please grab logs before process is stopped")
	}
	return ep.proc
}

func (ep *EtcdServerProcess) Kill() error {
	ep.cfg.lg.Info("killing server...", zap.String("name", ep.cfg.Name))
	return ep.proc.Signal(syscall.SIGKILL)
}

func (ep *EtcdServerProcess) Wait(ctx context.Context) error {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if ep.proc != nil {
			ep.proc.Wait()

			exitCode, exitErr := ep.proc.ExitCode()

			ep.cfg.lg.Info("server exited",
				zap.String("name", ep.cfg.Name),
				zap.Int("code", exitCode),
				zap.Error(exitErr),
			)
		}
	}()
	select {
	case <-ch:
		ep.proc = nil
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ep *EtcdServerProcess) IsRunning() bool {
	if ep.proc == nil {
		return false
	}

	exitCode, err := ep.proc.ExitCode()
	if err == expect.ErrProcessRunning {
		return true
	}

	ep.cfg.lg.Info("server exited",
		zap.String("name", ep.cfg.Name),
		zap.Int("code", exitCode),
		zap.Error(err))
	ep.proc = nil
	return false
}

func AssertProcessLogs(t *testing.T, ep EtcdProcess, expectLog string) {
	t.Helper()
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = ep.Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: expectLog})
	if err != nil {
		t.Fatal(err)
	}
}

func (ep *EtcdServerProcess) PeerProxy() proxy.Server {
	return ep.proxy
}

func (ep *EtcdServerProcess) LazyFS() *LazyFS {
	return ep.lazyfs
}

func (ep *EtcdServerProcess) Failpoints() *BinaryFailpoints {
	return ep.failpoints
}

type BinaryFailpoints struct {
	member         EtcdProcess
	availableCache map[string]struct{}
}

func (f *BinaryFailpoints) Setup(ctx context.Context, failpoint, payload string) error {
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

var httpClient = http.Client{
	Timeout: 10 * time.Millisecond,
}

func (f *BinaryFailpoints) Available() map[string]struct{} {
	if f.availableCache == nil {
		fs, err := fetchFailpoints(f.member)
		if err != nil {
			panic(err)
		}
		f.availableCache = fs
	}
	return f.availableCache
}

func fetchFailpoints(member EtcdProcess) (map[string]struct{}, error) {
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().GoFailPort)
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   address,
	}
	resp, err := http.Get(failpointUrl.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(body), "=", "")
	failpoints := map[string]struct{}{}
	for _, f := range strings.Split(text, "\n") {
		failpoints[f] = struct{}{}
	}
	return failpoints, nil
}

func GetVersionFromBinary(binaryPath string) (*semver.Version, error) {
	lines, err := RunUtilCompletion([]string{binaryPath, "--version"}, nil)
	if err != nil {
		return nil, fmt.Errorf("could not find binary version from %s, err: %w", binaryPath, err)
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "etcd Version:") {
			versionString := strings.TrimSpace(strings.SplitAfter(line, ":")[1])
			version, err := semver.NewVersion(versionString)
			if err != nil {
				return nil, err
			}
			return &semver.Version{
				Major: version.Major,
				Minor: version.Minor,
				Patch: version.Patch,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find version in binary output of %s, lines outputted were %v", binaryPath, lines)
}
