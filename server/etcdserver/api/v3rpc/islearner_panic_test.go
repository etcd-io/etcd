package v3rpc_test

import (
    "testing"

    "go.uber.org/zap"

    "go.etcd.io/etcd/client/pkg/v3/types"
    "go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

// The cluster may transiently remove the local member concurrently with
// Status() calls. IsLocalMemberLearner must not panic; it should return false.
func TestIsLearner_ReturnsFalseWhenLocalMemberMissing(t *testing.T) {
    lg := zap.NewNop()
    c := membership.NewCluster(lg)
    // Set a local member ID that is not present in c.members
    c.SetID(types.ID(1), types.ID(100))

    if got := c.IsLocalMemberLearner(); got {
        t.Fatalf("expected false when local member is missing, got %v", got)
    }
}
