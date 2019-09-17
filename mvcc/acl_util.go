package mvcc

import (
	"github.com/coreos/etcd/auth"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type CheckPutResult struct {
	CanWrite  bool
	CanRead   bool
	ProtoInfo PrototypeInfo
}

type CheckDeleteResult struct {
	Key     [][]byte
	MainRev int64
	SubRev  int64
	CanRead bool
}

func CheckPut(txn TxnRead, cs *auth.CapturedState, requests []*pb.PutRequest) []CheckPutResult {
	// TODO(s.vorobiev) : impl
	return nil
}

func CheckDelete(txn TxnRead, cs *auth.CapturedState, request *pb.DeleteRangeRequest) []CheckDeleteResult {
	// TODO(s.vorobiev) : impl
	return nil
}

func CheckGet(txn TxnRead, cs *auth.CapturedState, kv *mvccpb.KeyValue) bool {
	// TODO(s.vorobiev) : impl
	return false
}

func checkWatch(kvindex index, cs *auth.CapturedState, kv *mvccpb.KeyValue) bool {
	// TODO(s.vorobiev) : impl
	return false
}
