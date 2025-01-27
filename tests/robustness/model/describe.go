// Copyright 2023 The etcd Authors
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

package model

import (
	"fmt"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func describeEtcdResponse(request EtcdRequest, response MaybeEtcdResponse) string {
	if response.Error != "" {
		return fmt.Sprintf("err: %q", response.Error)
	}
	if response.ClientError != "" {
		return fmt.Sprintf("err: %q", response.ClientError)
	}
	if response.Persisted {
		if response.PersistedRevision != 0 {
			return fmt.Sprintf("unknown, rev: %d", response.PersistedRevision)
		}
		return "unknown"
	}
	switch request.Type {
	case Range:
		return fmt.Sprintf("%s, rev: %d", describeRangeResponse(request.Range.RangeOptions, *response.Range), response.Revision)
	case Txn:
		return fmt.Sprintf("%s, rev: %d", describeTxnResponse(request.Txn, response.Txn), response.Revision)
	case LeaseGrant, LeaseRevoke, Defragment:
		if response.Revision == 0 {
			return "ok"
		}
		return fmt.Sprintf("ok, rev: %d", response.Revision)
	case Compact:
		return "ok"
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
}

func describeEtcdRequest(request EtcdRequest) string {
	switch request.Type {
	case Range:
		return describeRangeRequest(request.Range.RangeOptions, request.Range.Revision)
	case Txn:
		guaranteedTxnDescription := describeGuaranteedTxn(request.Txn)
		if guaranteedTxnDescription != "" {
			return guaranteedTxnDescription
		}
		onSuccess := describeEtcdOperations(request.Txn.OperationsOnSuccess)
		if len(request.Txn.Conditions) != 0 {
			if len(request.Txn.OperationsOnFailure) == 0 {
				return fmt.Sprintf("if(%s).then(%s)", describeEtcdConditions(request.Txn.Conditions), onSuccess)
			}
			onFailure := describeEtcdOperations(request.Txn.OperationsOnFailure)
			return fmt.Sprintf("if(%s).then(%s).else(%s)", describeEtcdConditions(request.Txn.Conditions), onSuccess, onFailure)
		}
		return onSuccess
	case LeaseGrant:
		return fmt.Sprintf("leaseGrant(%d)", request.LeaseGrant.LeaseID)
	case LeaseRevoke:
		return fmt.Sprintf("leaseRevoke(%d)", request.LeaseRevoke.LeaseID)
	case Defragment:
		return "defragment()"
	case Compact:
		return fmt.Sprintf("compact(%d)", request.Compact.Revision)
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
}

func describeGuaranteedTxn(txn *TxnRequest) string {
	if len(txn.Conditions) != 1 || len(txn.OperationsOnSuccess) != 1 || len(txn.OperationsOnFailure) > 1 {
		return ""
	}
	switch txn.OperationsOnSuccess[0].Type {
	case PutOperation:
		if txn.Conditions[0].Key != txn.OperationsOnSuccess[0].Put.Key || (len(txn.OperationsOnFailure) == 1 && txn.Conditions[0].Key != txn.OperationsOnFailure[0].Range.Start) {
			return ""
		}
		if txn.Conditions[0].ExpectedVersion > 0 {
			return ""
		}
		if txn.Conditions[0].ExpectedRevision == 0 {
			return fmt.Sprintf("guaranteedCreate(%q, %s)", txn.Conditions[0].Key, describeValueOrHash(txn.OperationsOnSuccess[0].Put.Value))
		}
		return fmt.Sprintf("guaranteedUpdate(%q, %s, mod_rev=%d)", txn.Conditions[0].Key, describeValueOrHash(txn.OperationsOnSuccess[0].Put.Value), txn.Conditions[0].ExpectedRevision)
	case DeleteOperation:
		if txn.Conditions[0].Key != txn.OperationsOnSuccess[0].Delete.Key || (len(txn.OperationsOnFailure) == 1 && txn.Conditions[0].Key != txn.OperationsOnFailure[0].Range.Start) {
			return ""
		}
		return fmt.Sprintf("guaranteedDelete(%q, mod_rev=%d)", txn.Conditions[0].Key, txn.Conditions[0].ExpectedRevision)
	}
	return ""
}

func describeEtcdConditions(conds []EtcdCondition) string {
	opsDescription := make([]string, len(conds))
	for i, cond := range conds {
		if cond.ExpectedVersion > 0 {
			opsDescription[i] = fmt.Sprintf("ver(%s)==%d", cond.Key, cond.ExpectedVersion)
		} else {
			opsDescription[i] = fmt.Sprintf("mod_rev(%s)==%d", cond.Key, cond.ExpectedRevision)
		}
	}
	return strings.Join(opsDescription, " && ")
}

func describeEtcdOperations(ops []EtcdOperation) string {
	opsDescription := make([]string, len(ops))
	for i := range ops {
		opsDescription[i] = describeEtcdOperation(ops[i])
	}
	return strings.Join(opsDescription, ", ")
}

func describeTxnResponse(request *TxnRequest, response *TxnResponse) string {
	respDescription := make([]string, len(response.Results))
	for i, result := range response.Results {
		if response.Failure {
			respDescription[i] = describeEtcdOperationResponse(request.OperationsOnFailure[i], result)
		} else {
			respDescription[i] = describeEtcdOperationResponse(request.OperationsOnSuccess[i], result)
		}
	}
	description := strings.Join(respDescription, ", ")
	if len(request.Conditions) == 0 {
		return description
	}
	if response.Failure {
		return fmt.Sprintf("failure(%s)", description)
	}
	return fmt.Sprintf("success(%s)", description)
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case RangeOperation:
		return describeRangeRequest(op.Range, 0)
	case PutOperation:
		if op.Put.LeaseID != 0 {
			return fmt.Sprintf("put(%q, %s, %d)", op.Put.Key, describeValueOrHash(op.Put.Value), op.Put.LeaseID)
		}
		return fmt.Sprintf("put(%q, %s)", op.Put.Key, describeValueOrHash(op.Put.Value))
	case DeleteOperation:
		return fmt.Sprintf("delete(%q)", op.Delete.Key)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeRangeRequest(opts RangeOptions, revision int64) string {
	kwargs := []string{}
	if revision != 0 {
		kwargs = append(kwargs, fmt.Sprintf("rev=%d", revision))
	}
	if opts.Limit != 0 {
		kwargs = append(kwargs, fmt.Sprintf("limit=%d", opts.Limit))
	}
	kwargsString := strings.Join(kwargs, ", ")
	if kwargsString != "" {
		kwargsString = ", " + kwargsString
	}
	switch {
	case opts.End == "":
		return fmt.Sprintf("get(%q%s)", opts.Start, kwargsString)
	case opts.End == clientv3.GetPrefixRangeEnd(opts.Start):
		return fmt.Sprintf("list(%q%s)", opts.Start, kwargsString)
	default:
		return fmt.Sprintf("range(%q..%q%s)", opts.Start, opts.End, kwargsString)
	}
}

func describeEtcdOperationResponse(op EtcdOperation, resp EtcdOperationResult) string {
	switch op.Type {
	case RangeOperation:
		return describeRangeResponse(op.Range, resp.RangeResponse)
	case PutOperation:
		return "ok"
	case DeleteOperation:
		return fmt.Sprintf("deleted: %d", resp.Deleted)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeRangeResponse(request RangeOptions, response RangeResponse) string {
	if request.End != "" {
		kvs := make([]string, len(response.KVs))
		for i, kv := range response.KVs {
			kvs[i] = describeValueOrHash(kv.Value)
		}
		return fmt.Sprintf("[%s], count: %d", strings.Join(kvs, ","), response.Count)
	}

	if len(response.KVs) == 0 {
		return "nil"
	}
	return describeValueOrHash(response.KVs[0].Value)
}

func describeValueOrHash(value ValueOrHash) string {
	if value.Hash != 0 {
		return fmt.Sprintf("hash: %d", value.Hash)
	}
	if value.Value == "" {
		return "nil"
	}
	return fmt.Sprintf("%q", value.Value)
}
