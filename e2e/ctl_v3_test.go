// Copyright 2016 CoreOS, Inc.
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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/version"
)

func TestCtlV3Put(t *testing.T)          { testCtl(t, putTest) }
func TestCtlV3PutNoTLS(t *testing.T)     { testCtl(t, putTest, withCfg(configNoTLS)) }
func TestCtlV3PutClientTLS(t *testing.T) { testCtl(t, putTest, withCfg(configClientTLS)) }
func TestCtlV3PutPeerTLS(t *testing.T)   { testCtl(t, putTest, withCfg(configPeerTLS)) }
func TestCtlV3PutTimeout(t *testing.T)   { testCtl(t, putTest, withDialTimeout(0)) }

func TestCtlV3Get(t *testing.T)          { testCtl(t, getTest) }
func TestCtlV3GetNoTLS(t *testing.T)     { testCtl(t, getTest, withCfg(configNoTLS)) }
func TestCtlV3GetClientTLS(t *testing.T) { testCtl(t, getTest, withCfg(configClientTLS)) }
func TestCtlV3GetPeerTLS(t *testing.T)   { testCtl(t, getTest, withCfg(configPeerTLS)) }
func TestCtlV3GetTimeout(t *testing.T)   { testCtl(t, getTest, withDialTimeout(0)) }
func TestCtlV3GetQuorum(t *testing.T)    { testCtl(t, getTest, withQuorum()) }

func TestCtlV3GetFormat(t *testing.T) { testCtl(t, getFormatTest) }

func TestCtlV3Del(t *testing.T)          { testCtl(t, delTest) }
func TestCtlV3DelNoTLS(t *testing.T)     { testCtl(t, delTest, withCfg(configNoTLS)) }
func TestCtlV3DelClientTLS(t *testing.T) { testCtl(t, delTest, withCfg(configClientTLS)) }
func TestCtlV3DelPeerTLS(t *testing.T)   { testCtl(t, delTest, withCfg(configPeerTLS)) }
func TestCtlV3DelTimeout(t *testing.T)   { testCtl(t, delTest, withDialTimeout(0)) }

func TestCtlV3Watch(t *testing.T)          { testCtl(t, watchTest) }
func TestCtlV3WatchNoTLS(t *testing.T)     { testCtl(t, watchTest, withCfg(configNoTLS)) }
func TestCtlV3WatchClientTLS(t *testing.T) { testCtl(t, watchTest, withCfg(configClientTLS)) }
func TestCtlV3WatchPeerTLS(t *testing.T)   { testCtl(t, watchTest, withCfg(configPeerTLS)) }
func TestCtlV3WatchTimeout(t *testing.T)   { testCtl(t, watchTest, withDialTimeout(0)) }
func TestCtlV3WatchInteractive(t *testing.T) {
	testCtl(t, watchTest, withInteractive())
}
func TestCtlV3WatchInteractiveNoTLS(t *testing.T) {
	testCtl(t, watchTest, withInteractive(), withCfg(configNoTLS))
}
func TestCtlV3WatchInteractiveClientTLS(t *testing.T) {
	testCtl(t, watchTest, withInteractive(), withCfg(configClientTLS))
}
func TestCtlV3WatchInteractivePeerTLS(t *testing.T) {
	testCtl(t, watchTest, withInteractive(), withCfg(configPeerTLS))
}

func TestCtlV3TxnInteractiveSuccess(t *testing.T) {
	testCtl(t, txnTestSuccess, withInteractive())
}
func TestCtlV3TxnInteractiveSuccessNoTLS(t *testing.T) {
	testCtl(t, txnTestSuccess, withInteractive(), withCfg(configNoTLS))
}
func TestCtlV3TxnInteractiveSuccessClientTLS(t *testing.T) {
	testCtl(t, txnTestSuccess, withInteractive(), withCfg(configClientTLS))
}
func TestCtlV3TxnInteractiveSuccessPeerTLS(t *testing.T) {
	testCtl(t, txnTestSuccess, withInteractive(), withCfg(configPeerTLS))
}
func TestCtlV3TxnInteractiveFail(t *testing.T) {
	testCtl(t, txnTestFail, withInteractive())
}

func TestCtlV3Version(t *testing.T)        { testCtl(t, versionTest) }
func TestCtlV3EpHealthQuorum(t *testing.T) { testCtl(t, epHealthTest, withQuorum()) }

type ctlCtx struct {
	t   *testing.T
	cfg etcdProcessClusterConfig
	epc *etcdProcessCluster

	errc        chan error
	dialTimeout time.Duration

	quorum      bool // if true, set up 3-node cluster and linearizable read
	interactive bool
}

type ctlOption func(*ctlCtx)

func (cx *ctlCtx) applyOpts(opts []ctlOption) {
	for _, opt := range opts {
		opt(cx)
	}
}

func withCfg(cfg etcdProcessClusterConfig) ctlOption {
	return func(cx *ctlCtx) { cx.cfg = cfg }
}

func withDialTimeout(timeout time.Duration) ctlOption {
	return func(cx *ctlCtx) { cx.dialTimeout = timeout }
}

