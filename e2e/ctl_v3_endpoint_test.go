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

package e2e

import (
	"net/url"
	"testing"
)

func TestCtlV3EndpointHealth(t *testing.T) { testCtl(t, endpointHealthTest, withQuorum()) }
func TestCtlV3EndpointStatus(t *testing.T) { testCtl(t, endpointStatusTest, withQuorum()) }
func TestCtlV3EndpointHealthWithAuth(t *testing.T) {
	testCtl(t, endpointHealthTestWithAuth, withQuorum())
}

func endpointHealthTest(cx ctlCtx) {
	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}
}

func ctlV3EndpointHealth(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "endpoint", "health")
	lines := make([]string, cx.epc.cfg.clusterSize)
	for i := range lines {
		lines[i] = "is healthy"
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func ctlV3EndpointHealthWithKey(cx ctlCtx, key string) error {
	cmdArgs := append(cx.PrefixArgs(), "endpoint", "health", "--health-check-key", key)
	lines := make([]string, cx.epc.cfg.clusterSize)
	for i := range lines {
		lines[i] = "is healthy"
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func endpointStatusTest(cx ctlCtx) {
	if err := ctlV3EndpointStatus(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointStatus error (%v)", err)
	}
}

func ctlV3EndpointStatus(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "endpoint", "status")
	var eps []string
	for _, ep := range cx.epc.endpoints() {
		u, _ := url.Parse(ep)
		eps = append(eps, u.Host)
	}
	return spawnWithExpects(cmdArgs, eps...)
}

func ctlV3EndpointHealthFailPermissionDenied(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "endpoint", "health")
	lines := make([]string, cx.epc.cfg.clusterSize)
	for i := range lines {
		lines[i] = "is unhealthy: failed to commit proposal: etcdserver: permission denied"
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func endpointHealthTestWithAuth(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}

	// health checking with an ordinal user must fail because the user isn't granted a permission of the key "health"
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3EndpointHealthFailPermissionDenied(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}

	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "custom-key", "", false}); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3EndpointHealthWithKey(cx, "custom-key"); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}
}
