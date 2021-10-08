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
)

func createV2store(t testing.TB, dataDirPath string) {
	t.Log("Creating not-yet v2-deprecated etcd")

	cfg := configStandalone(etcdProcessClusterConfig{enableV2: true, dataDirPath: dataDirPath, snapshotCount: 5})
	epc, err := newEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, epc.Stop())
	}()

	// We need to exceed 'snapshotCount' such that v2 snapshot is dumped.
	for i := 0; i < 10; i++ {
		if err := cURLPut(epc, cURLReq{
			endpoint: "/v2/keys/foo", value: "bar" + fmt.Sprint(i),
			expected: `{"action":"set","node":{"key":"/foo","value":"bar` + fmt.Sprint(i)}); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}
	}
}

func assertVerifyCanStartV2deprecationNotYet(t testing.TB, dataDirPath string) {
	t.Log("verify: possible to start etcd with --v2-deprecation=not-yet mode")

	cfg := configStandalone(etcdProcessClusterConfig{enableV2: true, dataDirPath: dataDirPath, v2deprecation: "not-yet", keepDataDir: true})
	epc, err := newEtcdProcessCluster(t, cfg)
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, epc.Stop())
	}()

	if err := cURLGet(epc, cURLReq{
		endpoint: "/v2/keys/foo",
		expected: `{"action":"get","node":{"key":"/foo","value":"bar9","modifiedIndex":13,"createdIndex":13}}`}); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}

}

func assertVerifyCannotStartV2deprecationWriteOnly(t testing.TB, dataDirPath string) {
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=write-only mode")
	proc, err := spawnCmd([]string{binDir + "/etcd", "--v2-deprecation=write-only", "--data-dir=" + dataDirPath}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect("detected disallowed custom content in v2store for stage --v2-deprecation=write-only")
	assert.NoError(t, err)
}

func TestV2Deprecation(t *testing.T) {
	BeforeTest(t)
	dataDirPath := t.TempDir()

	t.Run("create-storev2-data", func(t *testing.T) {
		createV2store(t, dataDirPath)
	})

	t.Run("--v2-deprecation=write-only fails", func(t *testing.T) {
		assertVerifyCannotStartV2deprecationWriteOnly(t, dataDirPath)
	})

	t.Run("--v2-deprecation=not-yet succeeds", func(t *testing.T) {
		assertVerifyCanStartV2deprecationNotYet(t, dataDirPath)
	})

}

func TestV2DeprecationWriteOnlyNoV2Api(t *testing.T) {
	BeforeTest(t)
	proc, err := spawnCmd([]string{binDir + "/etcd", "--v2-deprecation=write-only", "--enable-v2"}, nil)
	assert.NoError(t, err)

	_, err = proc.Expect("--enable-v2 and --v2-deprecation=write-only are mutually exclusive")
	assert.NoError(t, err)
}
