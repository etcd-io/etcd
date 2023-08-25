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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3PutTimeout(t *testing.T) { testCtl(t, putTest, withDialTimeout(0)) }
func TestCtlV3PutClientTLSFlagByEnv(t *testing.T) {
	testCtl(t, putTest, withCfg(*e2e.NewConfigClientTLS()), withFlagByEnv())
}
func TestCtlV3PutIgnoreValue(t *testing.T) { testCtl(t, putTestIgnoreValue) }
func TestCtlV3PutIgnoreLease(t *testing.T) { testCtl(t, putTestIgnoreLease) }

func TestCtlV3GetTimeout(t *testing.T) { testCtl(t, getTest, withDialTimeout(0)) }

func TestCtlV3GetFormat(t *testing.T)    { testCtl(t, getFormatTest) }
func TestCtlV3GetRev(t *testing.T)       { testCtl(t, getRevTest) }
func TestCtlV3GetKeysOnly(t *testing.T)  { testCtl(t, getKeysOnlyTest) }
func TestCtlV3GetCountOnly(t *testing.T) { testCtl(t, getCountOnlyTest) }

func TestCtlV3DelTimeout(t *testing.T) { testCtl(t, delTest, withDialTimeout(0)) }

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
	err := ctlV3Put(cx, "k", "v", "")
	require.ErrorContains(cx.t, err, "context deadline exceeded")

	// test accept
	cx.epc.Cfg.Client.RevokeCerts = false
	if err := ctlV3Put(cx, "k", "v", ""); err != nil {
		cx.t.Fatal(err)
	}
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

func putTestIgnoreValue(cx ctlCtx) {
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Get(cx, []string{"foo"}, kv{"foo", "bar"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Put(cx, "foo", "", "", "--ignore-value"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Get(cx, []string{"foo"}, kv{"foo", "bar"}); err != nil {
		cx.t.Fatal(err)
	}
}

func putTestIgnoreLease(cx ctlCtx) {
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "foo", "bar", leaseID); err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3Put error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"foo"}, kv{"foo", "bar"}); err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3Get error (%v)", err)
	}
	if err := ctlV3Put(cx, "foo", "bar1", "", "--ignore-lease"); err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3Put error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"foo"}, kv{"foo", "bar1"}); err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3Get error (%v)", err)
	}
	if err := ctlV3LeaseRevoke(cx, leaseID); err != nil {
		cx.t.Fatalf("putTestIgnoreLease: ctlV3LeaseRevok error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}); err != nil { // expect no output
		cx.t.Fatalf("putTestIgnoreLease: ctlV3Get error (%v)", err)
	}
}

func getTest(cx ctlCtx) {
	var (
		kvs    = []kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}}
		revkvs = []kv{{"key3", "val3"}, {"key2", "val2"}, {"key1", "val1"}}
	)
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("getTest #%d: ctlV3Put error (%v)", i, err)
		}
	}

	tests := []struct {
		args []string

		wkv []kv
	}{
		{[]string{"key1"}, []kv{{"key1", "val1"}}},
		{[]string{"", "--prefix"}, kvs},
		{[]string{"", "--from-key"}, kvs},
		{[]string{"key", "--prefix"}, kvs},
		{[]string{"key", "--prefix", "--limit=2"}, kvs[:2]},
		{[]string{"key", "--prefix", "--order=ASCEND", "--sort-by=MODIFY"}, kvs},
		{[]string{"key", "--prefix", "--order=ASCEND", "--sort-by=VERSION"}, kvs},
		{[]string{"key", "--prefix", "--sort-by=CREATE"}, kvs}, // ASCEND by default
		{[]string{"key", "--prefix", "--order=DESCEND", "--sort-by=CREATE"}, revkvs},
		{[]string{"key", "--prefix", "--order=DESCEND", "--sort-by=KEY"}, revkvs},
	}
	for i, tt := range tests {
		if err := ctlV3Get(cx, tt.args, tt.wkv...); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Errorf("getTest #%d: ctlV3Get error (%v)", i, err)
			}
		}
	}
}

func getFormatTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "abc", "123", ""); err != nil {
		cx.t.Fatal(err)
	}

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
		if err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: tt.wstr}); err != nil {
			cx.t.Errorf("#%d: error (%v), wanted %v", i, err, tt.wstr)
		}
	}
}

