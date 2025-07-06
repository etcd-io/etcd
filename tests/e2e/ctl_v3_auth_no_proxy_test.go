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

// These tests depends on certificate-based authentication that is NOT supported
// by gRPC proxy.
//go:build !cluster_proxy
// +build !cluster_proxy

package e2e

import (
	"testing"
)

func TestCtlV3AuthCertCN(t *testing.T) {
	testCtl(t, authTestCertCN, withCfg(*newConfigClientTLSCertAuth()))
}
func TestCtlV3AuthCertCNAndUsername(t *testing.T) {
	testCtl(t, authTestCertCNAndUsername, withCfg(*newConfigClientTLSCertAuth()))
}
func TestCtlV3AuthCertCNAndUsernameNoPassword(t *testing.T) {
	testCtl(t, authTestCertCNAndUsernameNoPassword, withCfg(*newConfigClientTLSCertAuth()))
}
