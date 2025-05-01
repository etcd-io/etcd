// Copyright 2022 The etcd Authors
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
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestWarningApplyDuration(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(1),
		e2e.WithWarningUnaryRequestDuration(time.Microsecond),
	)
	require.NoErrorf(t, err, "could not start etcd process cluster (%v)", err)
	t.Cleanup(func() {
		require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	})

	cc := epc.Etcdctl()
	err = cc.Put(t.Context(), "foo", "bar", config.PutOptions{})
	require.NoErrorf(t, err, "error on put")

	// verify warning
	e2e.AssertProcessLogs(t, epc.Procs[0], "request stats")
}

// TestExperimentalWarningApplyDuration tests the experimental warning apply duration
// TODO: this test is a duplicate of TestWarningApplyDuration except it uses --experimental-warning-unary-request-duration
// Remove this test after --experimental-warning-unary-request-duration flag is removed.
func TestExperimentalWarningApplyDuration(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(1),
		e2e.WithExperimentalWarningUnaryRequestDuration(time.Microsecond),
	)
	require.NoErrorf(t, err, "could not start etcd process cluster (%v)", err)
	t.Cleanup(func() {
		require.NoErrorf(t, epc.Close(), "error closing etcd processes")
	})

	cc := epc.Etcdctl()
	err = cc.Put(t.Context(), "foo", "bar", config.PutOptions{})
	require.NoErrorf(t, err, "error on put")

	// verify warning
	e2e.AssertProcessLogs(t, epc.Procs[0], "request stats")
}

func TestBothWarningApplyDurationFlagsFail(t *testing.T) {
	e2e.BeforeTest(t)

	_, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(1),
		e2e.WithWarningUnaryRequestDuration(time.Second),
		e2e.WithExperimentalWarningUnaryRequestDuration(time.Second),
	)
	require.Errorf(t, err, "Expected process to fail")
}
