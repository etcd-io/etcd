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
	EtcdEndpointsEnv = "ETCD_ROBUSTNESS_ENDPOINTS"
	EtcdDataPathEnv  = "ETCD_ROBUSTNESS_DATA_PATHS"
	ReportPathEnv    = "ETCD_ROBUSTNESS_REPORT_PATH"
)

func GetPaths() ([]string, map[string]string, string) {
	endpointsEnv := validateEnvVar(EtcdEndpointsEnv)
	dataPathEnv := validateEnvVar(EtcdDataPathEnv)
	reportPath := validateEnvVar(ReportPathEnv)

	etcdEndpoints := strings.Split(endpointsEnv, ",")
	paths := strings.Split(dataPathEnv, ",")

	if len(paths) != len(etcdEndpoints) {
		panic(fmt.Sprintf("number of etcd endpoints (%d) and data paths (%d) do not match", len(etcdEndpoints), len(paths)))
	}

	for i, endpoint := range etcdEndpoints {
		if strings.TrimSpace(endpoint) == "" {
			panic(fmt.Sprintf("etcd endpoint at index %d is empty", i))
		}
		if strings.TrimSpace(paths[i]) == "" {
			panic(fmt.Sprintf("data path at index %d is empty", i))
		}
		// Trim whitespace from endpoints and paths
		etcdEndpoints[i] = strings.TrimSpace(endpoint)
		paths[i] = strings.TrimSpace(paths[i])
	}

	// Build the data paths map
	dataPaths := make(map[string]string)
	for i, etcd := range etcdEndpoints {
		dataPaths[etcd] = paths[i]
	}

	return etcdEndpoints, dataPaths, reportPath
}

func validateEnvVar(envName string) string {
	value := os.Getenv(envName)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", envName))
	}
	return value
}
