// Copyright 2025 The etcd Authors
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

//go:build cgo && amd64

package common

import (
	"fmt"
	"os"
)

const (
	defaultetcd0 = "etcd0:2379"
	defaultetcd1 = "etcd1:2379"
	defaultetcd2 = "etcd2:2379"
	// mounted by the client in docker compose
	defaultetcdDataPath = "/var/etcddata%d"
	defaultReportPath   = "/var/report/"

	localetcd0 = "127.0.0.1:12379"
	localetcd1 = "127.0.0.1:22379"
	localetcd2 = "127.0.0.1:32379"
	// used by default when running the client locally
	defaultetcdLocalDataPath = "/tmp/etcddata%d"
	localetcdDataPathEnv     = "ETCD_ROBUSTNESS_DATA_PATH"
	localReportPath          = "report"
)

func DefaultPaths() (string, []string, map[string]string) {
	hosts := []string{defaultetcd0, defaultetcd1, defaultetcd2}
	reportPath := defaultReportPath
	dataPaths := etcdDataPaths(defaultetcdDataPath, len(hosts))
	return reportPath, hosts, dataPaths
}

func LocalPaths() (string, []string, map[string]string) {
	hosts := []string{localetcd0, localetcd1, localetcd2}
	reportPath := localReportPath
	etcdDataPath := defaultetcdLocalDataPath
	envPath := os.Getenv(localetcdDataPathEnv)
	if envPath != "" {
		etcdDataPath = envPath + "%d"
	}
	dataPaths := etcdDataPaths(etcdDataPath, len(hosts))
	return reportPath, hosts, dataPaths
}

func etcdDataPaths(dir string, amount int) map[string]string {
	dataPaths := make(map[string]string)
	for i := range amount {
		dataPaths[fmt.Sprintf("etcd%d", i)] = fmt.Sprintf(dir, i)
	}
	return dataPaths
}
