package mvcc

import (
	"bytes"
	"sort"

	"github.com/coreos/etcd/auth"
	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type CheckPutResult struct {
	CanWrite  bool
	CanRead   bool
	ProtoInfo PrototypeInfo
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
			res[i].CanRead, res[i].CanWrite = cs.CanReadWrite(r.Key,
				res[i].ProtoInfo.PrototypeIdx, res[i].ProtoInfo.ForceFindDepth)
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
	// First, sort all requests by path
	sort.Slice(sortedRequests, func(i, j int) bool {
		return bytes.Compare(sortedRequests[i].req.Key, sortedRequests[j].req.Key) < 0
	})

	for i, r := range sortedRequests {
		prevPi := PrototypeInfo{}

		// Get parent dir
		p1 := auth.PathGetPrefix(r.req.Key, 1)
		if p1 != nil {
			idx := sort.Search(i, func(j int) bool { return bytes.Compare(sortedRequests[j].req.Key, p1) >= 0 })
			if idx < i && bytes.Compare(sortedRequests[idx].req.Key, p1) == 0 {
				// Parent dir was added in this transaction, get its proto info from processed request
				prevPi = res[sortedRequests[idx].idx].ProtoInfo
			} else {
				// Parent dir is in store, get proto info from store
				prevPi = txn.GetPrototypeInfo(p1, txn.Rev())
			}
		}

		var prevProto *auth.CachedPrototype
		if prevPi.PrototypeIdx != 0 {
			prevProto = cs.GetPrototype(prevPi.PrototypeIdx)
		}

		if (prevProto != nil) || auth.PathIsRoot(r.req.Key) {
			// Should only set proto info for this path if proto info for prev path exists
			// or if it's a root path, i.e. simply "/"
			if auth.PathIsDir(r.req.Key) {
				pn := auth.PathGetProtoName(r.req.Value)
				if pn != nil {
					proto := cs.GetPrototypeByName(string(pn))
					if proto != nil {
						res[r.idx].ProtoInfo.PrototypeIdx = proto.Idx
						if (prevProto != nil) && ((prevProto.Orig.Flags & uint32(authpb.FORCE_SUBOBJECTS_FIND)) != 0) {
							// Parent dir has FORCE_SUBOBJECTS_FIND on, so get ForceFindDepth from prev dir
							// and assign this dir's ForceFindDepth as prev dir's ForceFindDepth + 1
							// ForceFindDepth specifies how many consecutive FORCE_SUBOBJECTS_FIND parents are
							// on the path, this allows us to check for dir visibility very quickly on acl check time.
							res[r.idx].ProtoInfo.ForceFindDepth = prevPi.ForceFindDepth + 1
						}
					}
				}
			} else {
				// For keys proto info is just parent dir's proto info
				res[r.idx].ProtoInfo = prevPi
			}
		}

		// User without acl can always read/write everything (well, after etcd perm checks)
		res[r.idx].CanRead = true
		res[r.idx].CanWrite = true
	}

	return res
}

func CheckDelete(cs *auth.CapturedState, keys [][]byte, revs []revision, pi []PrototypeInfo) ([][]byte, []revision, []PrototypeInfo, []bool) {
	fKeys := make([][]byte, 0, len(keys))
	fRevs := make([]revision, 0, len(revs))
	fPi := make([]PrototypeInfo, 0, len(pi))
	fCanRead := make([]bool, 0, len(keys))

	for i, key := range keys {
		cr, cw := cs.CanReadWrite(key, pi[i].PrototypeIdx, pi[i].ForceFindDepth)
		if cw {
			fKeys = append(fKeys, key)
			fRevs = append(fRevs, revs[i])
			fPi = append(fPi, pi[i])
			fCanRead = append(fCanRead, cr)
		}
	}

	return fKeys, fRevs, fPi, fCanRead
}

func CheckRange(txn TxnRead, cs *auth.CapturedState, rer *RangeExResult) *RangeResult {
	rr := &RangeResult{KVs: nil, Count: rer.Count, Rev: rer.Rev}
	if rer.Limit > 0 {
		rr.KVs = make([]mvccpb.KeyValue, 0, rer.Limit)
		revBytes := newRevBytes()
		for _, rev := range rer.Revs {
			kv := mvccpb.KeyValue{}
			revToBytes(rev, revBytes)
			txn.RangeExReadKV(revBytes, &kv)
			cr, _ := cs.CanReadWrite(kv.Key, kv.PrototypeIdx, kv.ForceFindDepth)
			if cr {
				rr.KVs = append(rr.KVs, kv)
				if len(rr.KVs) >= rer.Limit {
					break
				}
			}
		}
		if len(rr.KVs) <= 0 {
			// It's hard to say what to do here, in general, .Count should be
			// equal to total number of keys visible, but in order to check that
			// we'll have to read all the keys and check acl against all keys, that's
			// an overkill, so we just return .Count as is. On the other hand .KVs
			// should have as much as .Limit kvs, but what if all acl checks failed,
			// we can't set .Count to 0 here as well, that'll be inconsistent with the
			// rest of the logic... So just set KVs no nil and pretend no keys are there,
			// but keep the original .Count. m.b. we'll have to fix this some time later...
			rr.KVs = nil
		}
	}
	return rr
}

func CheckWatch(cs *auth.CapturedState, kv *mvccpb.KeyValue) bool {
	cr, _ := cs.CanReadWrite(kv.Key, kv.PrototypeIdx, kv.ForceFindDepth)
	return cr
}

func CheckLease(cs *auth.CapturedState, li *lease.LeaseItem) bool {
	_, cw := cs.CanReadWrite([]byte(li.Key), li.PrototypeIdx, li.ForceFindDepth)
	return cw
}