func withQuorum() ctlOption {
	return func(cx *ctlCtx) { cx.quorum = true }
}

func withInteractive() ctlOption {
	return func(cx *ctlCtx) { cx.interactive = true }
}

func setupCtlV3Test(t *testing.T, cfg etcdProcessClusterConfig, quorum bool) *etcdProcessCluster {
	mustEtcdctl(t)
	if !quorum {
		cfg = *configStandalone(cfg)
	}
	epc, err := newEtcdProcessCluster(&cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}

func testCtl(t *testing.T, testFunc func(ctlCtx), opts ...ctlOption) {
	defer testutil.AfterTest(t)

	ret := ctlCtx{
		t:           t,
		cfg:         configAutoTLS,
		errc:        make(chan error, 1),
		dialTimeout: 7 * time.Second,
	}
	ret.applyOpts(opts)

	os.Setenv("ETCDCTL_API", "3")
	ret.epc = setupCtlV3Test(ret.t, ret.cfg, ret.quorum)

	defer func() {
		os.Unsetenv("ETCDCTL_API")
		if errC := ret.epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	go testFunc(ret)

	select {
	case <-time.After(2*ret.dialTimeout + time.Second):
		if ret.dialTimeout > 0 {
			t.Fatalf("test timed out for %v", ret.dialTimeout)
		}
	case err := <-ret.errc:
		if err != nil {
			t.Fatal(err)
		}
	}
	return
}

func putTest(cx ctlCtx) {
	defer close(cx.errc)

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

func getTest(cx ctlCtx) {
	defer close(cx.errc)

	var (
		kvs    = []kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}}
		revkvs = []kv{{"key3", "val3"}, {"key2", "val2"}, {"key1", "val1"}}
	)

	tests := []struct {
		args []string

		wkv []kv
	}{
		{[]string{"key1"}, []kv{{"key1", "val1"}}},
		{[]string{"key", "--prefix"}, kvs},
		{[]string{"key", "--prefix", "--limit=2"}, kvs[:2]},
		{[]string{"key", "--prefix", "--order=ASCEND", "--sort-by=MODIFY"}, kvs},
		{[]string{"key", "--prefix", "--order=ASCEND", "--sort-by=VERSION"}, kvs},
		{[]string{"key", "--prefix", "--order=DESCEND", "--sort-by=CREATE"}, revkvs},
		{[]string{"key", "--prefix", "--order=DESCEND", "--sort-by=KEY"}, revkvs},
	}

	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("getTest #%d: ctlV3Put error (%v)", i, err)
		}
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
	defer close(cx.errc)
	if err := ctlV3Put(cx, "abc", "123", ""); err != nil {
		cx.t.Fatal(err)
	}

	tests := []struct {
		format string

		wstr string
	}{
		{"simple", "abc"},
		{"json", "\"key\":\"YWJj\""},
		{"protobuf", "\x17\b\x93\xe7\xf6\x93\xd4ņ\xe14\x10\xed"},
	}

	for i, tt := range tests {
		cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "get")
		cmdArgs = append(cmdArgs, "--write-out="+tt.format)
		cmdArgs = append(cmdArgs, "abc")
		if err := spawnWithExpect(cmdArgs, tt.wstr); err != nil {
			cx.t.Errorf("#%d: error (%v), wanted %v", i, err, tt.wstr)
		}
	}
}

func delTest(cx ctlCtx) {
	defer close(cx.errc)

	tests := []struct {
		puts []kv
		args []string

		deletedNum int
	}{
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

func watchTest(cx ctlCtx) {
	defer close(cx.errc)

	tests := []struct {
		puts []kv
		args []string

		wkv []kv
	}{
		{
			[]kv{{"sample", "value"}},
			[]string{"sample", "--rev", "1"},
			[]kv{{"sample", "value"}},
		},
		{
			[]kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}},
			[]string{"key", "--rev", "1", "--prefix"},
			[]kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}},
		},
		{
			[]kv{{"etcd", "revision_1"}, {"etcd", "revision_2"}, {"etcd", "revision_3"}},
			[]string{"etcd", "--rev", "2"},
			[]kv{{"etcd", "revision_2"}, {"etcd", "revision_3"}},
		},
	}

	for i, tt := range tests {
		go func() {
			for j := range tt.puts {
				if err := ctlV3Put(cx, tt.puts[j].key, tt.puts[j].val, ""); err != nil {
					cx.t.Fatalf("watchTest #%d-%d: ctlV3Put error (%v)", i, j, err)
				}
			}
		}()
		if err := ctlV3Watch(cx, tt.args, tt.wkv...); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Errorf("watchTest #%d: ctlV3Watch error (%v)", i, err)
			}
		}
	}
}

func versionTest(cx ctlCtx) {
	defer close(cx.errc)

	if err := ctlV3Version(cx); err != nil {
		cx.t.Fatalf("versionTest ctlV3Version error (%v)", err)
	}
}

