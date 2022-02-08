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

// When the quorum isn't satisfied, then each etcd member isn't able to
// publish/register server information(i.e., clientURL) into the cluster.
// Accordingly, the v2 proxy can't get any member's clientURL, so this
// case will fail for sure in this case.
//
// todo(ahrtr): When v2 proxy is removed, then we can remove the go build
// lines below.
//go:build !cluster_proxy
// +build !cluster_proxy

package e2e

import (
	"testing"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestSerializableReadWithoutQuorum(t *testing.T) {
	// Initialize a cluster with 3 members
	epc, err := e2e.InitEtcdProcessCluster(t, e2e.NewConfigAutoTLS())
	if err != nil {
		t.Fatalf("Failed to initilize the etcd cluster: %v", err)
	}

	// Remove two members, so that only one etcd will get started
	epc.Procs = epc.Procs[:1]

	// Start the etcd cluster with only one member
	if err := epc.Start(); err != nil {
		t.Fatalf("Failed to start the etcd cluster: %v", err)
	}

	// construct the ctl context
	cx := getDefaultCtlCtx(t)
	cx.epc = epc

	// run serializable test and wait for result
	runCtlTest(t, serializableReadTest, nil, cx)

	// run linearizable test and wait for result
	runCtlTest(t, linearizableReadTest, nil, cx)
}

func serializableReadTest(cx ctlCtx) {
	cx.quorum = false
	if err := ctlV3Get(cx, []string{"key1"}, []kv{}...); err != nil {
		cx.t.Errorf("serializableReadTest failed: %v", err)
	}
}

func linearizableReadTest(cx ctlCtx) {
	cx.quorum = true
	if err := ctlV3Get(cx, []string{"key1"}, []kv{}...); err == nil {
		cx.t.Error("linearizableReadTest is expected to fail, but it succeeded")
	}
}
