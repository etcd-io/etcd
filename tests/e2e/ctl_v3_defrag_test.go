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
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3DefragOffline(t *testing.T) {
	testCtlWithOffline(t, maintenanceInitKeys, defragOfflineTest)
}

func maintenanceInitKeys(cx ctlCtx) {
	kvs := []kv{{"key", "val1"}, {"key", "val2"}, {"key", "val3"}}
	for i := range kvs {
		require.NoError(cx.t, ctlV3Put(cx, kvs[i].key, kvs[i].val, ""))
	}
}

func ctlV3OfflineDefrag(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgsUtl(), "defrag", "--data-dir", cx.dataDir)
	lines := []expect.ExpectedResponse{{Value: "finished defragmenting directory"}}
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, lines...)
}

func defragOfflineTest(cx ctlCtx) {
	if err := ctlV3OfflineDefrag(cx); err != nil {
		cx.t.Fatalf("defragTest ctlV3Defrag error (%v)", err)
	}
}