func epHealthTest(cx ctlCtx) {
	defer close(cx.errc)

	if err := ctlV3EpHealth(cx); err != nil {
		cx.t.Fatalf("epHealthTest ctlV3EpHealth error (%v)", err)
	}
}

func txnTestSuccess(cx ctlCtx) {
	defer close(cx.errc)

	if err := ctlV3Put(cx, "key1", "value1", ""); err != nil {
		cx.t.Fatalf("txnTestSuccess ctlV3Put error (%v)", err)
	}
	if err := ctlV3Put(cx, "key2", "value2", ""); err != nil {
		cx.t.Fatalf("txnTestSuccess ctlV3Put error (%v)", err)
	}

	rqs := txnRequests{
		compare:  []string{`version("key1") = "1"`, `version("key2") = "1"`},
		ifSucess: []string{"get key1", "get key2"},
		ifFail:   []string{`put key1 "fail"`, `put key2 "fail"`},
		results:  []string{"SUCCESS", "key1", "value1", "key2", "value2"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}
}

func txnTestFail(cx ctlCtx) {
	defer close(cx.errc)

	rqs := txnRequests{
		compare:  []string{`version("key") < "0"`},
		ifSucess: []string{`put key "success"`},
		ifFail:   []string{`put key "fail"`},
		results:  []string{"FAILURE", "OK"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3PrefixArgs(clus *etcdProcessCluster, dialTimeout time.Duration) []string {
	if len(clus.proxies()) > 0 { // TODO: add proxy check as in v2
		panic("v3 proxy not implemented")
	}

	endpoints := ""
	if backends := clus.backends(); len(backends) != 0 {
		es := []string{}
		for _, b := range backends {
			es = append(es, stripSchema(b.cfg.acurl))
		}
		endpoints = strings.Join(es, ",")
	}
	cmdArgs := []string{"../bin/etcdctl", "--endpoints", endpoints, "--dial-timeout", dialTimeout.String()}
	if clus.cfg.clientTLS == clientTLS {
		cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	}
	return cmdArgs
}

func ctlV3Put(cx ctlCtx, key, value, leaseID string) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "put", key, value)
	if leaseID != "" {
		cmdArgs = append(cmdArgs, "--lease", leaseID)
	}
	return spawnWithExpect(cmdArgs, "OK")
}

type kv struct {
	key, val string
}

func ctlV3Get(cx ctlCtx, args []string, kvs ...kv) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "get")
	cmdArgs = append(cmdArgs, args...)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	var lines []string
	for _, elem := range kvs {
		lines = append(lines, elem.key, elem.val)
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func ctlV3Del(cx ctlCtx, args []string, num int) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "del")
	cmdArgs = append(cmdArgs, args...)
	return spawnWithExpects(cmdArgs, fmt.Sprintf("%d", num))
}

func ctlV3Watch(cx ctlCtx, args []string, kvs ...kv) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "watch")
	if cx.interactive {
		cmdArgs = append(cmdArgs, "--interactive")
	} else {
		cmdArgs = append(cmdArgs, args...)
	}

	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}

	if cx.interactive {
		wl := strings.Join(append([]string{"watch"}, args...), " ") + "\r"
		if err = proc.Send(wl); err != nil {
			return err
		}
	}

	for _, elem := range kvs {
		if _, err = proc.Expect(elem.key); err != nil {
			return err
		}
		if _, err = proc.Expect(elem.val); err != nil {
			return err
		}
	}
	return proc.Stop()
}

type txnRequests struct {
	compare  []string
	ifSucess []string
	ifFail   []string
	results  []string
}

func ctlV3Txn(cx ctlCtx, rqs txnRequests) error {
	// TODO: support non-interactive mode
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "txn")
	if cx.interactive {
		cmdArgs = append(cmdArgs, "--interactive")
	}
	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}
	_, err = proc.Expect("compares:")
	if err != nil {
		return err
	}
	for _, req := range rqs.compare {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	_, err = proc.Expect("success requests (get, put, delete):")
	if err != nil {
		return err
	}
	for _, req := range rqs.ifSucess {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	_, err = proc.Expect("failure requests (get, put, delete):")
	if err != nil {
		return err
	}
	for _, req := range rqs.ifFail {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	for _, line := range rqs.results {
		_, err = proc.Expect(line)
		if err != nil {
			return err
		}
	}
	return proc.Close()
}

func ctlV3Version(cx ctlCtx) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "version")
	return spawnWithExpect(cmdArgs, version.Version)
}

func ctlV3EpHealth(cx ctlCtx) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "endpoint-health")
	lines := make([]string, cx.epc.cfg.clusterSize)
	for i := range lines {
		lines[i] = "is healthy"
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func isGRPCTimedout(err error) bool {
	return strings.Contains(err.Error(), "grpc: timed out trying to connect")
}

func stripSchema(s string) string {
	if strings.HasPrefix(s, "http://") {
		s = strings.Replace(s, "http://", "", -1)
	}
	if strings.HasPrefix(s, "https://") {
		s = strings.Replace(s, "https://", "", -1)
	}
	return s
}
