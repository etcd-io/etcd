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

package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

const (
	stateUninitialized = "uninitialized"
	stateStarted       = "started"
	stateStopped       = "stopped"
	stateTerminated    = "terminated"
)

type Agent struct {
	state string // the state of etcd process

	cmd     *exec.Cmd
	logfile *os.File

	cfg AgentConfig

	pmu                  sync.Mutex
	advertisePortToProxy map[int]transport.Proxy
}

type AgentConfig struct {
	EtcdPath      string
	LogDir        string
	FailpointAddr string
}

func newAgent(cfg AgentConfig) (*Agent, error) {
	// check if the file exists
	_, err := os.Stat(cfg.EtcdPath)
	if err != nil {
		return nil, err
	}

	c := exec.Command(cfg.EtcdPath)

	err = fileutil.TouchDirAll(cfg.LogDir)
	if err != nil {
		return nil, err
	}

	var f *os.File
	f, err = os.Create(filepath.Join(cfg.LogDir, "etcd.log"))
	if err != nil {
		return nil, err
	}

	return &Agent{
		state:                stateUninitialized,
		cmd:                  c,
		logfile:              f,
		cfg:                  cfg,
		advertisePortToProxy: make(map[int]transport.Proxy),
	}, nil
}

// start starts a new etcd process with the given args.
func (a *Agent) start(args ...string) error {
	args = append(args, "--data-dir", a.dataDir())
	a.cmd = exec.Command(a.cmd.Path, args...)
	a.cmd.Env = []string{"GOFAIL_HTTP=" + a.cfg.FailpointAddr}
	a.cmd.Stdout = a.logfile
	a.cmd.Stderr = a.logfile
	err := a.cmd.Start()
	if err != nil {
		return err
	}

	a.state = stateStarted

	a.pmu.Lock()
	defer a.pmu.Unlock()
	if len(a.advertisePortToProxy) == 0 {
		// enough time for etcd start before setting up proxy
		time.Sleep(time.Second)
		var (
			err                    error
			s                      string
			listenClientURL        *url.URL
			advertiseClientURL     *url.URL
			advertiseClientURLPort int
			listenPeerURL          *url.URL
			advertisePeerURL       *url.URL
			advertisePeerURLPort   int
		)
		for i := range args {
			switch args[i] {
			case "--listen-client-urls":
				listenClientURL, err = url.Parse(args[i+1])
				if err != nil {
					return err
				}
			case "--advertise-client-urls":
				advertiseClientURL, err = url.Parse(args[i+1])
				if err != nil {
					return err
				}
				_, s, err = net.SplitHostPort(advertiseClientURL.Host)
				if err != nil {
					return err
				}
				advertiseClientURLPort, err = strconv.Atoi(s)
				if err != nil {
					return err
				}
			case "--listen-peer-urls":
				listenPeerURL, err = url.Parse(args[i+1])
				if err != nil {
					return err
				}
			case "--initial-advertise-peer-urls":
				advertisePeerURL, err = url.Parse(args[i+1])
				if err != nil {
					return err
				}
				_, s, err = net.SplitHostPort(advertisePeerURL.Host)
				if err != nil {
					return err
				}
				advertisePeerURLPort, err = strconv.Atoi(s)
				if err != nil {
					return err
				}
			}
		}

		clientProxy := transport.NewProxy(transport.ProxyConfig{
			From: *advertiseClientURL,
			To:   *listenClientURL,
		})
		select {
		case err = <-clientProxy.Error():
			return err
		case <-time.After(time.Second):
		}
		a.advertisePortToProxy[advertiseClientURLPort] = clientProxy

		peerProxy := transport.NewProxy(transport.ProxyConfig{
			From: *advertisePeerURL,
			To:   *listenPeerURL,
		})
		select {
		case err = <-peerProxy.Error():
			return err
		case <-time.After(time.Second):
		}
		a.advertisePortToProxy[advertisePeerURLPort] = peerProxy
	}
	return nil
}

