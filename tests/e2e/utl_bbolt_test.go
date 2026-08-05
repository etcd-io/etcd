// Copyright 2026 The etcd Authors
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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestEtcdutlBboltCheck(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(1),
		e2e.WithKeepDataDir(true),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	dialTimeout := 10 * time.Second
	prefixArgs := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ","), "--dial-timeout", dialTimeout.String()}

	t.Log("Writing keys...")
	for i := 0; i < 5; i++ {
		require.NoError(t, e2e.SpawnWithExpect(append(prefixArgs, "put", fmt.Sprintf("key%d", i), fmt.Sprintf("val%d", i)), expect.ExpectedResponse{Value: "OK"}))
	}

	t.Log("Stopping the member")
	require.NoError(t, epc.Procs[0].Stop())

	dbPath := filepath.Join(epc.Procs[0].Config().DataDirPath, "member", "snap", "db")

	t.Log("Running etcdutl bbolt check")
	err = e2e.SpawnWithExpect(
		[]string{e2e.BinPath.Etcdutl, "bbolt", "check", dbPath},
		expect.ExpectedResponse{Value: "OK"},
	)
	require.NoError(t, err)
}
