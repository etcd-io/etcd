// Copyright 2022 The etcd Authors
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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestServerJsonLogging(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(1),
		e2e.WithLogLevel("debug"),
	)
	require.NoErrorf(t, err, "could not start etcd process cluster")
	logs := epc.Procs[0].Logs()
	time.Sleep(time.Second)
	require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	var entry logEntry
	lines := logs.Lines()
	if len(lines) == 0 {
		t.Errorf("Expected at least one log line")
	}
	for _, line := range lines {
		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Errorf("Failed to parse log line as json, err: %q, line: %s", err, line)
			continue
		}
		if entry.Level == "" {
			t.Errorf(`Missing "level" key, line: %s`, line)
		}
		if entry.Timestamp == "" {
			t.Errorf(`Missing "ts" key, line: %s`, line)
		}
		if _, err := time.Parse("2006-01-02T15:04:05.999999Z0700", entry.Timestamp); entry.Timestamp != "" && err != nil {
			t.Errorf(`Unexpected "ts" key format, err: %s`, err)
		}
		if entry.Caller == "" {
			t.Errorf(`Missing "caller" key, line: %s`, line)
		}
		if entry.Message == "" {
			t.Errorf(`Missing "message" key, line: %s`, line)
		}
	}
}

type logEntry struct {
	Level     string `json:"level"`
	Timestamp string `json:"ts"`
	Caller    string `json:"caller"`
	Message   string `json:"msg"`
	Error     string `json:"error"`
}

func TestConnectionRejectMessage(t *testing.T) {
	e2e.SkipInShortMode(t)

	testCases := []struct {
		name           string
		url            string
		expectedErrMsg string
	}{
		{
			name:           "reject client connection",
			url:            "https://127.0.0.1:2379/version",
			expectedErrMsg: "rejected connection on client endpoint",
		},
		{
			name:           "reject peer connection",
			url:            "https://127.0.0.1:2380/members",
			expectedErrMsg: "rejected connection on peer endpoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commonArgs := []string{
				e2e.BinPath.Etcd,
				"--name", "etcd1",
				"--listen-client-urls", "https://127.0.0.1:2379",
				"--advertise-client-urls", "https://127.0.0.1:2379",
				"--cert-file", e2e.CertPath,
				"--key-file", e2e.PrivateKeyPath,
				"--trusted-ca-file", e2e.CaPath,
				"--listen-peer-urls", "https://127.0.0.1:2380",
				"--initial-advertise-peer-urls", "https://127.0.0.1:2380",
				"--initial-cluster", "etcd1=https://127.0.0.1:2380",
				"--peer-cert-file", e2e.CertPath,
				"--peer-key-file", e2e.PrivateKeyPath,
				"--peer-trusted-ca-file", e2e.CaPath,
			}

			t.Log("Starting an etcd process and wait for it to get ready.")
			p, err := e2e.SpawnCmd(commonArgs, nil)
			require.NoError(t, err)
			err = e2e.WaitReadyExpectProc(t.Context(), p, e2e.EtcdServerReadyLines)
			require.NoError(t, err)
			defer func() {
				p.Stop()
				p.Close()
			}()

			t.Log("Starting a separate goroutine to verify the expected output.")
			startedCh := make(chan struct{}, 1)
			doneCh := make(chan struct{}, 1)
			go func() {
				startedCh <- struct{}{}
				verr := e2e.WaitReadyExpectProc(t.Context(), p, []string{tc.expectedErrMsg})
				assert.NoError(t, verr)
				doneCh <- struct{}{}
			}()

			// wait for the goroutine to get started
			<-startedCh

			t.Log("Running curl command to trigger the corresponding warning message.")
			curlCmdArgs := []string{"curl", "--connect-timeout", "1", "-k", tc.url}
			curlCmd, err := e2e.SpawnCmd(curlCmdArgs, nil)
			require.NoError(t, err)

			defer func() {
				curlCmd.Stop()
				curlCmd.Close()
			}()

			t.Log("Waiting for the result.")
			select {
			case <-doneCh:
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for the result")
			}
		})
	}
}
