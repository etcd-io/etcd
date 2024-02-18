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

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const exampleConfigFile = "../../etcd.conf.yml.sample"

func TestEtcdExampleConfig(t *testing.T) {
	e2e.SkipInShortMode(t)

	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--config-file", exampleConfigFile}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e2e.WaitReadyExpectProc(proc, e2e.EtcdServerReadyLines); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestEtcdMultiPeer(t *testing.T) {
	e2e.SkipInShortMode(t)

	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=http://127.0.0.1:%d", i, e2e.EtcdProcessBasePort+i)
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
			e2e.BinDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("http://127.0.0.1:%d,http://127.0.0.1:%d", e2e.EtcdProcessBasePort+i, e2e.EtcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("http://127.0.0.1:%d", e2e.EtcdProcessBasePort+i),
			"--initial-cluster", ic,
		}
		p, err := e2e.SpawnCmd(args, nil)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for _, p := range procs {
		if err := e2e.WaitReadyExpectProc(p, e2e.EtcdServerReadyLines); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEtcdUnixPeers checks that etcd will boot with unix socket peers.
func TestEtcdUnixPeers(t *testing.T) {
	e2e.SkipInShortMode(t)

	d, err := ioutil.TempDir("", "e1.etcd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	proc, err := e2e.SpawnCmd(
		[]string{
			e2e.BinDir + "/etcd",
			"--data-dir", d,
			"--name", "e1",
			"--listen-peer-urls", "unix://etcd.unix:1",
			"--initial-advertise-peer-urls", "unix://etcd.unix:1",
			"--initial-cluster", "e1=unix://etcd.unix:1",
		}, nil,
	)
	defer os.Remove("etcd.unix:1")
	if err != nil {
		t.Fatal(err)
	}
	if err = e2e.WaitReadyExpectProc(proc, e2e.EtcdServerReadyLines); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

// TestEtcdPeerCNAuth checks that the inter peer auth based on CN of cert is working correctly.
func TestEtcdPeerCNAuth(t *testing.T) {
	e2e.SkipInShortMode(t)

	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=https://127.0.0.1:%d", i, e2e.EtcdProcessBasePort+i)
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
			e2e.BinDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("https://127.0.0.1:%d,https://127.0.0.1:%d", e2e.EtcdProcessBasePort+i, e2e.EtcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", e2e.EtcdProcessBasePort+i),
			"--initial-cluster", ic,
		}

		var args []string
		if i <= 1 {
			args = []string{
				"--peer-cert-file", e2e.CertPath,
				"--peer-key-file", e2e.PrivateKeyPath,
				"--peer-client-cert-file", e2e.CertPath,
				"--peer-client-key-file", e2e.PrivateKeyPath,
				"--peer-trusted-ca-file", e2e.CaPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-cn", "example.com",
			}
		} else {
			args = []string{
				"--peer-cert-file", e2e.CertPath2,
				"--peer-key-file", e2e.PrivateKeyPath2,
				"--peer-client-cert-file", e2e.CertPath2,
				"--peer-client-key-file", e2e.PrivateKeyPath2,
				"--peer-trusted-ca-file", e2e.CaPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-cn", "example2.com",
			}
		}

		commonArgs = append(commonArgs, args...)

		p, err := e2e.SpawnCmd(commonArgs, nil)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for i, p := range procs {
		var expect []string
		if i <= 1 {
			expect = e2e.EtcdServerReadyLines
		} else {
			expect = []string{"remote error: tls: bad certificate"}
		}
		if err := e2e.WaitReadyExpectProc(p, expect); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEtcdPeerNameAuth checks that the inter peer auth based on cert name validation is working correctly.
func TestEtcdPeerNameAuth(t *testing.T) {
	e2e.SkipInShortMode(t)

	peers, tmpdirs := make([]string, 3), make([]string, 3)
	for i := range peers {
		peers[i] = fmt.Sprintf("e%d=https://127.0.0.1:%d", i, e2e.EtcdProcessBasePort+i)
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
			e2e.BinDir + "/etcd",
			"--name", fmt.Sprintf("e%d", i),
			"--listen-client-urls", "http://0.0.0.0:0",
			"--data-dir", tmpdirs[i],
			"--advertise-client-urls", "http://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("https://127.0.0.1:%d,https://127.0.0.1:%d", e2e.EtcdProcessBasePort+i, e2e.EtcdProcessBasePort+len(peers)+i),
			"--initial-advertise-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", e2e.EtcdProcessBasePort+i),
			"--initial-cluster", ic,
		}

		var args []string
		if i <= 1 {
			args = []string{
				"--peer-cert-file", e2e.CertPath,
				"--peer-key-file", e2e.PrivateKeyPath,
				"--peer-trusted-ca-file", e2e.CaPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-hostname", "localhost",
			}
		} else {
			args = []string{
				"--peer-cert-file", e2e.CertPath2,
				"--peer-key-file", e2e.PrivateKeyPath2,
				"--peer-trusted-ca-file", e2e.CaPath,
				"--peer-client-cert-auth",
				"--peer-cert-allowed-hostname", "example2.com",
			}
		}

		commonArgs = append(commonArgs, args...)

		p, err := e2e.SpawnCmd(commonArgs, nil)
		if err != nil {
			t.Fatal(err)
		}
		procs[i] = p
	}

	for i, p := range procs {
		var expect []string
		if i <= 1 {
			expect = e2e.EtcdServerReadyLines
		} else {
			expect = []string{"client certificate authentication failed"}
		}
		if err := e2e.WaitReadyExpectProc(p, expect); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGrpcproxyAndCommonName(t *testing.T) {
	e2e.SkipInShortMode(t)

	argsWithNonEmptyCN := []string{
		e2e.BinDir + "/etcd",
		"grpc-proxy",
		"start",
		"--cert", e2e.CertPath2,
		"--key", e2e.PrivateKeyPath2,
		"--cacert", e2e.CaPath,
	}

	argsWithEmptyCN := []string{
		e2e.BinDir + "/etcd",
		"grpc-proxy",
		"start",
		"--cert", e2e.CertPath3,
		"--key", e2e.PrivateKeyPath3,
		"--cacert", e2e.CaPath,
	}

	err := e2e.SpawnWithExpect(argsWithNonEmptyCN, "cert has non empty Common Name")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	p, err := e2e.SpawnCmd(argsWithEmptyCN, nil)
	defer func() {
		if p != nil {
			p.Stop()
		}
	}()

	if err != nil {
		t.Fatal(err)
	}
}

func TestGrpcproxyAndListenCipherSuite(t *testing.T) {
	e2e.SkipInShortMode(t)

	cases := []struct {
		name string
		args []string
	}{
		{
			name: "ArgsWithCipherSuites",
			args: []string{
				e2e.BinDir + "/etcd",
				"grpc-proxy",
				"start",
				"--listen-cipher-suites", "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
			},
		},
		{
			name: "ArgsWithoutCipherSuites",
			args: []string{
				e2e.BinDir + "/etcd",
				"grpc-proxy",
				"start",
				"--listen-cipher-suites", "",
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			pw, err := e2e.SpawnCmd(test.args, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = pw.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBootstrapDefragFlag(t *testing.T) {
	e2e.SkipInShortMode(t)

	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--experimental-bootstrap-defrag-threshold-megabytes", "1000"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e2e.WaitReadyExpectProc(proc, []string{"Skipping defragmentation"}); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestEtcdTLSVersion(t *testing.T) {
	e2e.SkipInShortMode(t)

	d := t.TempDir()
	proc, err := e2e.SpawnCmd(
		[]string{
			e2e.BinDir + "/etcd",
			"--data-dir", d,
			"--name", "e1",
			"--listen-client-urls", "https://0.0.0.0:0",
			"--advertise-client-urls", "https://0.0.0.0:0",
			"--listen-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", e2e.EtcdProcessBasePort),
			"--initial-advertise-peer-urls", fmt.Sprintf("https://127.0.0.1:%d", e2e.EtcdProcessBasePort),
			"--initial-cluster", fmt.Sprintf("e1=https://127.0.0.1:%d", e2e.EtcdProcessBasePort),
			"--peer-cert-file", e2e.CertPath,
			"--peer-key-file", e2e.PrivateKeyPath,
			"--cert-file", e2e.CertPath2,
			"--key-file", e2e.PrivateKeyPath2,

			"--tls-min-version", "TLS1.2",
			"--tls-max-version", "TLS1.3",
		}, nil,
	)
	assert.NoError(t, err)
	assert.NoError(t, e2e.WaitReadyExpectProc(proc, e2e.EtcdServerReadyLines), "did not receive expected output from etcd process")
	assert.NoError(t, proc.Stop())

}
