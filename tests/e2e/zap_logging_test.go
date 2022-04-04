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

//go:build !cov
// +build !cov

package e2e

import (
	"encoding/json"
	"testing"
	"time"
)

func TestServerJsonLogging(t *testing.T) {
	BeforeTest(t)

	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		clusterSize:  1,
		initialToken: "new",
		logLevel:     "debug",
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	logs := epc.procs[0].Logs()
	time.Sleep(time.Second)
	if err = epc.Close(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}
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
		if _, err := time.Parse("2006-01-02T15:04:05.000Z0700", entry.Timestamp); entry.Timestamp != "" && err != nil {
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
}
