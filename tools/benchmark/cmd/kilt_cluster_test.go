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

package cmd

import (
	"slices"
	"testing"

	mp_dp_proto "lwi-channel-common/pkg/mp-dp-proto"

	"google.golang.org/protobuf/proto"
)

func TestKiltEtcdClientEndpoints(t *testing.T) {
	want := []string{
		"10.201.162.151:2379",
		"10.201.47.57:2379",
		"10.201.98.50:2379",
		"10.201.5.177:2379",
		"10.201.117.10:2379",
	}
	got := kiltEtcdClientEndpoints()
	if !slices.Equal(got, want) {
		t.Fatalf("kiltEtcdClientEndpoints() = %v, want %v", got, want)
	}
}

func TestGenerateTwoPcRoundUsesKiltNodes(t *testing.T) {
	payload, key, err := generateTwoPcRound(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != twoPcKeySizeBytes {
		t.Fatalf("key length = %d, want %d", len(key), twoPcKeySizeBytes)
	}

	msg := &mp_dp_proto.TwoPcRound{}
	if err := proto.Unmarshal(payload, msg); err != nil {
		t.Fatalf("failed to unmarshal TwoPcRound: %v", err)
	}

	nodeIPs := make(map[string]struct{}, len(kiltEtcdNodes))
	for _, node := range kiltEtcdNodes {
		nodeIPs[node.ip] = struct{}{}
		if _, ok := msg.GetDpStates()[node.name]; !ok {
			t.Fatalf("dp_states missing node %q: %v", node.name, msg.GetDpStates())
		}
	}

	if _, ok := nodeIPs[msg.GetVip()]; !ok {
		t.Fatalf("vip = %q, want one of %v", msg.GetVip(), nodeIPs)
	}
	for _, backend := range msg.GetBackends() {
		if _, ok := nodeIPs[backend.GetIp()]; !ok {
			t.Fatalf("backend ip = %q, want one of %v", backend.GetIp(), nodeIPs)
		}
	}
}
