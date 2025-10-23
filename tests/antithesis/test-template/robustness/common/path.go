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
	"strings"
)

const (
	defaultetcd0 = "etcd0:2379"
	defaultetcd1 = "etcd1:2379"
	defaultetcd2 = "etcd2:2379"
	// mounted by the client in docker compose
	defaultetcdDataPath = "/var/etcddata%d"
	defaultReportPath   = "/var/report/"

	endpointsEnv  = "ETCD_ROBUSTNESS_ENDPOINTS"
	dataPathsEnv  = "ETCD_ROBUSTNESS_DATA_PATHS"
	reportPathEnv = "ETCD_ROBUSTNESS_REPORT_PATH"
)

func GetPaths(cfg *Config) (hosts []string, reportPath string, dataPaths map[string]string) {
	// Check for environment variable overrides first
	envDataPathsStr := os.Getenv(dataPathsEnv)
	envReportPath := os.Getenv(reportPathEnv)
	envEndpointsStr := os.Getenv(endpointsEnv)

	defaultHosts, defaultReportPath, defaultDataPaths := defaultPaths(cfg)

	hosts = defaultHosts
	if envEndpointsStr != "" {
		hosts = strings.Split(envEndpointsStr, ",")
		for i, host := range hosts {
			hosts[i] = strings.TrimSpace(host)
		}
	}

	reportPath = defaultReportPath
	if envReportPath != "" {
		reportPath = envReportPath
	}

	dataPaths = defaultDataPaths
	if envDataPathsStr != "" {
		envDataPaths := strings.Split(envDataPathsStr, ",")
		if len(envDataPaths) != len(hosts) {
			panic(fmt.Sprintf("Mismatched number of endpoints and data paths: %d endpoints, %d data paths", len(hosts), len(envDataPaths)))
		}

		dataPaths = make(map[string]string)
		for i, endpoint := range hosts {
			dataPaths[endpoint] = strings.TrimSpace(envDataPaths[i])
		}
	}
	return hosts, reportPath, dataPaths
}

func GetPathsFromEnv() (hosts []string, reportPath string, dataPaths map[string]string) {
	envDataPathsStr := os.Getenv(dataPathsEnv)
	envReportPath := os.Getenv(reportPathEnv)
	envEndpointsStr := os.Getenv(endpointsEnv)

	if envEndpointsStr == "" {
		panic(fmt.Sprintf("No endpoints specified in %s", endpointsEnv))
	}
	hosts = strings.Split(envEndpointsStr, ",")
	for i, host := range hosts {
		hosts[i] = strings.TrimSpace(host)
	}

	if envReportPath == "" {
		panic(fmt.Sprintf("No report path specified in %s", reportPathEnv))
	}
	reportPath = envReportPath

	if envDataPathsStr == "" {
		panic(fmt.Sprintf("No data paths specified in %s", dataPathsEnv))
	}

	envDataPaths := strings.Split(envDataPathsStr, ",")
	if len(envDataPaths) != len(hosts) {
		panic(fmt.Sprintf("Mismatched number of endpoints and data paths: %d endpoints, %d data paths", len(hosts), len(envDataPaths)))
	}

	dataPaths = make(map[string]string)
	for i, endpoint := range hosts {
		dataPaths[endpoint] = strings.TrimSpace(envDataPaths[i])
	}
	return hosts, reportPath, dataPaths
}

func defaultPaths(cfg *Config) (hosts []string, reportPath string, dataPaths map[string]string) {
	hosts = []string{defaultetcd0, defaultetcd1, defaultetcd2}[:cfg.NodeCount]
	reportPath = defaultReportPath
	dataPaths = etcdDataPaths(defaultetcdDataPath, cfg.NodeCount)
	return hosts, reportPath, dataPaths
}

func etcdDataPaths(dir string, amount int) map[string]string {
	dataPaths := make(map[string]string)
	for i := range amount {
		dataPaths[fmt.Sprintf("etcd%d", i)] = fmt.Sprintf(dir, i)
	}
	return dataPaths
}
