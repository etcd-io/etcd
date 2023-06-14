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
	default:
		return fmt.Sprintf("<! unknown request type: %q !>", request.Type)
	}
}

func describeEtcdRequest(request EtcdRequest) string {
	switch request.Type {
	case Range:
		return describeRangeRequest(request.Range.Key, request.Range.RangeOptions)
	case Txn:
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
	} else {
		return fmt.Sprintf("success(%s)", description)
	}
}

func describeEtcdOperation(op EtcdOperation) string {
	switch op.Type {
	case RangeOperation:
		return describeRangeRequest(op.Key, op.RangeOptions)
	case PutOperation:
		if op.LeaseID != 0 {
			return fmt.Sprintf("put(%q, %s, %d)", op.Key, describeValueOrHash(op.Value), op.LeaseID)
		}
		return fmt.Sprintf("put(%q, %s)", op.Key, describeValueOrHash(op.Value))
	case DeleteOperation:
		return fmt.Sprintf("delete(%q)", op.Key)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", op.Type)
	}
}

func describeRangeRequest(key string, opts RangeOptions) string {
	kwargs := []string{}
	if opts.Limit != 0 {
		kwargs = append(kwargs, fmt.Sprintf("limit=%d", opts.Limit))
	}
	command := "get"
	if opts.WithPrefix {
		command = "range"
	}
	if len(kwargs) == 0 {
		return fmt.Sprintf("%s(%q)", command, key)
	}
	return fmt.Sprintf("%s(%q, %s)", command, key, strings.Join(kwargs, ", "))
}

func describeEtcdOperationResponse(req EtcdOperation, resp EtcdOperationResult) string {
	switch req.Type {
	case RangeOperation:
		return describeRangeResponse(req.RangeOptions, resp.RangeResponse)
	case PutOperation:
		return fmt.Sprintf("ok")
	case DeleteOperation:
		return fmt.Sprintf("deleted: %d", resp.Deleted)
	default:
		return fmt.Sprintf("<! unknown op: %q !>", req.Type)
	}
}

func describeRangeResponse(opts RangeOptions, response RangeResponse) string {
	if opts.WithPrefix {
		kvs := make([]string, len(response.KVs))
		for i, kv := range response.KVs {
			kvs[i] = describeValueOrHash(kv.Value)
		}
		return fmt.Sprintf("[%s], count: %d", strings.Join(kvs, ","), response.Count)
	} else {
		if len(response.KVs) == 0 {
			return "nil"
		} else {
			return describeValueOrHash(response.KVs[0].Value)
		}
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
