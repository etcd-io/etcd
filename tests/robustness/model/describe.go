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
)

func describeEtcdNonDeterministicResponse(request EtcdRequest, response EtcdNonDeterministicResponse) string {
	if response.Err != nil {
		return fmt.Sprintf("err: %q", response.Err)
	}
	if response.ResultUnknown {
		return fmt.Sprintf("unknown, rev: %d", response.Revision)
	}
	return describeEtcdResponse(request, response.EtcdResponse)
}

func describeEtcdResponse(request EtcdRequest, response EtcdResponse) string {
	if request.Type == Txn {
		return fmt.Sprintf("%s, rev: %d", describeTxnResponse(request.Txn, response.Txn), response.Revision)
	}
	if response.Revision == 0 {
		return "ok"
	}
	return fmt.Sprintf("ok, rev: %d", response.Revision)
}

func describeEtcdRequest(request EtcdRequest) string {
	switch request.Type {
	case Txn:
		describeOperations := describeEtcdOperations(request.Txn.OperationsOnSuccess)
		if len(request.Txn.Conditions) != 0 {
			return fmt.Sprintf("if(%s).then(%s)", describeEtcdConditions(request.Txn.Conditions), describeOperations)
		}
		return describeOperations
	case LeaseGrant:
		return fmt.Sprintf("leaseGrant(%d)", request.LeaseGrant.LeaseID)
	case LeaseRevoke:
		return fmt.Sprintf("leaseRevoke(%d)", request.LeaseRevoke.LeaseID)
	case Defragment:
		return fmt.Sprintf("defragment()")
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
}

func describeEtcdConditions(conds []EtcdCondition) string {
	opsDescription := make([]string, len(conds))
	for i := range conds {
		opsDescription[i] = fmt.Sprintf("mod_rev(%s)==%d", conds[i].Key, conds[i].ExpectedRevision)
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
	if response.Failure {
		return fmt.Sprintf("txn failed")
	}
	respDescription := make([]string, len(response.Results))
	for i := range response.Results {
		respDescription[i] = describeEtcdOperationResponse(request.OperationsOnSuccess[i], response.Results[i])
	}
	return strings.Join(respDescription, ", ")
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case Range:
		if op.WithPrefix {
			if op.Limit != 0 {
				return fmt.Sprintf("range(%q, limit=%d)", op.Key, op.Limit)
			}
			return fmt.Sprintf("range(%q)", op.Key)
		}
		return fmt.Sprintf("get(%q)", op.Key)
	case Put:
		if op.LeaseID != 0 {
			return fmt.Sprintf("put(%q, %s, %d)", op.Key, describeValueOrHash(op.Value), op.LeaseID)
		}
		return fmt.Sprintf("put(%q, %s)", op.Key, describeValueOrHash(op.Value))
	case Delete:
		return fmt.Sprintf("delete(%q)", op.Key)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeEtcdOperationResponse(req EtcdOperation, resp EtcdOperationResult) string {
	switch req.Type {
	case Range:
		if req.WithPrefix {
			kvs := make([]string, len(resp.KVs))
			for i, kv := range resp.KVs {
				kvs[i] = describeValueOrHash(kv.Value)
			}
			return fmt.Sprintf("[%s], count: %d", strings.Join(kvs, ","), resp.Count)
		} else {
			if len(resp.KVs) == 0 {
				return "nil"
			} else {
				return describeValueOrHash(resp.KVs[0].Value)
			}
		}
	case Put:
		return fmt.Sprintf("ok")
	case Delete:
		return fmt.Sprintf("deleted: %d", resp.Deleted)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", req.Type)
	}
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
