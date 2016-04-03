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
	"strconv"
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

func TestCtlV3PutTimeout(t *testing.T) { testCtl(t, putTest, withDialTimeout(0)) }

func TestCtlV3Get(t *testing.T)          { testCtl(t, getTest) }
func TestCtlV3GetNoTLS(t *testing.T)     { testCtl(t, getTest, withCfg(configNoTLS)) }
func TestCtlV3GetClientTLS(t *testing.T) { testCtl(t, getTest, withCfg(configClientTLS)) }
func TestCtlV3GetPeerTLS(t *testing.T)   { testCtl(t, getTest, withCfg(configPeerTLS)) }

func TestCtlV3GetPrefix(t *testing.T)      { testCtl(t, getTest, withPrefix()) }
func TestCtlV3GetPrefixLimit(t *testing.T) { testCtl(t, getTest, withPrefix(), withLimit(2)) }
func TestCtlV3GetQuorum(t *testing.T)      { testCtl(t, getTest, withQuorum()) }

func TestCtlV3Watch(t *testing.T)          { testCtl(t, watchTest) }
func TestCtlV3WatchNoTLS(t *testing.T)     { testCtl(t, watchTest, withCfg(configNoTLS)) }
func TestCtlV3WatchClientTLS(t *testing.T) { testCtl(t, watchTest, withCfg(configClientTLS)) }
func TestCtlV3WatchPeerTLS(t *testing.T)   { testCtl(t, watchTest, withCfg(configPeerTLS)) }

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

// TODO: watch by prefix

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

func TestCtlV3Version(t *testing.T) { testCtl(t, versionTest) }

type ctlCtx struct {
	t   *testing.T
	cfg etcdProcessClusterConfig
	epc *etcdProcessCluster

	errc        chan error
	dialTimeout time.Duration

	prefix        bool
	quorum        bool // if true, set up 3-node cluster and linearizable read
	interactive   bool
	watchRevision int

	limit int
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

func withPrefix() ctlOption {
	return func(cx *ctlCtx) { cx.prefix = true }
}

func withQuorum() ctlOption {
	return func(cx *ctlCtx) { cx.quorum = true }
}

func withInteractive() ctlOption {
	return func(cx *ctlCtx) { cx.interactive = true }
}

func withWatchRevision(rev int) ctlOption {
	return func(cx *ctlCtx) { cx.watchRevision = rev }
}

func withLimit(limit int) ctlOption {
	return func(cx *ctlCtx) { cx.limit = limit }
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

	var (
		defaultDialTimeout   = 7 * time.Second
		defaultWatchRevision = 1
	)
	ret := ctlCtx{
		t:             t,
		cfg:           configAutoTLS,
		errc:          make(chan error, 1),
		dialTimeout:   defaultDialTimeout,
		watchRevision: defaultWatchRevision,
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

	if err := ctlV3Put(cx, key, value); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.t.Fatalf("putTest error (%v)", err)
		}
	}
	if err := ctlV3Get(cx, key, kv{key, value}); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.t.Fatalf("putTest error (%v)", err)
		}
	}
}

func watchTest(cx ctlCtx) {
	defer close(cx.errc)

	key, value := "foo", "bar"

	go func() {
		if err := ctlV3Put(cx, key, value); err != nil {
			cx.t.Fatalf("watchTest error (%v)", err)
		}
	}()

	if err := ctlV3Watch(cx, key, value); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.t.Fatalf("watchTest error (%v)", err)
		}
	}
}

func versionTest(cx ctlCtx) {
	defer close(cx.errc)

	if err := ctlV3Version(cx); err != nil {
		cx.t.Fatalf("versionTest error (%v)", err)
	}
}

func getTest(cx ctlCtx) {
	defer close(cx.errc)

	var (
		kvs = []kv{{"key1", "val1"}, {"key2", "val2"}, {"key3", "val3"}}
	)

	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val); err != nil {
			if cx.dialTimeout > 0 && isGRPCTimedout(err) {
				cx.t.Fatalf("getTest error (%v)", err)
			}
		}
	}

	if cx.limit > 0 {
		kvs = kvs[:cx.limit]
	}
	keyToGet := "key"
	if !cx.prefix {
		keyToGet = "key1"
		kvs = kvs[:1]
	}
	// TODO: configure order, sort-by

	if err := ctlV3Get(cx, keyToGet, kvs...); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.t.Fatalf("getTest error (%v)", err)
		}
	}
}

func txnTestSuccess(cx ctlCtx) {
	defer close(cx.errc)

	if err := ctlV3Put(cx, "key1", "value1"); err != nil {
		cx.t.Fatalf("txnTestSuccess error (%v)", err)
	}
	if err := ctlV3Put(cx, "key2", "value2"); err != nil {
		cx.t.Fatalf("txnTestSuccess error (%v)", err)
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

func ctlV3Put(cx ctlCtx, key, value string) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "put", key, value)
	return spawnWithExpect(cmdArgs, "OK")
}

type kv struct {
	key, val string
}

func ctlV3Get(cx ctlCtx, key string, kvs ...kv) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "get", key)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	if cx.prefix {
		cmdArgs = append(cmdArgs, "--prefix")
	}
	if cx.limit > 0 {
		cmdArgs = append(cmdArgs, "--limit", strconv.Itoa(cx.limit))
	}
	// TODO: configure order, sort-by

	var lines []string
	for _, elem := range kvs {
		lines = append(lines, elem.key, elem.val)
	}
	return spawnWithExpects(cmdArgs, lines...)
}

func ctlV3Watch(cx ctlCtx, key, value string) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "watch")
	if !cx.interactive {
		if cx.watchRevision > 0 {
			cmdArgs = append(cmdArgs, "--rev", strconv.Itoa(cx.watchRevision))
		}
		cmdArgs = append(cmdArgs, key)
		return spawnWithExpects(cmdArgs, key, value)
	}
	cmdArgs = append(cmdArgs, "--interactive")
	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}
	watchLine := fmt.Sprintf("watch %s", key)
	if cx.watchRevision > 0 {
		watchLine = fmt.Sprintf("watch %s --rev %d", key, cx.watchRevision)
	}
	if err = proc.Send(watchLine + "\r"); err != nil {
		return err
	}
	_, err = proc.Expect(key)
	if err != nil {
		return err
	}
	_, err = proc.Expect(value)
	if err != nil {
		return err
	}
	return proc.Close()
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
