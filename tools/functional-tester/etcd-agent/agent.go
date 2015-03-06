// Copyright 2015 CoreOS, Inc.
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
	"os"
	"os/exec"
	"path"
	"time"
)

type Agent struct {
	cmd     *exec.Cmd
	logfile *os.File
	l       net.Listener
}

func newAgent(etcd string) (*Agent, error) {
	// check if the file exists
	_, err := os.Stat(etcd)
	if err != nil {
		return nil, err
	}

	c := exec.Command(etcd)

	f, err := os.Create("etcd.log")
	if err != nil {
		return nil, err
	}

	return &Agent{cmd: c, logfile: f}, nil
}

// start starts a new etcd process with the given args.
func (a *Agent) start(args ...string) error {
	a.cmd = exec.Command(a.cmd.Path, args...)
	a.cmd.Stdout = a.logfile
	a.cmd.Stderr = a.logfile
	return a.cmd.Start()
}

// stop stops the existing etcd process the agent started.
func (a *Agent) stop() error {
	err := a.cmd.Process.Kill()
	if err != nil {
		return err
	}
	_, err = a.cmd.Process.Wait()
	return err
}

// restart restarts the stopped etcd process.
func (a *Agent) restart() error {
	a.cmd = exec.Command(a.cmd.Path, a.cmd.Args[1:]...)
	a.cmd.Stdout = a.logfile
	a.cmd.Stderr = a.logfile
	return a.cmd.Start()
}

func (a *Agent) cleanup() error {
	a.stop()
	a.logfile.Close()
	if err := archiveLogAndDataDir("etcd.log", a.dataDir()); err != nil {
		return err
	}
	f, err := os.Create("etcd.log")
	a.logfile = f
	return err
}

// terminate stops the exiting etcd process the agent started
// and removes the data dir.
func (a *Agent) terminate() error {
	a.stop()
	return os.RemoveAll(a.dataDir())
}

func (a *Agent) dataDir() string {
	datadir := path.Join(a.cmd.Path, "*.etcd")
	args := a.cmd.Args
	// only parse the simple case like "-data-dir /var/lib/etcd"
	for i, arg := range args {
		if arg == "-data-dir" {
			datadir = args[i+1]
			break
		}
	}
	return datadir
}

func archiveLogAndDataDir(log string, datadir string) error {
	dir := path.Join("failure_archive", fmt.Sprint(time.Now().Format(time.RFC3339)))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Rename(log, path.Join(dir, log)); err != nil {
		return err
	}
	return os.Rename(datadir, path.Join(dir, datadir))
}
