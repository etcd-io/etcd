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
	"time"
)

func TestCtlV3LeaseGrantTimeToLive(t *testing.T) { testCtl(t, leaseTestGrantTimeToLive) }
func TestCtlV3LeaseGrantTimeToLiveNoTLS(t *testing.T) {
	testCtl(t, leaseTestGrantTimeToLive, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseGrantTimeToLiveClientTLS(t *testing.T) {
	testCtl(t, leaseTestGrantTimeToLive, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseGrantTimeToLiveClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestGrantTimeToLive, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseGrantTimeToLivePeerTLS(t *testing.T) {
	testCtl(t, leaseTestGrantTimeToLive, withCfg(*newConfigPeerTLS()))
}

func TestCtlV3LeaseGrantLeases(t *testing.T) { testCtl(t, leaseTestGrantLeaseListed) }
func TestCtlV3LeaseGrantLeasesNoTLS(t *testing.T) {
	testCtl(t, leaseTestGrantLeaseListed, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseGrantLeasesClientTLS(t *testing.T) {
	testCtl(t, leaseTestGrantLeaseListed, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseGrantLeasesClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestGrantLeaseListed, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseGrantLeasesPeerTLS(t *testing.T) {
	testCtl(t, leaseTestGrantLeaseListed, withCfg(*newConfigPeerTLS()))
}

func TestCtlV3LeaseTestTimeToLiveExpired(t *testing.T) { testCtl(t, leaseTestTimeToLiveExpired) }
func TestCtlV3LeaseTestTimeToLiveExpiredNoTLS(t *testing.T) {
	testCtl(t, leaseTestTimeToLiveExpired, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseTestTimeToLiveExpiredClientTLS(t *testing.T) {
	testCtl(t, leaseTestTimeToLiveExpired, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseTestTimeToLiveExpiredClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestTimeToLiveExpired, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseTestTimeToLiveExpiredPeerTLS(t *testing.T) {
	testCtl(t, leaseTestTimeToLiveExpired, withCfg(*newConfigPeerTLS()))
}

func TestCtlV3LeaseKeepAlive(t *testing.T) { testCtl(t, leaseTestKeepAlive) }
func TestCtlV3LeaseKeepAliveNoTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAlive, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseKeepAliveClientTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAlive, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseKeepAliveClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAlive, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseKeepAlivePeerTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAlive, withCfg(*newConfigPeerTLS()))
}

func TestCtlV3LeaseKeepAliveOnce(t *testing.T) { testCtl(t, leaseTestKeepAliveOnce) }
func TestCtlV3LeaseKeepAliveOnceNoTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAliveOnce, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseKeepAliveOnceClientTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAliveOnce, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseKeepAliveOnceClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAliveOnce, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseKeepAliveOncePeerTLS(t *testing.T) {
	testCtl(t, leaseTestKeepAliveOnce, withCfg(*newConfigPeerTLS()))
}

func TestCtlV3LeaseRevoke(t *testing.T) { testCtl(t, leaseTestRevoked) }
func TestCtlV3LeaseRevokeNoTLS(t *testing.T) {
	testCtl(t, leaseTestRevoked, withCfg(*newConfigNoTLS()))
}
func TestCtlV3LeaseRevokeClientTLS(t *testing.T) {
	testCtl(t, leaseTestRevoked, withCfg(*newConfigClientTLS()))
}
func TestCtlV3LeaseRevokeClientAutoTLS(t *testing.T) {
	testCtl(t, leaseTestRevoked, withCfg(*newConfigClientAutoTLS()))
}
func TestCtlV3LeaseRevokePeerTLS(t *testing.T) {
	testCtl(t, leaseTestRevoked, withCfg(*newConfigPeerTLS()))
}

func leaseTestGrantTimeToLive(cx ctlCtx) {
	id, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("leaseTestGrantTimeToLive: ctlV3LeaseGrant error (%v)", err)
	}

	cmdArgs := append(cx.PrefixArgs(), "lease", "timetolive", id, "--keys")
	proc, err := spawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		cx.t.Fatalf("leaseTestGrantTimeToLive: error (%v)", err)
	}
	line, err := proc.Expect(" granted with TTL(")
	if err != nil {
		cx.t.Fatalf("leaseTestGrantTimeToLive: error (%v)", err)
	}
	if err = proc.Close(); err != nil {
		cx.t.Fatalf("leaseTestGrantTimeToLive: error (%v)", err)
	}
	if !strings.Contains(line, ", attached keys") {
		cx.t.Fatalf("leaseTestGrantTimeToLive: expected 'attached keys', got %q", line)
	}
	if !strings.Contains(line, id) {
		cx.t.Fatalf("leaseTestGrantTimeToLive: expected leaseID %q, got %q", id, line)
	}
}

func leaseTestGrantLeaseListed(cx ctlCtx) {
	err := leaseTestGrantLeasesList(cx)
	if err != nil {
		cx.t.Fatalf("leaseTestGrantLeasesList: (%v)", err)
	}
}

func leaseTestGrantLeasesList(cx ctlCtx) error {
	id, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		return fmt.Errorf("ctlV3LeaseGrant error (%v)", err)
	}

	cmdArgs := append(cx.PrefixArgs(), "lease", "list")
	proc, err := spawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return fmt.Errorf("lease list failed (%v)", err)
	}
	_, err = proc.Expect(id)
	if err != nil {
		return fmt.Errorf("lease id not in returned list (%v)", err)
	}
	return proc.Close()
}

func leaseTestTimeToLiveExpired(cx ctlCtx) {
	err := leaseTestTimeToLiveExpire(cx, 3)
	if err != nil {
		cx.t.Fatalf("leaseTestTimeToLiveExpire: (%v)", err)
	}
}

func leaseTestTimeToLiveExpire(cx ctlCtx, ttl int) error {
	leaseID, err := ctlV3LeaseGrant(cx, ttl)
	if err != nil {
		return fmt.Errorf("ctlV3LeaseGrant error (%v)", err)
	}

	if err = ctlV3Put(cx, "key", "val", leaseID); err != nil {
		return fmt.Errorf("ctlV3Put error (%v)", err)
	}
	// eliminate false positive
	time.Sleep(time.Duration(ttl+1) * time.Second)
	cmdArgs := append(cx.PrefixArgs(), "lease", "timetolive", leaseID)
	exp := fmt.Sprintf("lease %s already expired", leaseID)
	if err = spawnWithExpectWithEnv(cmdArgs, cx.envMap, exp); err != nil {
		return fmt.Errorf("lease not properly expired: (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}); err != nil {
		return fmt.Errorf("ctlV3Get error (%v)", err)
	}
	return nil
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

func leaseTestKeepAliveOnce(cx ctlCtx) {
	// put with TTL 10 seconds and keep-alive once
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "key", "val", leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Put error (%v)", err)
	}
	if err := ctlV3LeaseKeepAliveOnce(cx, leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseKeepAliveOnce error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}, kv{"key", "val"}); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Get error (%v)", err)
	}
}

func leaseTestRevoked(cx ctlCtx) {
	err := leaseTestRevoke(cx)
	if err != nil {
		cx.t.Fatalf("leaseTestRevoke: (%v)", err)
	}
}

func leaseTestRevoke(cx ctlCtx) error {
	// put with TTL 10 seconds and revoke
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		return fmt.Errorf("ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "key", "val", leaseID); err != nil {
		return fmt.Errorf("ctlV3Put error (%v)", err)
	}
	if err := ctlV3LeaseRevoke(cx, leaseID); err != nil {
		return fmt.Errorf("ctlV3LeaseRevoke error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}); err != nil { // expect no output
		return fmt.Errorf("ctlV3Get error (%v)", err)
	}
	return nil
}

func ctlV3LeaseGrant(cx ctlCtx, ttl int) (string, error) {
	cmdArgs := append(cx.PrefixArgs(), "lease", "grant", strconv.Itoa(ttl))
	proc, err := spawnCmd(cmdArgs, cx.envMap)
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

	proc, err := spawnCmd(cmdArgs, nil)
	if err != nil {
		return err
	}

	if _, err = proc.Expect(fmt.Sprintf("lease %s keepalived with TTL(", leaseID)); err != nil {
		return err
	}
	return proc.Stop()
}

func ctlV3LeaseKeepAliveOnce(cx ctlCtx, leaseID string) error {
	cmdArgs := append(cx.PrefixArgs(), "lease", "keep-alive", "--once", leaseID)

	proc, err := spawnCmd(cmdArgs, nil)
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
	return spawnWithExpectWithEnv(cmdArgs, cx.envMap, fmt.Sprintf("lease %s revoked", leaseID))
}
