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
	res := make([]CheckPutResult, len(requests))

	if cs.HaveAcl() {
		// Users with ACL cannot create dirs and change prototypes, they can only read/write keys
		for i, r := range requests {
			if auth.PathIsDir(r.Key) {
				res[i].ProtoInfo = txn.GetPrototypeInfo(r.Key, txn.Rev())
			} else {
				// If it's a key it may not exist yet, but its dir should already exist, so
				// get prototype of a dir instead
				p1 := auth.PathGetPrefix(r.Key, 1)
				if p1 != nil {
					res[i].ProtoInfo = txn.GetPrototypeInfo(p1, txn.Rev())
				}
			}
			res[i].CanRead, res[i].CanWrite = cs.CanReadWrite(r.Key, res[i].ProtoInfo.PrototypeIdx, res[i].ProtoInfo.ForceFindDepth)
		}
		return res
	}

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

	for i, r := range sortedRequests {
		prevPi := PrototypeInfo{}

		p1 := auth.PathGetPrefix(r.req.Key, 1)
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

		if (prevProto != nil) || auth.PathIsRoot(r.req.Key) {
			if auth.PathIsDir(r.req.Key) {
				pn := auth.PathGetProtoName(r.req.Value)
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

		res[r.idx].CanRead = true
		res[r.idx].CanWrite = true
	}

	return res
}

func CheckDelete(txn TxnRead, cs *auth.CapturedState, request *pb.DeleteRangeRequest) []CheckDeleteResult {
	// TODO(s.vorobiev) : impl
	return nil
}

func CheckGet(cs *auth.CapturedState, kv *mvccpb.KeyValue) bool {
	// TODO(s.vorobiev) : impl
	return false
}
