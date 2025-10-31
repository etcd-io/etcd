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
	"fmt"
	"strconv"
	"strings"
	"testing"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3LeaseKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigAutoTLS()
	enableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveDisableFastKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigAutoTLS()
	disableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveNoTLS(t *testing.T) {
	cfg := e2e.NewConfigNoTLS()
	enableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveNoTLSDisableFastKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigNoTLS()
	disableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveClientTLS(t *testing.T) {
	cfg := e2e.NewConfigClientTLS()
	enableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveClientTLSDisableFastKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigClientTLS()
	disableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveClientAutoTLS(t *testing.T) {
	cfg := e2e.NewConfigClientAutoTLS()
	enableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAliveClientAutoTLSDisableFastKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigClientAutoTLS()
	disableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAlivePeerTLS(t *testing.T) {
	cfg := e2e.NewConfigPeerTLS()
	enableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func TestCtlV3LeaseKeepAlivePeerTLSDisableFastKeepAlive(t *testing.T) {
	cfg := e2e.NewConfigPeerTLS()
	disableFastLeaseKeepAlive(cfg)
	testCtl(t, leaseTestKeepAlive, withCfg(*cfg))
}

func enableFastLeaseKeepAlive(cfg *e2e.EtcdProcessClusterConfig) {
	e2e.WithServerFeatureGate("FastLeaseKeepAlive", true)(cfg)
}

func disableFastLeaseKeepAlive(cfg *e2e.EtcdProcessClusterConfig) {
	e2e.WithServerFeatureGate("FastLeaseKeepAlive", false)(cfg)
}

func leaseTestKeepAlive(cx ctlCtx) {
	// put with TTL 10 seconds and keep-alive
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "key", "val", leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Put error (%v)", err)
	}
	if err := ctlV3LeaseKeepAlive(cx, leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseKeepAlive error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}, kv{"key", "val"}); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Get error (%v)", err)
	}
}

func ctlV3LeaseGrant(cx ctlCtx, ttl int) (string, error) {
	cmdArgs := append(cx.PrefixArgs(), "lease", "grant", strconv.Itoa(ttl))
	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return "", err
	}

	line, err := proc.Expect(" granted with TTL(")
	if err != nil {
		return "", err
	}
	if err = proc.Close(); err != nil {
		return "", err
	}

	// parse 'line LEASE_ID granted with TTL(5s)' to get lease ID
	hs := strings.Split(line, " ")
	if len(hs) < 2 {
		return "", fmt.Errorf("lease grant failed with %q", line)
	}
	return hs[1], nil
}

func ctlV3LeaseKeepAlive(cx ctlCtx, leaseID string) error {
	cmdArgs := append(cx.PrefixArgs(), "lease", "keep-alive", leaseID)

	proc, err := e2e.SpawnCmd(cmdArgs, nil)
	if err != nil {
		return err
	}

	if _, err = proc.Expect(fmt.Sprintf("lease %s keepalived with TTL(", leaseID)); err != nil {
		return err
	}
	return proc.Stop()
}

func ctlV3LeaseRevoke(cx ctlCtx, leaseID string) error {
	cmdArgs := append(cx.PrefixArgs(), "lease", "revoke", leaseID)
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: fmt.Sprintf("lease %s revoked", leaseID)})
}
