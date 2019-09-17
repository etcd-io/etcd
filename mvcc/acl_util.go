package mvcc

import (
	"bytes"
	"sort"

	"github.com/coreos/etcd/auth"
	"github.com/coreos/etcd/auth/authpb"
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

	for i, r := range sortedRequests {
		prevPi := PrototypeInfo{}

		p1 := pathGetPrefix(r.req.Key, 1)
		if p1 != nil {
			idx := sort.Search(i, func(j int) bool { return bytes.Compare(sortedRequests[j].req.Key, p1) >= 0 })
			if idx < i && bytes.Compare(sortedRequests[idx].req.Key, p1) == 0 {
				prevPi = res[sortedRequests[idx].idx].ProtoInfo
			} else {
				prevPi = txn.GetPrototypeInfo(p1, txn.Rev())
			}
		}

		var prevProto *auth.CachedPrototype
		if prevPi.PrototypeIdx != 0 {
			prevProto = cs.GetPrototype(prevPi.PrototypeIdx)
		}

		if (prevProto != nil) || pathIsRoot(r.req.Key) {
			if pathIsDir(r.req.Key) {
				pn := pathGetProtoName(r.req.Value)
				if pn != nil {
					proto := cs.GetPrototypeByName(string(pn))
					if proto != nil {
						res[r.idx].ProtoInfo.PrototypeIdx = proto.Idx
						if (prevProto != nil) && ((prevProto.Orig.Flags & uint32(authpb.FORCE_SUBOBJECTS_FIND)) != 0) {
							res[r.idx].ProtoInfo.ForceFindDepth = prevPi.ForceFindDepth + 1
						}
					}
				}
			} else {
				res[r.idx].ProtoInfo = prevPi
			}
		}
	}

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

func pathIsRoot(key []byte) bool {
	return (len(key) == 1) && key[0] == '/'
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
	} else if len(value) > 0 {
		return value
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
