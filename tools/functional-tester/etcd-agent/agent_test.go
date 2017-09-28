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
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

var etcdPath = filepath.Join(os.Getenv("GOPATH"), "bin/etcd")

func TestAgentStart(t *testing.T) {
	defer os.Remove("etcd.log")

	a := newTestAgent(t)
	defer a.terminate()

	err := a.start()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentRestart(t *testing.T) {
	defer os.Remove("etcd.log")

	a := newTestAgent(t)
	defer a.terminate()

	err := a.start()
	if err != nil {
		t.Fatal(err)
	}

	err = a.stopWithSig(syscall.SIGTERM)
	if err != nil {
		t.Fatal(err)
	}
	err = a.restart()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAgentTerminate(t *testing.T) {
	defer os.Remove("etcd.log")

	a := newTestAgent(t)

	err := a.start()
	if err != nil {
		t.Fatal(err)
	}

	err = a.terminate()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(a.dataDir()); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// newTestAgent creates a test agent
func newTestAgent(t *testing.T) *Agent {
	a, err := newAgent(AgentConfig{EtcdPath: etcdPath, LogDir: "etcd.log"})
	if err != nil {
		t.Fatal(err)
	}

	return a
}
