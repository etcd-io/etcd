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
	"fmt"
	"net/url"
	"os"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
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
	EndpointsMetrics() []string

	Start() error
	Restart() error
	Stop() error
	Close() error
	WithStopSignal(sig os.Signal) os.Signal
	Config() *EtcdServerProcessConfig
	Logs() LogsExpect
}

type LogsExpect interface {
	Expect(string) (string, error)
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

	DataDirPath string
	KeepDataDir bool

	Name string

	Purl url.URL

	Acurl string
	Murl  string

	InitialToken   string
	InitialCluster string
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

func (ep *EtcdServerProcess) Start() error {
	if ep.proc != nil {
		panic("already started")
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