// stop stops the existing etcd process the agent started.
func (a *Agent) stopWithSig(sig os.Signal) error {
	if a.state != stateStarted {
		return nil
	}

	a.pmu.Lock()
	if len(a.advertisePortToProxy) > 0 {
		for _, p := range a.advertisePortToProxy {
			if err := p.Close(); err != nil {
				a.pmu.Unlock()
				return err
			}
			select {
			case <-p.Done():
				// enough time to release port
				time.Sleep(time.Second)
			case <-time.After(time.Second):
			}
		}
		a.advertisePortToProxy = make(map[int]transport.Proxy)
	}
	a.pmu.Unlock()

	err := stopWithSig(a.cmd, sig)
	if err != nil {
		return err
	}

	a.state = stateStopped
	return nil
}

func stopWithSig(cmd *exec.Cmd, sig os.Signal) error {
	err := cmd.Process.Signal(sig)
	if err != nil {
		return err
	}

	errc := make(chan error)
	go func() {
		_, ew := cmd.Process.Wait()
		errc <- ew
		close(errc)
	}()

	select {
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
	case e := <-errc:
		return e
	}
	err = <-errc
	return err
}

// restart restarts the stopped etcd process.
func (a *Agent) restart() error {
	return a.start(a.cmd.Args[1:]...)
}

func (a *Agent) cleanup() error {
	// exit with stackstrace
	if err := a.stopWithSig(syscall.SIGQUIT); err != nil {
		return err
	}
	a.state = stateUninitialized

	a.logfile.Close()
	if err := archiveLogAndDataDir(a.cfg.LogDir, a.dataDir()); err != nil {
		return err
	}

	if err := fileutil.TouchDirAll(a.cfg.LogDir); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(a.cfg.LogDir, "etcd.log"))
	if err != nil {
		return err
	}
	a.logfile = f

	// https://www.kernel.org/doc/Documentation/sysctl/vm.txt
	// https://github.com/torvalds/linux/blob/master/fs/drop_caches.c
	cmd := exec.Command("/bin/sh", "-c", `echo "echo 1 > /proc/sys/vm/drop_caches" | sudo sh`)
	if err := cmd.Run(); err != nil {
		plog.Infof("error when cleaning page cache (%v)", err)
	}
	return nil
}

// terminate stops the exiting etcd process the agent started
// and removes the data dir.
func (a *Agent) terminate() error {
	err := a.stopWithSig(syscall.SIGTERM)
	if err != nil {
		return err
	}
	err = os.RemoveAll(a.dataDir())
	if err != nil {
		return err
	}
	a.state = stateTerminated
	return nil
}

func (a *Agent) dropPort(port int) error {
	a.pmu.Lock()
	defer a.pmu.Unlock()

	p, ok := a.advertisePortToProxy[port]
	if !ok {
		return fmt.Errorf("%d does not have proxy", port)
	}
	p.BlackholeTx()
	p.BlackholeRx()
	return nil
}

func (a *Agent) recoverPort(port int) error {
	a.pmu.Lock()
	defer a.pmu.Unlock()

	p, ok := a.advertisePortToProxy[port]
	if !ok {
		return fmt.Errorf("%d does not have proxy", port)
	}
	p.UnblackholeTx()
	p.UnblackholeRx()
	return nil
}

func (a *Agent) setLatency(ms, rv int) error {
	a.pmu.Lock()
	defer a.pmu.Unlock()

	if ms == 0 {
		for _, p := range a.advertisePortToProxy {
			p.UndelayTx()
			p.UndelayRx()
		}
	}
	for _, p := range a.advertisePortToProxy {
		p.DelayTx(time.Duration(ms)*time.Millisecond, time.Duration(rv)*time.Millisecond)
		p.DelayRx(time.Duration(ms)*time.Millisecond, time.Duration(rv)*time.Millisecond)
	}
	return nil
}

func (a *Agent) status() client.Status {
	return client.Status{State: a.state}
}

func (a *Agent) dataDir() string {
	return filepath.Join(a.cfg.LogDir, "etcd.data")
}

func existDir(fpath string) bool {
	st, err := os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	} else {
		return st.IsDir()
	}
	return false
}

func archiveLogAndDataDir(logDir string, datadir string) error {
	dir := filepath.Join(logDir, "failure_archive", time.Now().Format(time.RFC3339))
	if existDir(dir) {
		dir = filepath.Join(logDir, "failure_archive", time.Now().Add(time.Second).Format(time.RFC3339))
	}
	if err := fileutil.TouchDirAll(dir); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(logDir, "etcd.log"), filepath.Join(dir, "etcd.log")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(datadir, filepath.Join(dir, filepath.Base(datadir))); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
