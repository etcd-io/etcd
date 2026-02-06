// Copyright 2022 The etcd Authors
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

package e2e

import (
	"bytes"
	"encoding/json"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

func Test_addTxnResponse(t *testing.T) {
	jsonData := `{"header":{"cluster_id":238453183653593855,"member_id":14578408409545168728,"revision":3,"raft_term":2},"succeeded":true,"responses":[{"Response":{"response_range":{"header":{"revision":3},"kvs":[{"key":"a2V5MQ==","create_revision":2,"mod_revision":2,"version":1,"value":"dmFsdWUx"}],"count":1}}},{"Response":{"response_range":{"header":{"revision":3},"kvs":[{"key":"a2V5Mg==","create_revision":3,"mod_revision":3,"version":1,"value":"dmFsdWUy"}],"count":1}}}]}`
	var resp clientv3.TxnResponse
	addTxnResponse(&resp, jsonData)
	t.Logf("%#v\n%#v\n%#v", resp.Responses, resp.Responses[0].Response, resp.Responses[1].Response)
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Errorf("json Unmarshal failed. err: %s", err)
	}
	t.Logf("%#v\n%#v\n%#v", resp.Responses, resp.Responses[0].Response, resp.Responses[1].Response)
	enc, err := json.Marshal(&resp)
	if err != nil {
		t.Errorf("json Marshal failed. err: %s", err)
	}

	encproto, err := protojson.Marshal((*pb.TxnResponse)(&resp))
	if err != nil {
		t.Errorf("json Marshal failed. err: %s", err)
	}

	if string(enc) != jsonData {
		t.Log("original", format([]byte(jsonData)))
		t.Log("output json", format(enc))
		t.Log("output protojson", format(encproto))
		t.Error("could not get original message after encoding")
	}
}

func format(data []byte) string {
	b := &bytes.Buffer{}
	json.Indent(b, []byte(data), "", "  ")
	return b.String()
}
