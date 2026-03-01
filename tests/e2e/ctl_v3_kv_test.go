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
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3PutClientTLSFlagByEnv(t *testing.T) {
	testCtl(t, putTest, withCfg(*e2e.NewConfigClientTLS()), withFlagByEnv())
}

func TestCtlV3GetFormat(t *testing.T) { testCtl(t, getFormatTest) }

func TestCtlV3TimeoutWhenNoProcessListensOnEndpoint(t *testing.T) {
	e2e.BeforeTest(t)

	endpoint := unusedLocalTCPAddr(t)
	cmdArgs := []string{
		e2e.BinPath.Etcdctl,
		"--debug",
		"--endpoints", endpoint,
		"--dial-timeout", "5s",
		"get", "foo",
	}
	assertEtcdctlDialTimedout(t, cmdArgs, nil)
}

func TestCtlV3TimeoutWhenTLSClientCertMissing(t *testing.T) {
	testCtl(t, timeoutWhenTLSClientCertMissingTest, withCfg(*e2e.NewConfigClientTLS()))
}

func TestCtlV3GetRevokedCRL(t *testing.T) {
	cfg := e2e.NewConfig(
		e2e.WithClusterSize(1),
		e2e.WithClientConnType(e2e.ClientTLS),
		e2e.WithClientRevokeCerts(true),
		e2e.WithClientCertAuthority(true),
	)
	testCtl(t, testGetRevokedCRL, withCfg(*cfg))
}

func testGetRevokedCRL(cx ctlCtx) {
	// test reject
	require.ErrorContains(cx.t, ctlV3Put(cx, "k", "v", ""), "context deadline exceeded")

	// test accept
	cx.epc.Cfg.Client.RevokeCerts = false
	require.NoError(cx.t, ctlV3Put(cx, "k", "v", ""))
}

func timeoutWhenTLSClientCertMissingTest(cx ctlCtx) {
	cmdArgs := []string{
		e2e.BinPath.Etcdctl,
		"--debug",
		"--endpoints", strings.Join(cx.epc.EndpointsGRPC(), ","),
		"--dial-timeout", "5s",
		"get", "foo",
	}
	assertEtcdctlDialTimedout(cx.t, cmdArgs, nil)
}

func unusedLocalTCPAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return l.Addr().String()
}

func assertEtcdctlDialTimedout(t *testing.T, cmdArgs []string, envVars map[string]string) {
	t.Helper()

	proc, err := e2e.SpawnCmd(cmdArgs, envVars)
	require.NoError(t, err)
	proc.Wait()

	err = proc.Close()
	require.Error(t, err)

	out := strings.Join(proc.Lines(), "\n")
	require.Containsf(t, out, context.DeadlineExceeded.Error(),
		"expected timeout output, got close error: %v, output: %q", err, out,
	)
}

func putTest(cx ctlCtx) {
	key, value := "foo", "bar"

	if err := ctlV3Put(cx, key, value, ""); err != nil {
		if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
			cx.t.Fatalf("putTest ctlV3Put error (%v)", err)
		}
	}
	if err := ctlV3Get(cx, []string{key}, kv{key, value}); err != nil {
		if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
			cx.t.Fatalf("putTest ctlV3Get error (%v)", err)
		}
	}
}

func getFormatTest(cx ctlCtx) {
	require.NoError(cx.t, ctlV3Put(cx, "abc", "123", ""))

	tests := []struct {
		format    string
		valueOnly bool

		wstr string
	}{
		{"simple", false, "abc"},
		{"simple", true, "123"},
		{"json", false, `"kvs":[{"key":"YWJj"`},
		{"protobuf", false, "\x17\b\x93\xe7\xf6\x93\xd4ņ\xe14\x10\xed"},
	}

	for i, tt := range tests {
		cmdArgs := append(cx.PrefixArgs(), "get")
		cmdArgs = append(cmdArgs, "--write-out="+tt.format)
		if tt.valueOnly {
			cmdArgs = append(cmdArgs, "--print-value-only")
		}
		cmdArgs = append(cmdArgs, "abc")
		lines, err := e2e.RunUtilCompletion(cmdArgs, cx.envMap)
		if err != nil {
			cx.t.Errorf("#%d: error (%v)", i, err)
		}
		assert.Contains(cx.t, strings.Join(lines, "\n"), tt.wstr)
	}
}

func ctlV3Put(cx ctlCtx, key, value, leaseID string) error {
	cmdArgs := append(cx.PrefixArgs(), "put", key, value)
	if leaseID != "" {
		cmdArgs = append(cmdArgs, "--lease", leaseID)
	}
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "OK"})
}

type kv struct {
	key, val string
}

func ctlV3Get(cx ctlCtx, args []string, kvs ...kv) error {
	cmdArgs := append(cx.PrefixArgs(), "get")
	cmdArgs = append(cmdArgs, args...)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	var lines []expect.ExpectedResponse
	for _, elem := range kvs {
		lines = append(lines, expect.ExpectedResponse{Value: elem.key}, expect.ExpectedResponse{Value: elem.val})
	}
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, lines...)
}

func ctlV3Del(cx ctlCtx, args []string, num int) error {
	cmdArgs := append(cx.PrefixArgs(), "del")
	cmdArgs = append(cmdArgs, args...)
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: fmt.Sprintf("%d", num)})
}
