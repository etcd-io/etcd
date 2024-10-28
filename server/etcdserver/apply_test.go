package etcdserver

import (
	"context"
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/mvcc"
	betesting "go.etcd.io/etcd/server/v3/mvcc/backend/testing"
)

func TestReadonlyTxnError(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.New(zap.NewExample(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup minimal server to get access to applier
	srv := &EtcdServer{lgMu: new(sync.RWMutex), lg: zap.NewExample(), r: *newRaftNode(raftNodeConfig{lg: zap.NewExample(), Node: newNodeRecorder()})}
	srv.kv = s
	srv.be = b

	a := srv.newApplierV3Backend()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// put some data to prevent early termination in rangeKeys
	// we are expecting failure on cancelled context check
	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	_, _, err := a.Txn(ctx, txn)
	if err == nil || !strings.Contains(err.Error(), "applyTxn: failed Range: rangeKeys: context cancelled: context canceled") {
		t.Fatalf("Expected context canceled error, got %v", err)
	}
}

func TestWriteTxnPanic(t *testing.T) {
	b, bePath := betesting.NewDefaultTmpBackend(t)
	s := mvcc.New(zap.NewExample(), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup minimal server to get access to applier
	srv := &EtcdServer{lgMu: new(sync.RWMutex), lg: zap.NewExample(), r: *newRaftNode(raftNodeConfig{lg: zap.NewExample(), Node: newNodeRecorder()})}
	srv.kv = s
	srv.be = b

	a := srv.newApplierV3Backend()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// write txn that puts some data and then fails in range due to cancelled context
	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
				},
			},
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	// compute DB file hash before applying the txn
	dbHashBefore, err := computeFileHash(bePath)
	require.NoErrorf(t, err, "failed to compute DB file hash before txn")

	// we verify the following properties below:
	// 1. server panics after a write txn aply fails (invariant: server should never try to move on from a failed write)
	// 2. no writes from the txn are applied to the backend (invariant: failed write should have no side-effect on DB state besides panic)
	assert.Panics(t, func() { a.Txn(ctx, txn) }, "Expected panic in Txn with writes")
	dbHashAfter, err := computeFileHash(bePath)
	require.NoErrorf(t, err, "failed to compute DB file hash after txn")
	require.Equalf(t, dbHashBefore, dbHashAfter, "mismatch in DB hash before and after failed write txn")
}

func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return string(h.Sum(nil)), nil
}
