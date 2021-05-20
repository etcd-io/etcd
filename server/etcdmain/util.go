// Copyright 2017 The etcd Authors
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

package etcdmain

import (
	"fmt"
	"os"

	"go.etcd.io/etcd/client/pkg/v3/srv"
	"go.etcd.io/etcd/client/pkg/v3/transport"

	"go.uber.org/zap"
)

func discoverEndpoints(lg *zap.Logger, dns string, ca string, insecure bool, serviceName string) (s srv.SRVClients) {
	if dns == "" {
		return s
	}
	srvs, err := srv.GetClient("etcd-client", dns, serviceName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	endpoints := srvs.Endpoints

	if lg != nil {
		lg.Info(
			"discovered cluster from SRV",
			zap.String("srv-server", dns),
			zap.Strings("endpoints", endpoints),
		)
	}

	if insecure {
		return *srvs
	}
	// confirm TLS connections are good
	tlsInfo := transport.TLSInfo{
		TrustedCAFile: ca,
		ServerName:    dns,
	}

	if lg != nil {
		lg.Info(
			"validating discovered SRV endpoints",
			zap.String("srv-server", dns),
			zap.Strings("endpoints", endpoints),
		)
	}

	endpoints, err = transport.ValidateSecureEndpoints(tlsInfo, endpoints)
	if err != nil {
		if lg != nil {
			lg.Warn(
				"failed to validate discovered endpoints",
				zap.String("srv-server", dns),
				zap.Strings("endpoints", endpoints),
				zap.Error(err),
			)
		}
	} else {
		if lg != nil {
			lg.Info(
				"using validated discovered SRV endpoints",
				zap.String("srv-server", dns),
				zap.Strings("endpoints", endpoints),
			)
		}
	}

	// map endpoints back to SRVClients struct with SRV data
	eps := make(map[string]struct{})
	for _, ep := range endpoints {
		eps[ep] = struct{}{}
	}
	for i := range srvs.Endpoints {
		if _, ok := eps[srvs.Endpoints[i]]; !ok {
			continue
		}
		s.Endpoints = append(s.Endpoints, srvs.Endpoints[i])
		s.SRVs = append(s.SRVs, srvs.SRVs[i])
	}

	return s
}
