// Copyright 2026 The etcd Authors
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

package correctness

import (
	"fmt"
	"html"
	"maps"
	"slices"
	"strings"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func describeOperation(req StorageRequest, res StorageResponse) string {
	return fmt.Sprintf("%s -> %s", describeRequest(req), describeResponse(req, res))
}

func describeRequest(req StorageRequest) string {
	switch req.Op {
	case OpPut:
		if req.Lease != 0 {
			return fmt.Sprintf("Put(%q, %q, lease=%d)", string(req.Key), string(req.Value), req.Lease)
		}
		return fmt.Sprintf("Put(%q, %q)", string(req.Key), string(req.Value))
	case OpDelete:
		return fmt.Sprintf("Delete(%q)", string(req.Key))
	case OpDeleteRange:
		return fmt.Sprintf("DeleteRange(%q, %q)", string(req.Key), string(req.End))
	case OpTxn:
		return fmt.Sprintf("Txn([%s])", describeTxnOps(req.TxnOps))
	case OpLeaseRevoke:
		return fmt.Sprintf("LeaseRevoke(%d)", req.Lease)
	case OpCompact:
		return fmt.Sprintf("Compact(%d)", req.Rev)
	case OpFirstRev:
		return "FirstRev()"
	case OpRev:
		return "Rev()"
	case OpCommit:
		return "Commit()"
	case OpGet:
		return fmt.Sprintf("Get(%q)%s", string(req.Key), describeRevSuffix(req.Rev))
	case OpRange, OpReadTxn:
		return describeRangeRequest(req)
	case OpHash:
		return "Hash()"
	case OpHashByRev:
		return fmt.Sprintf("HashByRev(%d)", req.Rev)
	case OpBackendPut:
		return fmt.Sprintf("BackendPut(%s, %q, %q)", req.Bucket, string(req.Key), string(req.Value))
	case OpBackendDelete:
		return fmt.Sprintf("BackendDelete(%s, %q)", req.Bucket, string(req.Key))
	case OpBackendRange:
		return fmt.Sprintf("BackendRange(%s, %q)", req.Bucket, string(req.Key))
	case OpBackendForEach:
		return fmt.Sprintf("BackendForEach(%s)", req.Bucket)
	case OpBackendHash:
		return "BackendHash()"
	case OpCrossLayerTxn:
		return fmt.Sprintf("CrossLayerTxn(MVCC.Put(%q,%q), Meta.Put(%q,%q))",
			string(req.Key), string(req.Value), string(req.MetaKey), string(req.MetaVal))
	case OpDefrag:
		return "Defrag()"
	default:
		return "UnknownOp"
	}
}

func describeRangeRequest(req StorageRequest) string {
	prefix := "Range"
	if req.Op == OpReadTxn {
		prefix = "ReadTxnRange"
	}
	if req.CountOnly {
		return fmt.Sprintf("%sCount(%q..%q)%s", prefix, string(req.Key), string(req.End), describeRevSuffix(req.Rev))
	}
	var opts []string
	if req.Limit > 0 {
		opts = append(opts, fmt.Sprintf("limit=%d", req.Limit))
	}
	if req.FastKeysOnly {
		opts = append(opts, "fastKeys")
	}
	if req.WithTotalCount {
		opts = append(opts, "totalCount")
	}
	optSuffix := ""
	if len(opts) > 0 {
		optSuffix = fmt.Sprintf(" [%s]", strings.Join(opts, ","))
	}
	return fmt.Sprintf("%s(%q..%q)%s%s", prefix, string(req.Key), string(req.End), optSuffix, describeRevSuffix(req.Rev))
}

func describeRevSuffix(rev int64) string {
	if rev != 0 {
		return fmt.Sprintf(" [rev=%d]", rev)
	}
	return ""
}

func describeTxnOps(ops []TxnSubOp) string {
	subDescs := make([]string, 0, len(ops))
	for _, sub := range ops {
		switch sub.Type {
		case TxnSubOpPut:
			if sub.Lease != 0 {
				subDescs = append(subDescs, fmt.Sprintf("Put(%q,%q,L%d)", string(sub.Key), string(sub.Value), sub.Lease))
			} else {
				subDescs = append(subDescs, fmt.Sprintf("Put(%q,%q)", string(sub.Key), string(sub.Value)))
			}
		case TxnSubOpDeleteRange:
			subDescs = append(subDescs, fmt.Sprintf("Del(%q..%q)", string(sub.Key), string(sub.End)))
		}
	}
	return strings.Join(subDescs, ", ")
}

func describeResponse(req StorageRequest, res StorageResponse) string {
	if res.Err != nil {
		return fmt.Sprintf("err:%v (rev=%d)", res.Err, res.Rev)
	}
	switch req.Op {
	case OpPut, OpCrossLayerTxn:
		return fmt.Sprintf("rev:%d", res.Rev)
	case OpDelete, OpDeleteRange, OpTxn, OpLeaseRevoke:
		return fmt.Sprintf("rev:%d,del:%d", res.Rev, res.Deleted)
	case OpCompact, OpDefrag, OpCommit, OpBackendPut, OpBackendDelete:
		return "ok"
	case OpFirstRev:
		return fmt.Sprintf("firstRev:%d", res.FirstRev)
	case OpRev:
		return fmt.Sprintf("rev:%d", res.Rev)
	case OpGet:
		if len(res.KVs) == 0 {
			return fmt.Sprintf("<nil> (rev=%d)", res.Rev)
		}
		kv := res.KVs[0]
		return fmt.Sprintf("%q (c=%d,m=%d,v=%d,L=%d,rev=%d)",
			string(kv.Value), kv.CreateRevision, kv.ModRevision, kv.Version, kv.Lease, res.Rev)
	case OpRange, OpReadTxn:
		if req.CountOnly {
			return fmt.Sprintf("count:%d (rev=%d)", res.Count, res.Rev)
		}
		return fmt.Sprintf("[%s] (count=%d,rev=%d)", describeKVPairs(res.KVs), res.Count, res.Rev)
	case OpHash:
		return fmt.Sprintf("hash:%d (rev=%d)", res.Hash, res.Rev)
	case OpHashByRev:
		return fmt.Sprintf("hash:%d (hashRev=%d,compactRev=%d,rev=%d)", res.Hash, res.HashRev, res.CompactRev, res.Rev)
	case OpBackendRange, OpBackendForEach:
		return fmt.Sprintf("[%s]", describeBackendPairs(res.BackendKeys, res.BackendVals))
	case OpBackendHash:
		return fmt.Sprintf("hash:%d", res.Hash)
	default:
		return "unknown"
	}
}

func describeKVPairs(kvs []*mvccpb.KeyValue) string {
	pairs := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		pairs = append(pairs, fmt.Sprintf("%q:%q(c=%d,m=%d,v=%d,L=%d)",
			string(kv.Key), string(kv.Value), kv.CreateRevision, kv.ModRevision, kv.Version, kv.Lease))
	}
	return strings.Join(pairs, ", ")
}

