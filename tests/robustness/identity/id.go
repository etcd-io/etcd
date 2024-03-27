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
	// NewStreamID returns an integer starting from zero to make it render nicely by porcupine visualization.
	NewStreamID() int
	// NewRequestID returns unique identification used to make write requests unique.
	NewRequestID() int
	// NewClientID returns unique identification for client and their reports.
	NewClientID() int
}

func NewIDProvider() Provider {
	return &atomicProvider{}
}

type atomicProvider struct {
	streamID  atomic.Int64
	requestID atomic.Int64
	clientID  atomic.Int64
}

func (id *atomicProvider) NewStreamID() int {
	return int(id.streamID.Add(1) - 1)
}

func (id *atomicProvider) NewRequestID() int {
	return int(id.requestID.Add(1))
}

func (id *atomicProvider) NewClientID() int {
	return int(id.clientID.Add(1))
}
