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
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
)

func TestCtlV3Migrate(t *testing.T) {
	BeforeTest(t)

	cfg := newConfigNoTLS()
	cfg.enableV2 = true
	epc := setupEtcdctlTest(t, cfg, false)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	keys := make([]string, 3)
	vals := make([]string, 3)
	for i := range keys {
		keys[i] = fmt.Sprintf("foo_%d", i)
		vals[i] = fmt.Sprintf("bar_%d", i)
	}
	os.Setenv("ETCDCTL_API", "2")
	for i := range keys {
		if err := etcdctlSet(epc, keys[i], vals[i]); err != nil {
			t.Fatal(err)
		}
	}

	dataDir := epc.procs[0].Config().dataDirPath
	if err := epc.Stop(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}

	os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         *newConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		epc:         epc,
	}
	if err := ctlV3Migrate(cx, dataDir, ""); err != nil {
		t.Fatal(err)
	}

	epc.procs[0].Config().keepDataDir = true
	if err := epc.Restart(); err != nil {
		t.Fatal(err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   epc.EndpointsV3(),
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	resp, err := cli.Get(context.TODO(), "test")
	if err != nil {
		t.Fatal(err)
	}
	revAfterMigrate := resp.Header.Revision
	// to ensure revision increment is continuous from migrated v2 data
	if err := ctlV3Put(cx, "test", "value", ""); err != nil {
		t.Fatal(err)
	}

	resp, err = cli.Get(context.TODO(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("len(resp.Kvs) expected 1, got %+v", resp.Kvs)
	}

	if resp.Kvs[0].CreateRevision != revAfterMigrate+1 {
		t.Fatalf("expected revision increment is continuous from migrated v2, got %d", resp.Kvs[0].CreateRevision)
	}
}

func ctlV3Migrate(cx ctlCtx, dataDir, walDir string) error {
	cmdArgs := append(cx.PrefixArgs(), "migrate", "--data-dir", dataDir, "--wal-dir", walDir)
	return spawnWithExpects(cmdArgs, "finished transforming keys")
}
