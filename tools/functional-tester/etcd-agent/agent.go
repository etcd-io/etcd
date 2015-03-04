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
	"net"
	"os"
	"os/exec"
	"path"
)

type Agent struct {
	cmd *exec.Cmd
	l   net.Listener
}

func newAgent(etcd string) (*Agent, error) {
	// check if the file exists
	_, err := os.Stat(etcd)
	if err != nil {
		return nil, err
	}

	c := exec.Command(etcd)
	return &Agent{cmd: c}, nil
}

// start starts a new etcd process with the given args.
func (a *Agent) start(args ...string) error {
	a.cmd = exec.Command(a.cmd.Path, args...)
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
	return a.cmd.Start()
}

// terminate stops the exiting etcd process the agent started
// and removes the data dir.
func (a *Agent) terminate() error {
	a.cmd.Process.Kill()
	args := a.cmd.Args

	datadir := path.Join(a.cmd.Path, "*.etcd")
	// only parse the simple case like "-data-dir /var/lib/etcd"
	for i, arg := range args {
		if arg == "-data-dir" {
			datadir = args[i+1]
			break
		}
	}
	return os.RemoveAll(datadir)
}
