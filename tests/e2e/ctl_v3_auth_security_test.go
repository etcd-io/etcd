// Copyright 2023 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestAuth_CVE_2021_28235 verifies https://nvd.nist.gov/vuln/detail/CVE-2021-28235
func TestAuth_CVE_2021_28235(t *testing.T) {
	testCtl(t, authTest_CVE_2021_28235, withCfg(*e2e.NewConfigNoTLS()), withLogLevel("debug"))
}

func authTest_CVE_2021_28235(cx ctlCtx) {
	// create root user with root role
	rootPass := "changeme123"
	err := ctlV3User(cx, []string{"add", "root", "--interactive=false"}, "User root created", []string{rootPass})
	require.NoError(cx.t, err)
	err = ctlV3User(cx, []string{"grant-role", "root", "root"}, "Role root is granted to user root", nil)
	require.NoError(cx.t, err)
	err = ctlV3AuthEnable(cx)
	require.NoError(cx.t, err)

	// issue a put request
	cx.user, cx.pass = "root", rootPass
	err = ctlV3Put(cx, "foo", "bar", "")
	require.NoError(cx.t, err)

	// GET /debug/requests
	httpEndpoint := cx.epc.Procs[0].EndpointsHTTP()[0]
	req := e2e.CURLReq{Endpoint: "/debug/requests?fam=grpc.Recv.etcdserverpb.Auth&b=0&exp=1", Timeout: 5}
	respData, err := curl(httpEndpoint, "GET", req, e2e.ClientNonTLS)
	require.NoError(cx.t, err)

	if strings.Contains(respData, rootPass) {
		cx.t.Errorf("The root password is included in the request.\n %s", respData)
	}
}
