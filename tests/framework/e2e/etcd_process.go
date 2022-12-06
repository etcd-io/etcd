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
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

var (
	EtcdServerReadyLines = []string{"ready to serve client requests"}
)

// EtcdProcess is a process that serves etcd requests.
type EtcdProcess interface {
	EndpointsV2() []string
	EndpointsV3() []string
	EndpointsMetrics() []string
	Client(opts ...config.ClientOption) *EtcdctlV3

	IsRunning() bool
	Wait(ctx context.Context) error
	Start(ctx context.Context) error
	Restart(ctx context.Context) error
	Stop() error
	Close() error
	Config() *EtcdServerProcessConfig
	Logs() LogsExpect
	Kill() error
}

type LogsExpect interface {
	ExpectWithContext(context.Context, string) (string, error)
	Lines() []string
	LineCount() int
}

type EtcdServerProcess struct {
	cfg   *EtcdServerProcessConfig
	proc  *expect.ExpectProcess
	donec chan struct{} // closed when Interact() terminates
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

	Purl url.URL

	Acurl string
	Murl  string

	InitialToken   string
	InitialCluster string
	GoFailPort     int
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
	return &EtcdServerProcess{cfg: cfg, donec: make(chan struct{})}, nil
}

func (ep *EtcdServerProcess) EndpointsV2() []string      { return []string{ep.cfg.Acurl} }
func (ep *EtcdServerProcess) EndpointsV3() []string      { return ep.EndpointsV2() }
func (ep *EtcdServerProcess) EndpointsMetrics() []string { return []string{ep.cfg.Murl} }

func (epc *EtcdServerProcess) Client(opts ...config.ClientOption) *EtcdctlV3 {
	etcdctl, err := NewEtcdctl(epc.Config().Client, epc.EndpointsV3(), opts...)
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
	if ep.cfg.Purl.Scheme == "unix" || ep.cfg.Purl.Scheme == "unixs" {
		err = os.Remove(ep.cfg.Purl.Host + ep.cfg.Purl.Path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	ep.cfg.lg.Info("stopped server.", zap.String("name", ep.cfg.Name))
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
			ep.cfg.lg.Info("server exited", zap.String("name", ep.cfg.Name))
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
	_, err := ep.proc.ExitCode()
	if err == expect.ErrProcessRunning {
		return true
	}
	ep.cfg.lg.Info("server exited", zap.String("name", ep.cfg.Name))
	ep.proc = nil
	return false
}

func AssertProcessLogs(t *testing.T, ep EtcdProcess, expectLog string) {
	t.Helper()
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = ep.Logs().ExpectWithContext(ctx, expectLog)
	if err != nil {
		t.Fatal(err)
	}
}
