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
	"encoding/json"
	"strings"
	"testing"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command"
)

func Test_jsonTxnResponse(t *testing.T) {
	jsonData := `{"header":{"cluster_id":238453183653593855,"member_id":14578408409545168728,"revision":3,"raft_term":2},"succeeded":true,"responses":[{"Response":{"response_range":{"header":{"revision":3},"kvs":[{"key":"a2V5MQ==","create_revision":2,"mod_revision":2,"version":1,"value":"dmFsdWUx"}],"count":1}}},{"Response":{"response_range":{"header":{"revision":3},"kvs":[{"key":"a2V5Mg==","create_revision":3,"mod_revision":3,"version":1,"value":"dmFsdWUy"}],"count":1}}}]}`

	var jsonTxnResponse command.TxnResponseJSON
	decoder := json.NewDecoder(strings.NewReader(jsonData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&jsonTxnResponse); err != nil {
		t.Fatal(err)
	}

	pb := jsonTxnResponse.ToProto()

	roundTrippedJSONTxnResponse := command.TxnResponseJSONFromProto(pb)
	roundTrippedJSONData, err := json.Marshal(roundTrippedJSONTxnResponse)
	if err != nil {
		t.Fatal(err)
	}

	if jsonData != string(roundTrippedJSONData) {
		t.Error("could not get original message after encoding")
	}
}
