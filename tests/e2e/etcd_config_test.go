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

	"go.etcd.io/etcd/pkg/v3/expect"
)

const exampleConfigFile = "../../etcd.conf.yml.sample"

func TestEtcdExampleConfig(t *testing.T) {
	skipInShortMode(t)

	proc, err := spawnCmd([]string{binDir + "/etcd", "--config-file", exampleConfigFile})
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReadyExpectProc(proc, etcdServerReadyLines); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestEtcdMultiPeer(t *testing.T) {
	skipInShortMode(t)

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
		if err := waitReadyExpectProc(p, etcdServerReadyLines); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEtcdUnixPeers checks that etcd will boot with unix socket peers.
func TestEtcdUnixPeers(t *testing.T) {
	skipInShortMode(t)

	d, err := ioutil.TempDir("", "e1.etcd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	proc, err := spawnCmd(
		[]string{
			binDir + "/etcd",
			"--data-dir", d,
			"--name", "e1",
			"--listen-peer-urls", "unix://etcd.unix:1",
			"--initial-advertise-peer-urls", "unix://etcd.unix:1",
			"--initial-cluster", "e1=unix://etcd.unix:1",
		},
	)
	defer os.Remove("etcd.unix:1")
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReadyExpectProc(proc, etcdServerReadyLines); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

// TestEtcdPeerCNAuth checks that the inter peer auth based on CN of cert is working correctly.
func TestEtcdPeerCNAuth(t *testing.T) {
	skipInShortMode(t)

	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=https://127.0.0.1:%d", i, etcdProcessBasePort+i)
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

	// node 0 and 1 have a cert with the correct CN, node 2 doesn't
	for i := range procs {
		commonArgs := []string{
			binDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("https://127.0.0.1:%d,https://127.0.0.1:%d", etcdProcessBasePort+i, etcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", etcdProcessBasePort+i),
			"--initial-cluster", ic,
		}

		var args []string
		if i <= 1 {
			args = []string{
				"--peer-cert-file", certPath,
				"--peer-key-file", privateKeyPath,
				"--peer-client-cert-file", certPath,
				"--peer-client-key-file", privateKeyPath,
				"--peer-trusted-ca-file", caPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-cn", "example.com",
			}
		} else {
			args = []string{
				"--peer-cert-file", certPath2,
				"--peer-key-file", privateKeyPath2,
				"--peer-client-cert-file", certPath2,
				"--peer-client-key-file", privateKeyPath2,
				"--peer-trusted-ca-file", caPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-cn", "example2.com",
			}
		}

		commonArgs = append(commonArgs, args...)

		p, err := spawnCmd(commonArgs)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for i, p := range procs {
		var expect []string
		if i <= 1 {
			expect = etcdServerReadyLines
		} else {
			expect = []string{"remote error: tls: bad certificate"}
		}
		if err := waitReadyExpectProc(p, expect); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEtcdPeerNameAuth checks that the inter peer auth based on cert name validation is working correctly.
func TestEtcdPeerNameAuth(t *testing.T) {
	skipInShortMode(t)

	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=https://127.0.0.1:%d", i, etcdProcessBasePort+i)
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

	// node 0 and 1 have a cert with the correct certificate name, node 2 doesn't
	for i := range procs {
		commonArgs := []string{
			binDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("https://127.0.0.1:%d,https://127.0.0.1:%d", etcdProcessBasePort+i, etcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", etcdProcessBasePort+i),
			"--initial-cluster", ic,
		}

		var args []string
		if i <= 1 {
			args = []string{
				"--peer-cert-file", certPath,
				"--peer-key-file", privateKeyPath,
				"--peer-trusted-ca-file", caPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-hostname", "localhost",
			}
		} else {
			args = []string{
				"--peer-cert-file", certPath2,
				"--peer-key-file", privateKeyPath2,
				"--peer-trusted-ca-file", caPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-hostname", "example2.com",
			}
		}

		commonArgs = append(commonArgs, args...)

		p, err := spawnCmd(commonArgs)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for i, p := range procs {
		var expect []string
		if i <= 1 {
			expect = etcdServerReadyLines
		} else {
			expect = []string{"client certificate authentication failed"}
		}
		if err := waitReadyExpectProc(p, expect); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGrpcproxyAndCommonName(t *testing.T) {
	skipInShortMode(t)

	argsWithNonEmptyCN := []string{
		binDir + "/etcd",
		"grpc-proxy",
		"start",
		"--cert", certPath2,
		"--key", privateKeyPath2,
		"--cacert", caPath,
	}

	argsWithEmptyCN := []string{
		binDir + "/etcd",
		"grpc-proxy",
		"start",
		"--cert", certPath3,
		"--key", privateKeyPath3,
		"--cacert", caPath,
	}

	err := spawnWithExpect(argsWithNonEmptyCN, "cert has non empty Common Name")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	p, err := spawnCmd(argsWithEmptyCN)
	defer func() {
		if p != nil {
			p.Stop()
		}
	}()

	if err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapDefragFlag(t *testing.T) {
	skipInShortMode(t)

	proc, err := spawnCmd([]string{binDir + "/etcd", "--experimental-bootstrap-defrag-threshold-megabytes", "1000"})
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReadyExpectProc(proc, []string{"Skipping defragmentation"}); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}
