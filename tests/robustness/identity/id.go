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

package identity

import "sync/atomic"

type Provider interface {
	// NewStreamId returns an integer starting from zero to make it render nicely by porcupine visualization.
	NewStreamId() int
	// NewRequestId returns unique identification used to make write requests unique.
	NewRequestId() int
	// NewClientId returns unique identification for client and their reports.
	NewClientId() int
}

func NewIdProvider() Provider {
	return &atomicProvider{}
}

type atomicProvider struct {
	streamId  atomic.Int64
	requestId atomic.Int64
	clientId  atomic.Int64
}

func (id *atomicProvider) NewStreamId() int {
	return int(id.streamId.Add(1) - 1)
}

func (id *atomicProvider) NewRequestId() int {
	return int(id.requestId.Add(1))
}

func (id *atomicProvider) NewClientId() int {
	return int(id.clientId.Add(1))
}
