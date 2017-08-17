// Copyright 2016 The etcd Authors
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
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/coreos/etcd/pkg/expect"
)

const exampleConfigFile = "../etcd.conf.yml.sample"

func TestEtcdExampleConfig(t *testing.T) {
	proc, err := spawnCmd([]string{binDir + "/etcd", "--config-file", exampleConfigFile})
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReadyExpectProc(proc, false); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestEtcdMultiPeer(t *testing.T) {
	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=http://127.0.0.1:%d", i, etcdProcessBasePort+i)
		d, err := ioutil.TempDir("", fmt.Sprintf("e%d.etcd", i))
		if err != nil {
			t.Fatal(err)
		}
		tmpdirs[i] = d
	}
	ic := strings.Join(peers, ",")

	procs := make([]*expect.ExpectProcess, len(peers))
	defer func() {
		for i := range procs {
			if procs[i] != nil {
				procs[i].Stop()
			}
			os.RemoveAll(tmpdirs[i])
		}
	}()
	for i := range procs {
		args := []string{
			binDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("http://127.0.0.1:%d,http://127.0.0.1:%d", etcdProcessBasePort+i, etcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("http://127.0.0.1:%d", etcdProcessBasePort+i),
			"--initial-cluster", ic,
		}
		p, err := spawnCmd(args)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for _, p := range procs {
		if err := waitReadyExpectProc(p, false); err != nil {
			t.Fatal(err)
		}
	}
}
