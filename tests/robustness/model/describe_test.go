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
	"errors"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"

	"github.com/stretchr/testify/assert"
)

func TestModelDescribe(t *testing.T) {
	tcs := []struct {
		req            EtcdRequest
		resp           MaybeEtcdResponse
		expectDescribe string
	}{
		{
			req:            getRequest("key1"),
			resp:           emptyGetResponse(1),
			expectDescribe: `get("key1") -> nil, rev: 1`,
		},
		{
			req:            getRequest("key2"),
			resp:           getResponse("key", "2", 2, 2),
			expectDescribe: `get("key2") -> "2", rev: 2`,
		},
		{
			req:            getRequest("key2b"),
			resp:           getResponse("key2b", "01234567890123456789", 2, 2),
			expectDescribe: `get("key2b") -> hash: 2945867837, rev: 2`,
		},
		{
			req:            putRequest("key3", "3"),
			resp:           putResponse(3),
			expectDescribe: `put("key3", "3") -> ok, rev: 3`,
		},
		{
			req:            putWithLeaseRequest("key3b", "3b", 3),
			resp:           putResponse(3),
			expectDescribe: `put("key3b", "3b", 3) -> ok, rev: 3`,
		},
		{
			req:            putRequest("key3c", "01234567890123456789"),
			resp:           putResponse(3),
			expectDescribe: `put("key3c", hash: 2945867837) -> ok, rev: 3`,
		},
		{
			req:            putRequest("key4", "4"),
			resp:           failedResponse(errors.New("failed")),
			expectDescribe: `put("key4", "4") -> err: "failed"`,
		},
		{
			req:            putRequest("key4b", "4b"),
			resp:           partialResponse(42),
			expectDescribe: `put("key4b", "4b") -> unknown, rev: 42`,
		},
		{
			req:            deleteRequest("key5"),
			resp:           deleteResponse(1, 5),
			expectDescribe: `delete("key5") -> deleted: 1, rev: 5`,
		},
		{
			req:            deleteRequest("key6"),
			resp:           failedResponse(errors.New("failed")),
			expectDescribe: `delete("key6") -> err: "failed"`,
		},
		{
			req:            compareRevisionAndPutRequest("key7", 7, "77"),
			resp:           txnEmptyResponse(false, 7),
			expectDescribe: `if(mod_rev(key7)==7).then(put("key7", "77")) -> failure(), rev: 7`,
		},
		{
			req:            compareRevisionAndPutRequest("key8", 8, "88"),
			resp:           txnPutResponse(true, 8),
			expectDescribe: `if(mod_rev(key8)==8).then(put("key8", "88")) -> success(ok), rev: 8`,
		},
		{
			req:            compareRevisionAndPutRequest("key9", 9, "99"),
			resp:           failedResponse(errors.New("failed")),
			expectDescribe: `if(mod_rev(key9)==9).then(put("key9", "99")) -> err: "failed"`,
		},
		{
			req:            txnRequest([]EtcdCondition{{Key: "key9b", ExpectedRevision: 9}}, []EtcdOperation{{Type: PutOperation, Put: PutOptions{Key: "key9b", Value: ValueOrHash{Value: "991"}}}}, []EtcdOperation{{Type: RangeOperation, Range: RangeOptions{Start: "key9b"}}}),
			resp:           txnResponse([]EtcdOperationResult{{}}, true, 10),
			expectDescribe: `if(mod_rev(key9b)==9).then(put("key9b", "991")).else(get("key9b")) -> success(ok), rev: 10`,
		},
		{
			req:            txnRequest([]EtcdCondition{{Key: "key9c", ExpectedRevision: 9}}, []EtcdOperation{{Type: PutOperation, Put: PutOptions{Key: "key9c", Value: ValueOrHash{Value: "992"}}}}, []EtcdOperation{{Type: RangeOperation, Range: RangeOptions{Start: "key9c"}}}),
			resp:           txnResponse([]EtcdOperationResult{{RangeResponse: RangeResponse{KVs: []KeyValue{{Key: "key9c", ValueRevision: ValueRevision{Value: ValueOrHash{Value: "993"}, ModRevision: 10}}}}}}, false, 10),
			expectDescribe: `if(mod_rev(key9c)==9).then(put("key9c", "992")).else(get("key9c")) -> failure("993"), rev: 10`,
		},
		{
			req:            txnRequest(nil, []EtcdOperation{{Type: RangeOperation, Range: RangeOptions{Start: "10"}}, {Type: PutOperation, Put: PutOptions{Key: "11", Value: ValueOrHash{Value: "111"}}}, {Type: DeleteOperation, Delete: DeleteOptions{Key: "12"}}}, nil),
			resp:           txnResponse([]EtcdOperationResult{{RangeResponse: RangeResponse{KVs: []KeyValue{{ValueRevision: ValueRevision{Value: ValueOrHash{Value: "110"}}}}}}, {}, {Deleted: 1}}, true, 10),
			expectDescribe: `get("10"), put("11", "111"), delete("12") -> "110", ok, deleted: 1, rev: 10`,
		},
		{
			req:            defragmentRequest(),
			resp:           defragmentResponse(10),
			expectDescribe: `defragment() -> ok, rev: 10`,
		},
		{
			req:            listRequest("key11", 0),
			resp:           rangeResponse(nil, 0, 11),
			expectDescribe: `list("key11") -> [], count: 0, rev: 11`,
		},
		{
			req:            listRequest("key12", 0),
			resp:           rangeResponse([]*mvccpb.KeyValue{{Value: []byte("12")}}, 2, 12),
			expectDescribe: `list("key12") -> ["12"], count: 2, rev: 12`,
		},
		{
			req:            listRequest("key13", 0),
			resp:           rangeResponse([]*mvccpb.KeyValue{{Value: []byte("01234567890123456789")}}, 1, 13),
			expectDescribe: `list("key13") -> [hash: 2945867837], count: 1, rev: 13`,
		},
		{
			req:            listRequest("key14", 14),
			resp:           rangeResponse(nil, 0, 14),
			expectDescribe: `list("key14", limit=14) -> [], count: 0, rev: 14`,
		},
		{
			req:            staleListRequest("key15", 0, 15),
			resp:           rangeResponse(nil, 0, 15),
			expectDescribe: `list("key15", rev=15) -> [], count: 0, rev: 15`,
		},
		{
			req:            staleListRequest("key15", 2, 15),
			resp:           rangeResponse(nil, 0, 15),
			expectDescribe: `list("key15", rev=15, limit=2) -> [], count: 0, rev: 15`,
		},
		{
			req:            rangeRequest("key16", "key16b", 0),
			resp:           rangeResponse(nil, 0, 16),
			expectDescribe: `range("key16".."key16b") -> [], count: 0, rev: 16`,
		},
		{
			req:            rangeRequest("key16", "key16b", 2),
			resp:           rangeResponse(nil, 0, 16),
			expectDescribe: `range("key16".."key16b", limit=2) -> [], count: 0, rev: 16`,
		},
	}
	for _, tc := range tcs {
		assert.Equal(t, tc.expectDescribe, NonDeterministicModel.DescribeOperation(tc.req, tc.resp))
	}
}
