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

package linearizability

import "sync/atomic"

type idProvider interface {
	ClientId() int
	RequestId() int
}

func newIdProvider() idProvider {
	return &atomicProvider{}
}

type atomicProvider struct {
	clientId  atomic.Int64
	requestId atomic.Int64
}

func (id *atomicProvider) ClientId() int {
	// Substract one as ClientId should start from zero.
	return int(id.clientId.Add(1) - 1)
}

func (id *atomicProvider) RequestId() int {
	return int(id.requestId.Add(1))
}
