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
	endpointsEnv  = "ETCD_ROBUSTNESS_ENDPOINTS"
	dataPathsEnv  = "ETCD_ROBUSTNESS_DATA_PATHS"
	reportPathEnv = "ETCD_ROBUSTNESS_REPORT_PATH"
)

func GetPaths() (hosts []string, reportPath string, dataPaths map[string]string) {
	// Check for environment variable overrides first
	envDataPathsStr := os.Getenv(dataPathsEnv)
	envReportPath := os.Getenv(reportPathEnv)
	envEndpointsStr := os.Getenv(endpointsEnv)

	// Temporary disable to make PR simpler to review
	//revive:disable:early-return
	if envEndpointsStr != "" {
		hosts = strings.Split(envEndpointsStr, ",")
		for i, host := range hosts {
			hosts[i] = strings.TrimSpace(host)
		}
	} else {
		panic(fmt.Sprintf("No endpoints specified in %s", endpointsEnv))
	}

	if envReportPath != "" {
		reportPath = envReportPath
	} else {
		panic(fmt.Sprintf("No report path specified in %s", reportPathEnv))
	}

	if envDataPathsStr != "" {
		envDataPaths := strings.Split(envDataPathsStr, ",")
		if len(envDataPaths) != len(hosts) {
			panic(fmt.Sprintf("Mismatched number of endpoints and data paths: %d endpoints, %d data paths", len(hosts), len(envDataPaths)))
		}

		dataPaths = make(map[string]string)
		for i, endpoint := range hosts {
			dataPaths[endpoint] = strings.TrimSpace(envDataPaths[i])
		}
	} else {
		panic(fmt.Sprintf("No data paths specified in %s", dataPathsEnv))
	}
	//revive:enable:early-return
	return hosts, reportPath, dataPaths
}
