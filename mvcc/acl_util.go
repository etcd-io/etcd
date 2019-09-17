package mvcc

import (
	"bytes"
	"sort"

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
	Key     []byte
	MainRev int64
	SubRev  int64
	CanRead bool
}

func CheckPut(txn TxnRead, cs *auth.CapturedState, requests []*pb.PutRequest) []CheckPutResult {
	sortedRequests := make([]struct {
		idx int
		req *pb.PutRequest
	}, len(requests))
	for i, req := range requests {
		sortedRequests[i].idx = i
		sortedRequests[i].req = req
	}
	sort.Slice(sortedRequests, func(i, j int) bool {
		return bytes.Compare(sortedRequests[i].req.Key, sortedRequests[j].req.Key) < 0
	})

	res := make([]CheckPutResult, len(requests))

	/*for i, r := range sortedRequests {
		if pathIsDir(r.req.Key) {
		} else {
		}
	}*/

	return res
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

func pathIsDir(key []byte) bool {
	return (len(key) > 0) && key[len(key)-1] == '/'
}

func pathGetProtoName(value []byte) []byte {
	pos := bytes.IndexByte(value, ',')
	if pos >= 0 {
		value = value[pos+1:]
		if (len(value) > 0) && (value[0] != '/') {
			return value
		}
	}
	return nil
}

func pathGetPrefix(key []byte, N int) []byte {
	for i := 0; i < N; i++ {
		if len(key) < 1 {
			return nil
		}
		pos := bytes.LastIndexByte(key[:len(key)-1], '/')
		if pos < 0 {
			return nil
		}
		key = key[:pos+1]
	}
	return key
}
