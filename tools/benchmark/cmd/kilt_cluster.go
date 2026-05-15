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

type kiltEtcdNode struct {
	name string
	ip   string
}

// kiltEtcdNodes is the current five-node KILT dev cluster used by the local
// protobuf benchmark binaries. Pass --endpoints to override the client target.
var kiltEtcdNodes = []kiltEtcdNode{
	{name: "kilt-phx-dev-1-etcdn1-1", ip: "10.201.162.151"},
	{name: "kilt-phx-dev-1-etcdn2-1", ip: "10.201.47.57"},
	{name: "kilt-phx-dev-1-etcdn3-1", ip: "10.201.98.50"},
	{name: "kilt-phx-dev-1-etcdn4-1", ip: "10.201.5.177"},
	{name: "kilt-phx-dev-1-etcdn5-1", ip: "10.201.117.10"},
}

func kiltEtcdClientEndpoints() []string {
	endpoints := make([]string, 0, len(kiltEtcdNodes))
	for _, node := range kiltEtcdNodes {
		endpoints = append(endpoints, node.ip+":2379")
	}
	return endpoints
}

func kiltEtcdNodeIP(offset int) string {
	if len(kiltEtcdNodes) == 0 {
		return ""
	}
	if offset < 0 {
		offset = -offset
	}
	return kiltEtcdNodes[offset%len(kiltEtcdNodes)].ip
}
