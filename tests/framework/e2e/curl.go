// Copyright 2021 The etcd Authors
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
	"math/rand"
	"strings"
)

type CURLReq struct {
	Username string
	Password string

	IsTLS   bool
	Timeout int

	Endpoint string

	Value    string
	Expected string
	Header   string

	MetricsURLScheme string

	Ciphers     string
	HttpVersion string

	OutputFile string
}

// CURLPrefixArgsCluster builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func CURLPrefixArgsCluster(clus *EtcdProcessCluster, method string, req CURLReq) []string {
	member := clus.Procs[rand.Intn(clus.Cfg.ClusterSize)]
	clientURL := member.Config().Acurl
	if req.MetricsURLScheme != "" {
		clientURL = member.EndpointsMetrics()[0]
	}
	return CURLPrefixArgs(clientURL, clus.Cfg.ClientTLS, !clus.Cfg.NoCN, method, req)
}

// CURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func CURLPrefixArgs(clientURL string, connType ClientConnType, CN bool, method string, req CURLReq) []string {
	var (
		cmdArgs = []string{"curl"}
	)
	if req.HttpVersion != "" {
		cmdArgs = append(cmdArgs, "--http"+req.HttpVersion)
	}
	if req.MetricsURLScheme != "https" {
		if req.IsTLS {
			if connType != ClientTLSAndNonTLS {
				panic("should not use cURLPrefixArgsUseTLS when serving only TLS or non-TLS")
			}
			cmdArgs = append(cmdArgs, "--cacert", CaPath, "--cert", CertPath, "--key", PrivateKeyPath)
			clientURL = ToTLS(clientURL)
		} else if connType == ClientTLS {
			if CN {
				cmdArgs = append(cmdArgs, "--cacert", CaPath, "--cert", CertPath, "--key", PrivateKeyPath)
			} else {
				cmdArgs = append(cmdArgs, "--cacert", CaPath, "--cert", CertPath3, "--key", PrivateKeyPath3)
			}
		}
	}
	ep := clientURL + req.Endpoint

	if req.Username != "" || req.Password != "" {
		cmdArgs = append(cmdArgs, "-L", "-u", fmt.Sprintf("%s:%s", req.Username, req.Password), ep)
	} else {
		cmdArgs = append(cmdArgs, "-L", ep)
	}
	if req.Timeout != 0 {
		cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", req.Timeout))
	}

	if req.Header != "" {
		cmdArgs = append(cmdArgs, "-H", req.Header)
	}

	if req.Ciphers != "" {
		cmdArgs = append(cmdArgs, "--ciphers", req.Ciphers)
	}

	if req.OutputFile != "" {
		cmdArgs = append(cmdArgs, "--output", req.OutputFile)
	}

	switch method {
	case "POST", "PUT":
		dt := req.Value
		if !strings.HasPrefix(dt, "{") { // for non-JSON value
			dt = "value=" + dt
		}
		cmdArgs = append(cmdArgs, "-X", method, "-d", dt)
	}
	return cmdArgs
}

func CURLPost(clus *EtcdProcessCluster, req CURLReq) error {
	return SpawnWithExpect(CURLPrefixArgsCluster(clus, "POST", req), req.Expected)
}

func CURLPut(clus *EtcdProcessCluster, req CURLReq) error {
	return SpawnWithExpect(CURLPrefixArgsCluster(clus, "PUT", req), req.Expected)
}

func CURLGet(clus *EtcdProcessCluster, req CURLReq) error {
	return SpawnWithExpect(CURLPrefixArgsCluster(clus, "GET", req), req.Expected)
}
