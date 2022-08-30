// Copyright 2021 The etcd Authors
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

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestInitDaemonNotifyWithoutQuorum(t *testing.T) {
	// Initialize a cluster with 3 members
	epc, err := e2e.InitEtcdProcessCluster(t, e2e.NewConfigAutoTLS())
	if err != nil {
		t.Fatalf("Failed to initilize the etcd cluster: %v", err)
	}

	// Remove two members, so that only one etcd will get started
	epc.Procs = epc.Procs[:1]

	// Start the etcd cluster with only one member
	if err := epc.Start(context.TODO()); err != nil {
		t.Fatalf("Failed to start the etcd cluster: %v", err)
	}

	// Expect log message indicating time out waiting for quorum hit
	e2e.AssertProcessLogs(t, epc.Procs[0], "startEtcd: timed out waiting for the ready notification")
	// Expect log message indicating systemd notify message has been sent
	e2e.AssertProcessLogs(t, epc.Procs[0], "notifying init daemon")

	epc.Close()
}