func getRevTest(cx ctlCtx) {
	var (
		kvs = []kv{{"key", "val1"}, {"key", "val2"}, {"key", "val3"}}
	)
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("getRevTest #%d: ctlV3Put error (%v)", i, err)
		}
	}

	tests := []struct {
		args []string

		wkv []kv
	}{
		{[]string{"key", "--rev", "2"}, kvs[:1]},
		{[]string{"key", "--rev", "3"}, kvs[1:2]},
		{[]string{"key", "--rev", "4"}, kvs[2:]},
	}

	for i, tt := range tests {
		if err := ctlV3Get(cx, tt.args, tt.wkv...); err != nil {
			cx.t.Errorf("getTest #%d: ctlV3Get error (%v)", i, err)
		}
	}
}

func getKeysOnlyTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "key", "val", ""); err != nil {
		cx.t.Fatal(err)
	}
	cmdArgs := append(cx.PrefixArgs(), []string{"get", "--keys-only", "key"}...)
	if err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "key"}); err != nil {
		cx.t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines, err := e2e.SpawnWithExpectLines(ctx, cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "key"})
	require.NoError(cx.t, err)
	require.NotContains(cx.t, lines, "val", "got value but passed --keys-only")
}

func getCountOnlyTest(cx ctlCtx) {
	cmdArgs := append(cx.PrefixArgs(), []string{"get", "--count-only", "key", "--prefix", "--write-out=fields"}...)
	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "\"Count\" : 0"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Put(cx, "key", "val", ""); err != nil {
		cx.t.Fatal(err)
	}
	cmdArgs = append(cx.PrefixArgs(), []string{"get", "--count-only", "key", "--prefix", "--write-out=fields"}...)
	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "\"Count\" : 1"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Put(cx, "key1", "val", ""); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Put(cx, "key1", "val", ""); err != nil {
		cx.t.Fatal(err)
	}
	cmdArgs = append(cx.PrefixArgs(), []string{"get", "--count-only", "key", "--prefix", "--write-out=fields"}...)
	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "\"Count\" : 2"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Put(cx, "key2", "val", ""); err != nil {
		cx.t.Fatal(err)
	}
	cmdArgs = append(cx.PrefixArgs(), []string{"get", "--count-only", "key", "--prefix", "--write-out=fields"}...)
	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "\"Count\" : 3"}); err != nil {
		cx.t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdArgs = append(cx.PrefixArgs(), []string{"get", "--count-only", "key3", "--prefix", "--write-out=fields"}...)
	lines, err := e2e.SpawnWithExpectLines(ctx, cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "\"Count\""})
	require.NoError(cx.t, err)
	require.NotContains(cx.t, lines, "\"Count\" : 3")
}

func delTest(cx ctlCtx) {
	tests := []struct {
		puts []kv
		args []string

		deletedNum int
	}{
		{ // delete all keys
			[]kv{{"foo1", "bar"}, {"foo2", "bar"}, {"foo3", "bar"}},
			[]string{"", "--prefix"},
			3,
		},
		{ // delete all keys
			[]kv{{"foo1", "bar"}, {"foo2", "bar"}, {"foo3", "bar"}},
			[]string{"", "--from-key"},
			3,
		},
		{
			[]kv{{"this", "value"}},
			[]string{"that"},
			0,
		},
		{
			[]kv{{"sample", "value"}},
			[]string{"sample"},
			1,
		},
		{
			[]kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}},
			[]string{"key", "--prefix"},
			3,
		},
		{
			[]kv{{"zoo1", "bar"}, {"zoo2", "bar2"}, {"zoo3", "bar3"}},
			[]string{"zoo1", "--from-key"},
			3,
		},
	}

	for i, tt := range tests {
		for j := range tt.puts {
			if err := ctlV3Put(cx, tt.puts[j].key, tt.puts[j].val, ""); err != nil {
				cx.t.Fatalf("delTest #%d-%d: ctlV3Put error (%v)", i, j, err)
			}
		}
		if err := ctlV3Del(cx, tt.args, tt.deletedNum); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Fatalf("delTest #%d: ctlV3Del error (%v)", i, err)
			}
		}
	}
}

func ctlV3Put(cx ctlCtx, key, value, leaseID string, flags ...string) error {
	skipValue := false
	skipLease := false
	for _, f := range flags {
		if f == "--ignore-value" {
			skipValue = true
		}
		if f == "--ignore-lease" {
			skipLease = true
		}
	}
	cmdArgs := append(cx.PrefixArgs(), "put", key)
	if !skipValue {
		cmdArgs = append(cmdArgs, value)
	}
	if leaseID != "" && !skipLease {
		cmdArgs = append(cmdArgs, "--lease", leaseID)
	}
	if len(flags) != 0 {
		cmdArgs = append(cmdArgs, flags...)
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