func describeBackendPairs(keys, vals [][]byte) string {
	pairs := make([]string, 0, len(keys))
	for i := range keys {
		pairs = append(pairs, fmt.Sprintf("%q:%q", string(keys[i]), string(vals[i])))
	}
	return strings.Join(pairs, ", ")
}

func describeState(state *StorageState) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<p style=\"margin: 0.25em 0;\">rev: %d, compactRev: %d</p>", state.currentRev, state.compactRev)

	keys := slices.Sorted(maps.Keys(state.items))
	if len(keys) > 0 {
		sb.WriteString("<ul style=\"margin: 0.25em 0;\">")
		for _, k := range keys {
			kv := state.items[k]
			fmt.Fprintf(&sb, "<li><b>%s</b>: %s (c=%d, m=%d, v=%d, L=%d)</li>",
				html.EscapeString(k), html.EscapeString(string(kv.Value)),
				kv.CreateRevision, kv.ModRevision, kv.Version, kv.Lease)
		}
		sb.WriteString("</ul>")
	} else {
		sb.WriteString("<p style=\"margin: 0.25em 0;\"><i>empty</i></p>")
	}

	buckets := slices.Sorted(maps.Keys(state.backendBuckets))
	for _, b := range buckets {
		bkeys := slices.Sorted(maps.Keys(state.backendBuckets[b]))
		if len(bkeys) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "<p style=\"margin: 0.25em 0;\">bucket %s:</p><ul style=\"margin: 0.25em 0;\">", html.EscapeString(b))
		for _, k := range bkeys {
			fmt.Fprintf(&sb, "<li><b>%s</b>: %s</li>",
				html.EscapeString(k), html.EscapeString(string(state.backendBuckets[b][k])))
		}
		sb.WriteString("</ul>")
	}

	return sb.String()
}
