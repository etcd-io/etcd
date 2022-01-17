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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func createV2store(t testing.TB, lastReleaseBinary string, dataDirPath string) {
	t.Log("Creating not-yet v2-deprecated etcd")

	cfg := e2e.ConfigStandalone(e2e.EtcdProcessClusterConfig{ExecPath: lastReleaseBinary, EnableV2: true, DataDirPath: dataDirPath, SnapshotCount: 5})
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, epc.Stop())
	}()

	// We need to exceed 'SnapshotCount' such that v2 snapshot is dumped.
	for i := 0; i < 10; i++ {
		if err := e2e.CURLPut(epc, e2e.CURLReq{
			Endpoint: "/v2/keys/foo", Value: "bar" + fmt.Sprint(i),
			Expected: `{"action":"set","node":{"key":"/foo","value":"bar` + fmt.Sprint(i)}); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}
	}
}

func assertVerifyCannotStartV2deprecationWriteOnly(t testing.TB, dataDirPath string) {
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=write-only mode")
	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--v2-deprecation=write-only", "--data-dir=" + dataDirPath}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect("detected disallowed custom content in v2store for stage --v2-deprecation=write-only")
	assert.NoError(t, err)
}

func assertVerifyCannotStartV2deprecationNotYet(t testing.TB, dataDirPath string) {
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=not-yet mode")
	proc, err := e2e.SpawnCmd([]string{e2e.BinDir + "/etcd", "--v2-deprecation=not-yet", "--data-dir=" + dataDirPath}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect(`invalid value "not-yet" for flag -v2-deprecation: invalid value "not-yet"`)
	assert.NoError(t, err)
}

func TestV2Deprecation(t *testing.T) {
	e2e.BeforeTest(t)
	dataDirPath := t.TempDir()

	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	t.Run("create-storev2-data", func(t *testing.T) {
		createV2store(t, lastReleaseBinary, dataDirPath)
	})

	t.Run("--v2-deprecation=not-yet fails", func(t *testing.T) {
		assertVerifyCannotStartV2deprecationNotYet(t, dataDirPath)
	})

	t.Run("--v2-deprecation=write-only fails", func(t *testing.T) {
		assertVerifyCannotStartV2deprecationWriteOnly(t, dataDirPath)
	})

}
