package v3rpc_test

import (
    "testing"

    "go.uber.org/zap"

    "go.etcd.io/etcd/client/pkg/v3/types"
    "go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

// This test intentionally panics to capture the failing CI stack trace for
// RaftCluster.IsLocalMemberLearner when the local member is missing.
// Do NOT wrap with recover; we want CI to fail with exit code 2.
func TestIsLearner_PanicsWhenLocalMemberMissing(t *testing.T) {
    lg := zap.NewNop()
    c := membership.NewCluster(lg)
    // Set a local member ID that is not present in c.members
    c.SetID(types.ID(1), types.ID(100))

    // This should panic inside IsLocalMemberLearner with
    // "failed to find local ID in cluster members".
    _ = c.IsLocalMemberLearner()
}

// trigger
