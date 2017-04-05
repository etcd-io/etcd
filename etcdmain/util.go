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

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
)

func discoverEndpoints(dns string, ca string, insecure bool) (endpoints []string) {
	if dns == "" {
		return nil
	}
	endpoints, err := client.NewSRVDiscover().Discover(dns)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	plog.Infof("discovered the cluster %s from %s", endpoints, dns)
	if insecure {
		return endpoints
	}
	// confirm TLS connections are good
	tlsInfo := transport.TLSInfo{
		TrustedCAFile: ca,
		ServerName:    dns,
	}
	plog.Infof("validating discovered endpoints %v", endpoints)
	endpoints, err = transport.ValidateSecureEndpoints(tlsInfo, endpoints)
	if err != nil {
		plog.Warningf("%v", err)
	}
	plog.Infof("using discovered endpoints %v", endpoints)
	return endpoints
}
