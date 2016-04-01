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
)

func TestCtlV3Put(t *testing.T)              { testCtl(t, putTest) }
func TestCtlV3PutTimeout(t *testing.T)       { testCtl(t, putTest, withDialTimeout(0)) }
func TestCtlV3PutTimeoutQuorum(t *testing.T) { testCtl(t, putTest, withDialTimeout(0), withQuorum()) }
func TestCtlV3PutAutoTLS(t *testing.T)       { testCtl(t, putTest, withCfg(configAutoTLS)) }
func TestCtlV3PutPeerTLS(t *testing.T)       { testCtl(t, putTest, withCfg(configPeerTLS)) }
func TestCtlV3PutClientTLS(t *testing.T)     { testCtl(t, putTest, withCfg(configClientTLS)) }

func TestCtlV3Watch(t *testing.T)        { testCtl(t, watchTest) }
func TestCtlV3WatchAutoTLS(t *testing.T) { testCtl(t, watchTest, withCfg(configAutoTLS)) }
func TestCtlV3WatchPeerTLS(t *testing.T) { testCtl(t, watchTest, withCfg(configPeerTLS)) }

// TODO: Watch with client TLS is not working
// func TestCtlV3WatchClientTLS(t *testing.T) {
// 	testCtl(t, watchTest, withCfg(configClientTLS))
// }

func TestCtlV3WatchInteractive(t *testing.T) { testCtl(t, watchTest, withInteractive()) }
func TestCtlV3WatchInteractiveAutoTLS(t *testing.T) {
	testCtl(t, watchTest, withInteractive(), withCfg(configAutoTLS))
}
func TestCtlV3WatchInteractivePeerTLS(t *testing.T) {
	testCtl(t, watchTest, withInteractive(), withCfg(configPeerTLS))
}

// TODO: Watch with client TLS is not working
// func TestCtlV3WatchInteractiveClientTLS(t *testing.T) {
// 	testCtl(t, watchTest, withInteractive(), withCfg(configClientTLS))
// }

type ctlCtx struct {
	t   *testing.T
	cfg etcdProcessClusterConfig
	epc *etcdProcessCluster

	errc        chan error
	dialTimeout time.Duration

	quorum        bool
	interactive   bool
	watchRevision int
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

func withWatchRevision(rev int) ctlOption {
	return func(cx *ctlCtx) { cx.watchRevision = rev }
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
		cfg:           configNoTLS,
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
	key, value := "foo", "bar"

	defer close(cx.errc)

	if err := ctlV3Put(cx, key, value); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.errc <- fmt.Errorf("put error (%v)", err)
			return
		}
	}
	if err := ctlV3Get(cx, key, value); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.errc <- fmt.Errorf("get error (%v)", err)
			return
		}
	}
}

func watchTest(cx ctlCtx) {
	key, value := "foo", "bar"

	defer close(cx.errc)

	go func() {
		if err := ctlV3Put(cx, key, value); err != nil {
			cx.t.Fatal(err)
		}
	}()

	if err := ctlV3Watch(cx, key, value); err != nil {
		if cx.dialTimeout > 0 && isGRPCTimedout(err) {
			cx.errc <- fmt.Errorf("watch error (%v)", err)
			return
		}
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

func ctlV3Get(cx ctlCtx, key, value string) error {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "get", key)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	return spawnWithExpects(cmdArgs, key, value)
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
	if err = proc.SendLine(watchLine); err != nil {
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
