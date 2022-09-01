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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestWarningApplyDuration(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t, &e2e.EtcdProcessClusterConfig{
		ClusterSize: 1,
		//Set very small duration to trigger warning
		WarningUnaryRequestDuration: time.Microsecond,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc, err := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())
	require.NoError(t, err)
	err = cc.Put(context.TODO(), "foo", "bar", config.PutOptions{})
	assert.NoError(t, err, "error on put")

	// verify warning
	e2e.AssertProcessLogs(t, epc.Procs[0], "request stats")
}

// TODO: this test is a duplicate of TestWarningApplyDuration except it uses --experimental-warning-unary-request-duration
// Remove this test after --experimental-warning-unary-request-duration flag is removed.
func TestExperimentalWarningApplyDuration(t *testing.T) {
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t, &e2e.EtcdProcessClusterConfig{
		ClusterSize: 1,
		//Set very small duration to trigger warning
		ExperimentalWarningUnaryRequestDuration: time.Microsecond,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	cc, err := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())
	require.NoError(t, err)
	err = cc.Put(context.TODO(), "foo", "bar", config.PutOptions{})
	assert.NoError(t, err, "error on put")

	// verify warning
	e2e.AssertProcessLogs(t, epc.Procs[0], "request stats")
}

func TestBothWarningApplyDurationFlagsFail(t *testing.T) {
	e2e.BeforeTest(t)

	_, err := e2e.NewEtcdProcessCluster(context.TODO(), t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:                             1,
		WarningUnaryRequestDuration:             time.Second,
		ExperimentalWarningUnaryRequestDuration: time.Second,
	})
	if err == nil {
		t.Fatal("Expected process to fail")
	}
}
